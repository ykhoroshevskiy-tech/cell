#!/bin/sh
set -eu

SESSION="${TMUX_SESSION:-opencode}"
REPO_DIR="${REPO_DIR:-/project}"
AGENT_USER="${AGENT_USER:-agent}"
OPENCODE_CMD="${OPENCODE_CMD:-opencode --auto}"

if ! tmux has-session -t "${SESSION}" 2>/dev/null; then
  cd "${REPO_DIR}" || exit 1
  exec tmux new-session -s "${SESSION}" "${OPENCODE_CMD}; exec zsh -l"
fi

if pgrep -u "${AGENT_USER}" -x opencode >/dev/null 2>&1; then
  exec tmux attach-session -t "${SESSION}"
fi

exec tmux attach-session -t "${SESSION}" \; send-keys -t "${SESSION}:0" "${OPENCODE_CMD}" Enter
