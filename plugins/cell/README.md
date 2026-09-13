# cell oh-my-zsh plugin

Zsh aliases for the [cell](../../README.md) microVM runtime, with live
`--session` completion (session ID + repo path + running/stopped state).
Flags are baked in: `ca <id>` runs `sudo cell attach --session <id>` directly.

## Install

```sh
cp -r plugins/cell ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/cell
# in ~/.zshrc: plugins=(... cell)
exec zsh
```

## Aliases

| Alias | Expands to |
|-------|------------|
| `ca` | `sudo cell attach --session` |
| `cl` | `sudo cell launch --repo` |
| `cs` | `sudo cell start --session` |
| `cps` | `cell ps` |
| `cstop` | `sudo cell stop` |
| `crm` | `sudo cell rm --session` |
| `cssh` | `sudo cell ssh --session` |
| `cstat` | `sudo cell status --session` |
| `cverify` | `sudo cell verify --session` |
| `clogs` | `cell logs --session` |
| `cpull` | `sudo cell pull --session` |
| `crescue` | `sudo cell rescue --session` |
| `cboot` | `sudo cell bootstrap` |
| `cnet` | `sudo cell net-setup` |
| `cver` | `cell version` |
| `cstopall` | `sudo cell stop --all` |

## Completion

Tab after any session-taking alias lists live sessions as
`id — /repo/path (running|stopped)`; `cl <TAB>` completes directories for
`--repo`; `cstop <TAB>` offers sessions and `--all`; `cl /repo <TAB>` offers
`--config`/`--no-attach`.
