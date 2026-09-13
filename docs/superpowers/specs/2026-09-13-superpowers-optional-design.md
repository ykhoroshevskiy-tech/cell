# Design: optional Superpowers install in the guest rootfs

## Problem

Superpowers (agent-development tooling: brainstorming/spec skills for AI coding
agents) is hardcoded into every guest rootfs: a chroot `npm install` from GitHub
at rootfs build time plus a hardcoded `plugin` entry in the guest's default
opencode.json. Users who only want the coding agent pay extra image build time
and size for tooling they never use.

## Goal

`CELL_INSTALL_SUPERPOWERS` (config `install_superpowers`, default **false**)
makes the Superpowers install opt-in. When true, the rootfs build runs the
existing chroot npm install; when false, the guest ships without it and the
default agent config contains no plugin entry.

## Non-Goals

- Changing the superpowers package pin (`superpowers@git+…`).
- Installing other plugins or a plugin manager.
- Making the plugin set configurable per session.

## Design

- `internal/config`: `InstallSuperpowers bool` (viper key `install_superpowers`,
  default false, env `CELL_INSTALL_SUPERPOWERS`).
- `internal/bootstrap`: `buildRootfs` runs the chroot npm install only when
  `cfg.InstallSuperpowers` is true; `guestCustomizeScript` takes the config and
  includes the plugin-directory verify line only when true.
- Rootfs stamp gains `+sp:on|off` so toggling the flag invalidates the cached
  rootfs. Because the default flips from always-on to off, one cached-rootfs
  rebuild is expected on first `sudo cell bootstrap` after this change.
- `guestinit/guest-entry.sh`: the default `opencode.json` written on first boot
  includes the `plugin` key only when
  `/opt/opencode-plugins/node_modules/superpowers` exists (conditional inside
  the script — no bootstrap state needs to reach the guest).

## Gates

1. `config.Load()` with no env yields `InstallSuperpowers == false`; with
   `CELL_INSTALL_SUPERPOWERS=true` yields true.
2. `guestCustomizeScript` output contains the superpowers verify only when the
   flag is true.
3. `squashfsBuildStamp` differs between `InstallSuperpowers` true/false.
4. `guestinit/guest-entry.sh` contains the conditional plugin marker.
5. `go build ./... && go vet ./... && go test ./...` → green.

Operator-only: `sudo cell bootstrap` rebuilds the rootfs (one-time, expected —
the stamp default flips), and guests boot with/without the plugin per flag.

## Gate evidence

```
$ go test ./internal/config/ -run InstallSuperpowers -v
--- PASS: TestInstallSuperpowersDefaultFalseAndEnvOverride
$ go test ./internal/bootstrap/ -run 'Customize|Stamp' -v
--- PASS: TestGuestCustomizeScriptOmitsSuperpowersWhenDisabled
--- PASS: TestGuestCustomizeScriptChecksGitSudoNode        (enabled case)
--- PASS: TestSquashfsBuildStampSuperpowersToggle          (+sp:off vs +sp:on)
--- PASS: TestSquashfsBuildStampIncludesNodeVersion        (+sp:off)
--- PASS: TestSquashfsBuildStampUsesNodePin                (+sp:off)
$ go test ./guestinit/ -run Plugin -v
--- PASS: TestGuestEntryPluginEntryConditionalOnSuperpowers
$ go build ./... && go vet ./... && go test ./...   # green
$ bash -n guestinit/guest-entry.sh                  # syntax clean
```
Operator-only (pending): `sudo cell bootstrap` — one-time rootfs rebuild
(stamp default flips to `+sp:off`); with `CELL_INSTALL_SUPERPOWERS=true` the
old behavior returns.
