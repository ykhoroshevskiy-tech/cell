# Contract: Bootstrap

**Phase**: 1 | **Normative for**: `internal/bootstrap/`

## Overview

`bootstrap.Ensure(cfg)` downloads kernel, firecracker, and builds rootfs overlay. Called by `cell bootstrap` and implicitly by `cell launch`.

## Artifact paths

Under `{DataDir}/` (defaults: `DataDir=/var/lib/cell`, `ImagesDir={DataDir}/images`):

| Path | Content |
|------|---------|
| `images/bin/firecracker` | Symlink → versioned Firecracker binary (755) |
| `images/bin/firecracker-{version}` | Cached Firecracker binary |
| `images/vmlinux` | Symlink → `vmlinux-{kernel_version}` |
| `images/vmlinux-{kernel_version}` | Cached kernel |
| `images/ubuntu-{squashfs_version}.squashfs` | Cached Ubuntu squashfs |
| `images/rootfs.ext4` | Customized rootfs |
| `images/rootfs.ext4.squashfs-version` | Stamp of squashfs pin/path used to build rootfs |
| `init-scripts/` | Host-level authorized_keys (and keypair if generated) |

## Pinned artifacts (config + env)

Versions are **pinned in `config.Default()`** and overridable via `CELL_*`. Bootstrap hot path MUST NOT list S3 prefixes or follow GitHub `/releases/latest`.

| Config field | Env | Default |
|--------------|-----|---------|
| `ci_prefix` | `CELL_CI_PREFIX` | `firecracker-ci/20260708-f11c230ed107-0/` |
| `kernel_version` | `CELL_KERNEL_VERSION` | `6.1.176` |
| `firecracker_version` | `CELL_FIRECRACKER_VERSION` | `v1.16.1` |
| `squashfs_version` | `CELL_SQUASHFS_VERSION` | `24.04` |

`resolveArtifacts(arch, pins)` builds URLs (no network discovery):

```text
kernel:      {s3}/{ci_prefix}{arch}/vmlinux-{kernel_version}
squashfs:    {s3}/{ci_prefix}{arch}/ubuntu-{squashfs_version}.squashfs
firecracker: https://github.com/firecracker-microvm/firecracker/releases/download/{tag}/firecracker-{tag}-{arch}.tgz
```

where `{s3}=https://s3.amazonaws.com/spec.ccfc.min`, `{arch}=x86_64|aarch64`, `{tag}=firecracker_version`.

Bumping the default stack = edit `config.Default()` (and docs). Operators override via env without rebuilding `cell`.

### Path overrides vs managed defaults

- Managed defaults: `KernelPath={ImagesDir}/vmlinux`, `FirecrackerBin={ImagesDir}/bin/firecracker`, `SquashfsPath={ImagesDir}/ubuntu-{squashfs_version}.squashfs`.
- **Custom path wins** only when `CELL_KERNEL_PATH` / `CELL_FIRECRACKER_BIN` / `CELL_SQUASHFS_PATH` points at a path **different from** the managed default **and** that path is an existing non-empty non-directory file (and `!force`). Then skip download for that artifact.
- Managed paths MUST still download into versioned cache files and refresh symlinks when pins change (existence of the default symlink alone MUST NOT skip).

### Rootfs invalidation

After a successful rootfs build, write stamp `{RootfsPath}.squashfs-version` with either `squashfs_version` or `custom:{cleaned SquashfsPath}`. If stamp missing/mismatches expected stamp (or `--rebuild-rootfs` / `RebuildRootfs`), rebuild rootfs.

## Download

```text
download(dst, art Artifact) error
```

Algorithm:

