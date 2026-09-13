# cell oh-my-zsh plugin — aliases for the cell microVM runtime.
#
# Mutating aliases run under sudo (cell requires root for them);
# read-only ones (cps, clogs, cver) stay sudo-free. Flags are baked in:
# `ca <id>` runs `sudo cell attach --session <id>` directly.
#
# Tab completion: every session-taking alias lists live sessions as
# "id — /repo/path (running|stopped)" via `cell __complete`; `cl` completes
# directories for --repo.

# Resolve the cell binary once (completion must work regardless of PATH quirks).
typeset -g _cell_bin
if [[ -z "$_cell_bin" ]]; then
  _cell_bin="$(command -v cell 2>/dev/null || true)"
  if [[ -z "$_cell_bin" ]]; then
    typeset _p
    for _p in /usr/bin/cell /usr/local/bin/cell "$HOME/.local/bin/cell"; do
      if [[ -x "$_p" ]]; then _cell_bin="$_p"; break; fi
    done
  fi
fi

# Live sessions as completion candidates: "id — /repo/path (running|stopped)".
_cell_aliases_sessionIds() {
  local -a sessions
  local comp_status line
  [[ -n "$_cell_bin" ]] || return
  comp_status=$($_cell_bin __complete attach --session "" 2>/dev/null)
  if [[ -n "$comp_status" ]]; then
    while IFS= read -r line; do
      [[ "$line" == :* ]] && continue
      sessions+=("$line")
    done <<< "$comp_status"
  fi
  _describe -t sessions 'sessions (id — repo (state))' sessions
}

_cell_aliases_repos() {
  _files -/
}

# Per-alias argument completion.
_cell_aliases_args() {
  local alias_name="${words[1]}"
  case "$alias_name" in
    ca|cs|crm|cssh|cstat|cverify|clogs|cpull|crescue)
      if (( CURRENT == 2 )); then
        _cell_aliases_sessionIds
      elif [[ "$alias_name" == "crescue" && "$CURRENT" -ge 3 ]]; then
        _files -/
      fi
      ;;
    cstop)
      if (( CURRENT == 2 )); then
        _alternative \
          'sessions:session ids:_cell_aliases_sessionIds' \
          'flags:flag:(--all)'
      fi
      ;;
    cl)
      if (( CURRENT == 2 )); then
        _cell_aliases_repos
      elif (( CURRENT == 3 )); then
        _alternative \
          'flags:flag:(--config --no-attach)' \
          'files:opencode config:_files'
      fi
      ;;
  esac
}

# attach to a running session TUI:  ca <TAB> <id>
alias ca='sudo cell attach --session'
# launch a VM for a repo:            cl <TAB> /path/to/repo
alias cl='sudo cell launch --repo'
# boot an existing session disk:     cs <TAB>
alias cs='sudo cell start --session'
# list sessions (no sudo needed)
alias cps='cell ps'
# stop one VM: cstop <TAB>, or all:  cstop --all
alias cstop='sudo cell stop'
# remove a stopped session
alias crm='sudo cell rm --session'
# debug SSH + tmux in the guest
alias cssh='sudo cell ssh --session'
# probe VM/SSH/server for one session
alias cstat='sudo cell status --session'
# readiness check with serial tail
alias cverify='sudo cell verify --session'
# print serial.log (no sudo needed)
alias clogs='cell logs --session'
# rsync guest workspace back to the host repo
alias cpull='sudo cell pull --session'
# extract workspace from a project disk: crescue <id> <dest>
alias crescue='sudo cell rescue --session'
# download/build kernel, rootfs, firecracker
alias cboot='sudo cell bootstrap'
# reconcile bridge/TAP/firewall state
alias cnet='sudo cell net-setup'
alias cver='cell version'
alias cstopall='sudo cell stop --all'

if (( $+functions[compdef] )); then
  compdef _cell_aliases_args ca cl cs cstop crm cssh cstat cverify clogs cpull crescue
fi
