# Configuration

All settings are overridden with `CELL_*` environment variables. Defaults are
fixed for reproducible builds.

## Artifact pins

| Setting | Default |
|---------|---------|
| `CELL_CI_PREFIX` | `firecracker-ci/20260708-f11c230ed107-0/` |
| `CELL_KERNEL_VERSION` | `6.1.176` |
| `CELL_FIRECRACKER_VERSION` | `v1.16.1` |

Path overrides (`CELL_KERNEL_PATH`, `CELL_FIRECRACKER_BIN`) still win when the
file already exists. Managed defaults refresh from pins (versioned cache +
symlink). A rootfs rebuild is triggered when any pin changes (rootfs stamp).

## In-guest agent (vendor-neutral; defaults install OpenCode)

| Setting | Default |
|---------|---------|
| `CELL_AGENT_URL` | OpenCode release tarball (`…/opencode-{target}.tar.gz`); empty skips install |
| `CELL_AGENT_BIN` | `opencode` |
| `CELL_AGENT_CMD` | `opencode serve --hostname 127.0.0.1 --port 4096` |
| `CELL_AGENT_SERVE_PORT` | `4096` |
| `CELL_HOST_AGENT_BIN` | `opencode` |
| `CELL_TMUX_SESSION_NAME` | `agent` |

`{target}` in the URL is replaced with `linux-x64-baseline` /
`linux-arm64-baseline`.

The host must have OpenCode installed (`CELL_HOST_AGENT_BIN` or `opencode` on
PATH). `sudo cell launch` / `sudo cell attach` fail fast if it is missing; the
VM keeps running.

## Superpowers (opt-in agent tooling)

| Setting | Default |
|---------|---------|
| `CELL_INSTALL_SUPERPOWERS` | `false` |

When `true`, the rootfs build installs
[Superpowers](https://github.com/obra/superpowers) (skill tooling for AI coding
agents) into the guest at `/opt/opencode-plugins` and the guest's default agent
config loads it as a plugin. Default guests ship without it. Changing the flag
forces a one-time rootfs rebuild (the build stamp includes it).

## Runtime

| Setting | Default |
|---------|---------|
| `CELL_DATA_DIR` | `/var/lib/cell` |
| `CELL_PROJECT_DISK_SIZE_MB` | `3072` |
| `CELL_ROOTFS_SIZE_MB` | `4096` |
| `CELL_NODE_VERSION` | `v24.20.0` |
| `CELL_UV_VERSION` | `0.12.7` |
| `CELL_PYTHON_VERSION` | `3.13` |
| `CELL_VCPU_COUNT` | `4` |
| `CELL_MEM_SIZE_MIB` | `8192` |
| `CELL_AUTO_PULL` | `true` |
| `CELL_AUTO_PULL_INTERVAL_SEC` | `30` |

## Guest rootfs

Ubuntu 24.04 via `debootstrap` (apt works), mounted read-only. `/usr/local` is
bind-mounted from the project disk so the agent can install to
`/usr/local/bin`. Node LTS, uv, and CPython (see runtime versions above) are
installed to `/usr/local`. The guest has `git` and passwordless `sudo` for
user `agent`. Rebuild the rootfs after changing pins:
`sudo cell bootstrap --rebuild-rootfs` (the host needs the `debootstrap`
package).

New sessions get a 3072 MiB project disk; the rootfs size (4096 MiB) does not
scale with the project disk, and existing session disks are not resized.

## `--config` (guest agent config)

`sudo cell launch --config /path/to/opencode.json` copies the file to
`.filter/opencode.json`; the guest loads it as
`~/.config/opencode/opencode.json`. It is not stored in `session.json` and is
skipped by pull.
