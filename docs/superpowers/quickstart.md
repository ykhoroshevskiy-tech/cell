# Quickstart: cell

**Requirements:** Linux + KVM, root for runtime, Go 1.26+ to build.

```sh
go build -o cell ./cmd/cell
sudo install -m 755 cell /usr/bin/cell

sudo cell bootstrap
sudo cell launch --repo /path/to/your/repo
```

**Expected:** SSH into tmux session `agent` in `/project` (default agent command is OpenCode). Detach with `Ctrl-b d`.

```sh
sudo cell ps
sudo cell status --session <id>
sudo cell ssh --session <id>
sudo cell pull --session <id>
sudo cell stop --session <id>
```

Optional pin override (reproducible stack; defaults already pinned):

```sh
export CELL_CI_PREFIX=firecracker-ci/20260708-f11c230ed107-0/
export CELL_KERNEL_VERSION=6.1.176
export CELL_FIRECRACKER_VERSION=v1.16.1
export CELL_SQUASHFS_VERSION=24.04
sudo -E cell bootstrap --force
```

See `contracts/cli.md` and `contracts/bootstrap.md` for flags and artifact rules.
