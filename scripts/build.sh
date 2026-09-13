#!/usr/bin/env bash
# Build the cell binary with a version derived from git history:
# 0.<minor>.<commit-count>+g<short-sha>[-dirty] — bumps on every commit.
set -euo pipefail

cd "$(dirname "$0")/.."

commit_count="$(git rev-list --count HEAD 2>/dev/null || echo 0)"
short_sha="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
if git diff --quiet 2>/dev/null && git diff --cached --quiet 2>/dev/null; then
	dirty=""
else
	dirty="-dirty"
fi

version="0.1.${commit_count}+g${short_sha}${dirty}"
out="${1:-cell}"

go build -ldflags "-X github.com/ykhoroshevskiy-tech/cell/internal/version.Version=${version}" -o "$out" ./cmd/cell
echo "built $out (cell ${version})"
