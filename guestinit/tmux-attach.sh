#!/bin/sh
set -eu

SESSION="${TMUX_SESSION:-agent}"
REPO_DIR="${REPO_DIR:-/project}"
AGENT_USER="${AGENT_USER:-agent}"
AGENT_CMD="${AGENT_CMD:-}"
AGENT_BIN="${AGENT_BIN:-}"

if ! tmux has-session -t "${SESSION}" 2>/dev/null; then
  cd "${REPO_DIR}" || exit 1
  if [ -n "${AGENT_CMD}" ]; then
    exec tmux new-session -s "${SESSION}" "${AGENT_CMD}; exec zsh -l"
  fi
  exec tmux new-session -s "${SESSION}" -c "${REPO_DIR}"
fi

if [ -n "${AGENT_BIN}" ] && pgrep -u "${AGENT_USER}" -x "${AGENT_BIN}" >/dev/null 2>&1; then
  exec tmux attach-session -t "${SESSION}"
fi

if [ -n "${AGENT_CMD}" ]; then
  exec tmux attach-session -t "${SESSION}" \; send-keys -t "${SESSION}:0" "${AGENT_CMD}" Enter
fi
exec tmux attach-session -t "${SESSION}"