1. **Cache hit check**: if `dst` exists AND `sha256sum(dst) == art.SHA256` → print `✓ {art.Name} cached ({human_size})`, skip download. Mismatch → delete `dst` (protects against partial/corrupt files).
2. **Retry loop** (3 attempts, exponential backoff: 1s, 2s, 4s):
   - Before the first attempt print `Downloading {art.Name} ({art.Version}) from {art.URL}`. Print `art.Version` as-is (no extra `v` prefix — versions may already be `v1.9.0` or `ubuntu-22.04`).
   - Stream to `dst + ".tmp"` (truncate on first attempt, resume via HTTP `Range` on subsequent attempts if server supports it).
   - **Progress reporting** (docker-pull style): emit a line that updates in place on stderr as bytes stream in. Format:

     ```text
     {art.Name}:  {percent}%  {downloaded}/{total}  {rate}/s  eta {eta}
     ```

     - `{percent}` = `bytes / total * 100` (right-padded to 3 digits). If `Content-Length` unknown, show `?%` and omit `{total}` and `{eta}`.
     - `{downloaded}`, `{total}` = human-readable sizes (`12.3 MiB`, `1.1 GiB`; 1024-based, one decimal).
     - `{rate}` = rolling average since last tick (`4.2 MiB`); tick at most every 200ms to avoid I/O thrash.
     - `{eta}` = `(total - bytes) / rate`, formatted `mm:ss`; show `?` when rate unknown or total unknown.
     - Use `\r` to redraw the same line. Each redraw MUST clear to end-of-line (`\033[K` or equivalent padding) so a shorter final line never leaves garbage from a longer previous line (e.g. no `doneMiB/s` leftovers).
     - Final line on success: `\r{art.Name}:  100%  {total}/{total}  done` + clear-to-EOL + `\n`.
     - Suppress progress when stderr is not a TTY (pipe/file) — fall back to one line per phase: start, done.
   - On HTTP error / network error / timeout → log attempt, sleep backoff, retry.
   - On success → proceed to verify.
3. **Verify**: `sha256sum(dst + ".tmp") == art.SHA256`. Print `verifying sha256…`. Mismatch → delete `.tmp`, print `sha256 mismatch (expected {art.SHA256}, got {actual})`, count as failed attempt, retry.
4. **Atomic commit**: `rename(dst + ".tmp", dst)`. Print `✓ {art.Name} {human_size}`.
5. **Post-processing**:
   - firecracker: extract binary from `.tgz`. The GitHub release archive layout is:

     ```text
     release-{version}-{arch}/
       firecracker-{version}-{arch}          # the binary to install
       firecracker-{version}-{arch}.debug    # MUST NOT be selected
       jailer-{version}-{arch}
       …
     ```

     Algorithm:
     1. Extract tgz into a temp dir under `{images_dir}/bin/` (or extract in place).
     2. Walk the extracted tree; select the first regular file whose basename matches `firecracker-{version}-*` and does **not** end in `.debug`.
     3. `rename`/`copy` that file to `bin/firecracker-{version}` and `chmod 755`.
     4. Symlink `bin/firecracker` → `firecracker-{version}`.
     5. Remove the extracted release directory (keep only the versioned binary + tgz cache).
     6. Print `extracting firecracker…` then `✓ firecracker {human_size}`.

     If no matching binary is found → fail with `firecracker binary not found after extract` and list candidate paths seen.
   - kernel, squashfs: no extraction.

**Failure**: after 3 failed attempts, return error with full context:

```text
failed to download <art.Name> v<art.Version> after 3 attempts:
  url:    <art.URL>
  sha256: <art.SHA256> (expected)
  last error: <last error>
```

No mirror list, no manual escape hatch. Retry + clear error is the whole strategy.

## Rootfs source

rootfs.ext4 is NOT downloaded — it is built locally (see Rootfs overlay build) from the cached ubuntu-squashfs artifact. Skip build only when the rootfs file is ready **and** the squashfs stamp matches (see Rootfs invalidation). If squashfs is missing → download via the pinned-artifact flow above.

## authorized_keys (MUST be non-empty)

Before rootfs build:

1. If `cfg.SSHPublicKey` is set, use it
2. Else generate a host-level ed25519 keypair via `internal/ssh.GenerateKeyPair()` and write public key to `{init-scripts}/authorized_keys`
3. If `authorized_keys` is still empty after step 1–2, bootstrap MUST fail with clear error

Empty `authorized_keys` is a hard error — guest SSH would be unreachable.

## Rootfs build (from ubuntu squashfs)

