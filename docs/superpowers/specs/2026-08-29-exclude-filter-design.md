# Exclude `.filter` from stage, pull, and OpenCode snapshots

Date: 2026-08-29

## Problem

OpenCode snapshots `/project` into `~/.local/share/opencode/snapshot`, which is bind-mounted from `/project/.filter/agent-home`. `cell pull` copies `.filter` to the host repo. Snapshots include previous snapshots; guest `project.ext4` fills; host `.filter` balloons.

## Goal

Never copy `.filter` host↔guest via stage/pull. Make OpenCode's git snapshot skip `.filter/`. Existing sessions stay the same size; operator deletes snapshot dirs to reclaim space.

## Non-goals

- Resizing existing `project.ext4`
- Moving agent-home off the project disk
- OpenCode config beyond a `.gitignore` line

## Design

- `StageRepository` always skips a path component named `.filter` (cell-private).
- `cell pull` rsync: `--exclude=.filter` (keep `--exclude=.filter-staged`).
- `guest-entry.sh`: if `/project/.gitignore` has no `.filter` / `.filter/` line, append `.filter/`.

## Success

- Staging a host tree with `.filter/agent-home` does not copy it.
- Pull argv includes `--exclude=.filter`.
- Guest init script ensures `.filter/` gitignore.
- Operator can free the current session with `rm` of snapshot dirs (guest + host) without a new launch.
