# Guest OpenCode provider config + superpowers plugin

Date: 2026-08-27

## Problem

Guest `opencode.json` is only `"permission":"allow"`. The agent has no default model, no host proxy, and no superpowers plugin. The working host/project config points at `http://172.16.107.1:8080/v1` (cell0 bridge). Default model is now `kimi_k3`. Superpowers is `plugin: ["superpowers@git+https://github.com/obra/superpowers.git"]` — OpenCode installs it on serve start (guest already has `git` and egress).

## Goal

Every guest boot, agent-home OpenCode config is the cell-managed file: allow-all permissions, opencode-go provider via host `:8080`, default model `opencode-go/kimi_k3`, superpowers plugin spec.

## Non-goals

- Configurable `CELL_*` for baseURL/model (hardcode bridge host).
- Pre-vendoring the plugin into rootfs.
- Writing `/project/opencode.json` (would leak into the user repo on pull).
- Host TUI `--auto`.

## Design

Overwrite `/project/.filter/agent-home/.config/opencode/opencode.json` every boot (same path as today):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "permission": "allow",
  "model": "opencode-go/kimi_k3",
  "plugin": ["superpowers@git+https://github.com/obra/superpowers.git"],
  "provider": {
    "opencode-go": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "OpenCode Go",
      "options": {
        "baseURL": "http://172.16.107.1:8080/v1",
        "apiKey": "dummy"
      },
      "models": {
        "kimi_k3": { "name": "kimi_k3" }
      }
    }
  }
}
```

OpenCode fetches the plugin on first `serve` (needs guest git + NAT, already true). No extra install step in guest-init.

## Files

- `guestinit/guest-entry.sh` — the heredoc in `setup_agent_home`

## Success

- After rebuild-rootfs + start/launch, guest config has `permission`, `kimi_k3`, `172.16.107.1:8080`, and the superpowers plugin spec.
- No `OPENCODE_PERMISSION` env (already gone).