When `rootfs.ext4` missing or `cfg.RebuildRootfs` / `--rebuild-rootfs`:

Pinned artifact is Firecracker CI **Ubuntu** squashfs (`ubuntu-{squashfs_version}.squashfs`, default `24.04`). It already contains many packages (sshd, tmux, curl, useradd) but **ships without a usable dpkg database** (`/var/lib/dpkg/status` is absent). Therefore:

- **MUST NOT** rely on `apt-get update/install` as the primary install path (it fails with DNS/dpkg errors on this image).
- **MUST NOT** use host `apt-get download` either — the host may be a different Ubuntu release (ABI mismatch with jammy guest). Fetch jammy `.deb`s directly from `http://archive.ubuntu.com/ubuntu/`.
- **MUST** copy host `/etc/resolv.conf` into the tree (kept for any future in-chroot network need; current apt-free path fetches debs on the host).
- Missing packages (**zsh**, **rsync**) MUST be installed by downloading the corresponding Ubuntu **jammy** `.deb` and extracting with `dpkg-deb -x {deb} {root}` (no dpkg database required). Fail if a required binary is still missing after extraction.
- `dpkg-deb -x` **destroys usrmerge** symlinks (`/bin → usr/bin` etc. become real dirs, breaking `/bin/bash`). After **every** extract, **MUST** repair: merge any materialized `/bin,/sbin,/lib,/lib64` dir contents back into `usr/{bin,sbin,lib,lib64}`, then restore the `usr/…` symlink.

### Steps

1. `unsquashfs -d {workDir}/root {squashfsPath}`
2. Copy host `/etc/resolv.conf` → `{root}/etc/resolv.conf` (overwrite).
3. Ensure dirs: `{root}/project`, `{root}/run/sshd`, `{root}/tmp`, `{root}/opt/guest-init`.
4. Bind-mount host `/proc`, `/sys`, `/dev` into `{root}`; mount tmpfs on `{root}/tmp`. Defer unmount (reverse order) on error path.
5. **Ensure packages** (without apt):

   | Binary | Required by | Strategy |
   |--------|-------------|----------|
   | `sshd` | guest SSH | expect present in squashfs; fail if missing |
   | `tmux` | guest tmux | expect present; fail if missing |
   | `curl` | agent tarball fetch (host) | expect present; fail if missing |
   | `useradd` / `passwd` | agent user | expect present; fail if missing |
   | `zsh` | guest-boot | if missing → fetch Ubuntu jammy `.deb` + `dpkg-deb -x` + usrmerge repair |
   | `rsync` | pull | if missing → fetch Ubuntu jammy `.deb` + deps + `dpkg-deb -x` + usrmerge repair |

   Jammy deb resolution: fetch `http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/Packages.gz`, parse `Package:`/`Filename:` pairs to map package name → pool path, then download `http://archive.ubuntu.com/ubuntu/{Filename}`. Required packages and their jammy pool paths (x86_64):
   - `zsh-common` → `pool/main/z/zsh/zsh-common_5.8.1-1_all.deb` (supporting files; no binary check)
   - `zsh` → `pool/main/z/zsh/zsh_5.8.1-1_amd64.deb` (installs to `/bin/zsh`, **not** `/usr/bin/zsh`)
   - `libpopt0` → `pool/main/p/popt/libpopt0_1.18-3build1_amd64.deb` (provides `libpopt.so.0`; required by rsync — MUST install before `rsync` or guest `rsync` fails with `error while loading shared libraries: libpopt.so.0`)
   - `rsync` → `pool/main/r/rsync/rsync_3.2.3-8ubuntu3_amd64.deb` (installs to `/usr/bin/rsync`)

   Extract order: `zsh-common` → `zsh` → `libpopt0` → `rsync`. After each extract, run usrmerge repair (see above).

   Post-verify for `rsync`: not only `test -x /usr/bin/rsync`, but also that dynamic loader can resolve deps (`ldd /usr/bin/rsync` MUST NOT report `not found` for any library). Fail bootstrap if `libpopt.so.0` (or any other NEEDED lib) is missing.
