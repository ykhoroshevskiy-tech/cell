#!/usr/bin/env bash
# Build the cell binary with a tag-driven semver version:
#   exact tag vX.Y.Z      -> X.Y.Z[-dirty]
#   commits after a tag   -> X.Y.Z-<n>+g<sha>[-dirty]
#   no tags               -> 0.0.0-dev+g<sha>[-dirty]
set -euo pipefail

cd "$(dirname "$0")/.."

short_sha="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
if git diff --quiet 2>/dev/null && git diff --cached --quiet 2>/dev/null; then
	dirty=""
else
	dirty="-dirty"
fi

if desc="$(git describe --tags --first-parent 2>/dev/null)"; then
	version="$(echo "${desc}" | sed -E 's/^v//; s/(.*)-g([0-9a-f]+)$/\1+g\2/')"
else
	version="0.0.0-dev+g${short_sha}"
fi
version="${version}${dirty}"
out="${1:-cell}"

go build -ldflags "-X github.com/ykhoroshevskiy-tech/cell/internal/version.Version=${version}" -o "$out" ./cmd/cell
echo "built $out (cell ${version})"
