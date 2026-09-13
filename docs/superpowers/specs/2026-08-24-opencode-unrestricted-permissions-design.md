# OpenCode unrestricted (no permission prompts)

Date: 2026-08-24

## Problem

Guest `opencode serve` still prompts for tool approval. Cell already sets `OPENCODE_PERMISSION={"*":"allow"}` in `/run/opencode.env`. That shape is stale, and persisted `~/.config/opencode/opencode.json` on the bind-mounted agent-home can win over the env. `opencode attach` has no `--auto` (permissions live on the server).

## Goal

In-guest OpenCode runs tools (bash, edit, web, etc.) without asking. Same for `cell launch` and `cell start` after a rootfs rebuild.

## Non-goals

- Changing host `opencode attach` argv (`--auto` is not on `attach`).
- Per-repo or configurable permission policies.
- Rebuilding rootfs in this change (operator runs `sudo cell bootstrap --rebuild-rootfs` so `guest-entry.sh` lands on vda).

## Design

One place: guest OpenCode config on the project disk. Do not also change `OPENCODE_PERMISSION`. Drop that line from `/run/opencode.env` (password env stays).

Every boot, `setup_agent_home` overwrites `/project/.filter/agent-home/.config/opencode/opencode.json`:

```json
{"$schema":"https://opencode.ai/config.json","permission":"allow"}
```

Current all-at-once form. Lives on `project.ext4`. Overwrite so an old `ask` config in agent-home cannot keep prompting.

Do not add host `--auto`. Serve reads this file.

## Files

- `guestinit/guest-entry.sh` — write opencode.json; remove `OPENCODE_PERMISSION` from `/run/opencode.env`

## Success

- After rebuild-rootfs + launch/start, bash/edit do not prompt.
- `guest-entry.sh` writes `opencode.json` with `"permission":"allow"` and no longer sets `OPENCODE_PERMISSION`.
