# Design: git-derived version, bumped on every commit

## Problem

`cell version` printed a hardcoded `0.1.0` forever; builds were
indistinguishable and there was no way to map a binary back to a commit.

## Goal

The version is derived from git history at build time and changes on every
commit, so any binary is traceable to an exact commit and dirty state.

## Non-Goals

- SemVer release tagging or GitHub releases.
- Version bump files edited by hooks (hooks do not travel with clones).

## Design

- `internal/version/version.go`: `var Version = "0.0.0+dev"` (ldflags target).
- `scripts/build.sh` computes
  `0.1.<git rev-list --count HEAD>+g<short sha>[-dirty]` (dirty when the tree
  is not clean) and builds with
  `-ldflags "-X .../internal/version.Version=${version}"`. Plain `go build`
  without the script yields the dev fallback.
- `cell version` prints `cell <version>`.

## Gates

1. `./scripts/build.sh && ./cell version` output matches
   `^cell 0\.[0-9]+\.[0-9]+\+g[0-9a-f]+(-dirty)?$`.
2. Committing changes the version: build → commit → build increments the
   commit-count component.
3. `go build ./... && go vet ./... && go test ./...` → green.

## Gate evidence

```
$ ./scripts/build.sh && ./cell version
built cell (cell 0.1.49+g793b3c1-dirty)
cell 0.1.49+g793b3c1-dirty          # dirty tree before the version commit
$ git commit … && ./scripts/build.sh && ./cell version
cell 0.1.50+gcaeeb17                # count incremented, dirty suffix gone
$ go build ./... && go vet ./... && go test ./...   # green
```
