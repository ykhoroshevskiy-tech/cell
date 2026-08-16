#!/bin/sh
set -eu
SESSION="${TMUX_SESSION:-agent}"
REPO_DIR="${REPO_DIR:-/project}"
if tmux has-session -t "${SESSION}" 2>/dev/null; then
  exec tmux attach-session -t "${SESSION}"
fi
cd "${REPO_DIR}" || exit 1
exec tmux new-session -s "${SESSION}" -c "${REPO_DIR}"