6. **Chroot customization** (errors surfaced):

```text
id agent >/dev/null 2>&1 || useradd -m -s /bin/zsh agent
mkdir -p /run/sshd
```

Write `/etc/ssh/sshd_config`:

```text
Port 22
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes
AuthorizedKeysFile /project/.filter/authorized_keys
UsePAM yes
PidFile /run/sshd/sshd.pid
Subsystem sftp /usr/lib/openssh/sftp-server
```

7. Copy embedded guest-init scripts → `/opt/guest-init/{guest-entry.sh,tmux-attach.sh}` (mode 755).
8. Install in-guest **agent** (optional, vendor-neutral). If `CELL_AGENT_URL` / `agent_url` is empty → skip. Otherwise download on the **host** (not in chroot). URL may contain `{target}` → `linux-x64-baseline` / `linux-arm64-baseline`. Cache: `{ImagesDir}/{agent_bin}-{target}.tar.gz`. Extract tarball top-level `{agent_bin}`, install to `{root}/opt/agent/bin/{agent_bin}`, symlink `{root}/usr/local/bin/{agent_bin}`. On failure: stub binary + clear error. Defaults target OpenCode (`agent_bin=opencode`, OpenCode release URL).
9. Verify inside chroot: `which tmux`, `which rsync`, `which sshd`, `which zsh`, `test -x /opt/guest-init/guest-entry.sh`. Fail bootstrap if any missing.
10. **MUST unmount** chroot bind mounts (`/tmp` tmpfs, `/dev`, `/sys`, `/proc` under `{workDir}/root`) **before** `mkfs.ext4 -d`. If any pseudo-fs is still mounted, populate walks `/proc/*` etc. and fills the image (ENOSPC / hangs). Defer MUST still unmount on error paths.
11. Build the ext4 image with `mkfs.ext4 -d` (matches Python launcher; proven fast). Filter the harmless `__populate_fs: symlink increased in size` stderr lines unless `--verbose`. With `--verbose`, stream raw mkfs stdout/stderr.

```text
rootfs_size_mb = max(4096, ProjectDiskSizeMB * 4)   # default 4096 when project_disk_size_mb=1024
truncate -s {rootfs_size_mb}M {rootfsPath}
mkfs.ext4 -F -d {workDir}/root {rootfsPath}   # filter "__populate_fs: symlink increased in size" unless verbose
```

12. Print `✓ rootfs {human_size}`.

`authorized_keys` for the **session** lives on the project disk (see `project-disk.md`); rootfs only configures sshd to read `AuthorizedKeysFile /project/.filter/authorized_keys`. Host-level key at `{init-scripts}/authorized_keys` MUST still be non-empty (bootstrap sanity).

### Unmount / cleanup

Defer MUST unmount `/tmp` (tmpfs), `/dev`, `/sys`, `/proc` binds and `RemoveAll(workDir)` on the error path — no leaked mounts.

## Init scripts extraction

Write to `{init-scripts}/`:

| File | Purpose |
|------|---------|
| `authorized_keys` | Session/host public key (non-empty) |
| `00-ssh.sh` | Install SSH key at boot |
| `10-network.sh` | Configure eth0 |
| `20-sshd.sh` | Start sshd |
| `90-runtime.sh` | Create tmux session |
| `99-done.sh` | Completion marker |

Scripts MUST be executable (755) when embedded into rootfs at `/opt/guest-init/`.

## Force / rebuild flags

| Flag | Effect |
|------|--------|
| `--force` | Remove versioned kernel/fc/squashfs caches for current pins; re-download; rebuild rootfs |
| `--rebuild-rootfs` | Rebuild rootfs only; keep cached kernel + firecracker + squashfs |

Download failures MUST wrap the error with the pin values used (`ci_prefix`, `kernel`, `firecracker`, `squashfs`).

**Note on sha256:** `Artifact.SHA256` is verified when non-empty. Current pin resolve leaves SHA256 empty (size/cache presence gates reuse). Re-introducing pinned checksums is allowed as a hardening follow-up without changing the pin/env surface.
