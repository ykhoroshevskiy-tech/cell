# Design: oh-my-zsh plugin — aliases with baked flags and live session completion

## Problem

`cell` requires typing long commands with flags (`sudo cell attach --session
<id>`). Shell completion alone did not cover the sudo-prefixed muscle memory,
and `cstop` was inconsistent with `ca`/`cs`/`crm` — it did not bake `--session`,
so `cstop <id>` did not work while `ca <id>` and `cs <id>` did.

## Goal

Short, consistent aliases where every session-taking alias bakes its flag:
`ca <id>` = `sudo cell attach --session <id>`, `cs <id>` = `sudo cell start
--session <id>`, `cstop <id>` = `sudo cell stop --session <id>`, `cl <repo>` =
`sudo cell launch --repo <repo>` — each with live session/directory completion.

## Non-Goals

- Completing `cell` itself (cobra's `cell completion zsh` covers that).
- Alias completion for `--all` on `cstop` (baking `--session` there would break
  `--all`; the dedicated `cstopall` alias covers it).
- Bash/fish support.

## Design

- `plugins/cell/cell.plugin.zsh` (single file, omz convention):
  - aliases with baked flags — `ca` `sudo cell attach --session`, `cl`
    `sudo cell launch --repo`, `cs` `sudo cell start --session`, `cps`
    `cell ps`, `cstop` `sudo cell stop --session`, `crm` `sudo cell rm
    --session`, `cssh`, `cstat`, `cverify`, `clogs`, `cpull`, `crescue`
    (`--session` baked), `cboot` `sudo cell bootstrap`, `cnet`
    `sudo cell net-setup`, `cver` `cell version`, `cstopall`
    `sudo cell stop --all`;
  - `compdef _cell_aliases_args` registered for every session-taking alias,
    guarded by `(( $+functions[compdef] ))`;
  - `_cell_aliases_sessionIds` shells out to `cell __complete attach --session
    ''` (same data source as `ps`: sudo-free, no probes), strips cobra's
    `:<directive>` line, feeds `_describe`;
  - `cl` completes directories for `--repo` (`_files -/`), third word offers
    `--config`/`--no-attach`; `crescue` third word completes the dest dir;
  - binary resolution falls back to `/usr/bin/cell`, `/usr/local/bin/cell`,
    `$HOME/.local/bin/cell` when `command -v cell` fails.

## Gates

1. Sourcing the plugin in `zsh -f` defines all aliases; `compdef` registration
   is guarded (no error in non-completion shells).
2. With a seeded `CELL_SESSION_DATA_DIR` and a stubbed `_describe`,
   `ca <TAB>` and `cstop <TAB>` yield the same session candidates
   (`id\trepo (state)`); `--all` is NOT offered on `cstop` anymore.
3. Alias table matches this spec (every session-taking alias bakes its flag).

## Gate evidence

```
$ PATH=/project:$PATH zsh -f (stub harness) words=(ca '') CURRENT=2
CAND: aaa111	/home/u/my-repo (running)
CAND: bbb222	/home/u/other (stopped)
$ words=(cstop '') CURRENT=2
CAND: aaa111	/home/u/my-repo (running)
CAND: bbb222	/home/u/other (stopped)      # no --all candidate
$ alias | grep -c 'sudo cell.*--session'       # 9 session aliases bake the flag
9
$ alias cstop
cstop='sudo cell stop --session'
```
