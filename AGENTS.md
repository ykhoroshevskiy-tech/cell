# AGENTS.md — workflow rules for this repo

## Rule 1: Spec first (mandatory)

1. **Every feature or behavior change starts with a spec** before any code:
   `docs/superpowers/specs/<YYYY-MM-DD>-<slug>-design.md`
2. The spec MUST contain, in order:
   - **Problem** — what is broken/missing today
   - **Goal** — the end state, one paragraph
   - **Non-Goals** — explicitly out of scope
   - **Design** — files, interfaces, behavior; concrete enough to implement without inventing
   - **Gates** — strict, executable checks (see Rule 2)
3. Code is written **only after the spec exists**. If implementation forces a design
   change, update the spec first, then the code.
4. **One feature = one spec = one commit.** Never bundle unrelated changes.
5. The master spec `docs/superpowers/spec.md` must be reconciled to the code whenever
   behavior lands (spec is as-built truth; stale specs are bugs).

## Rule 2: Gates — a spec is done only when every gate passes

1. Each spec lists **Gates**: numbered, executable checks — build/vet/test commands
   plus behavioral CLI checks with the **exact expected output** (e.g.
   `cell __complete attach --session ''` must print `id\trepo (state)` rows).
2. **Implemented ⇔ all gates pass.** No passing, no completion claims, no "done" status.
3. Run each gate and record the evidence (command + result) in the spec under a
   `## Gate evidence` section before declaring the feature implemented.
4. Gates must be runnable by an agent in this workspace. KVM-only paths are gated by
   `sudo scripts/e2e-network.sh` and may be marked operator-only, not skippable.

## Rule 3: Always build yourself

- Build with `./scripts/build.sh` (embeds the git-derived version
  `0.1.<commit-count>+g<short-sha>`; plain `go build` yields `0.0.0+dev`).
- Rebuild `/project/cell` after every code change; the host installs it via
  `sudo install -m 755 cell /usr/bin/cell`.
- Verification always includes: `go build ./... && go vet ./... && go test ./...`
  plus the spec's behavioral gates.

## Environment facts (do not re-derive)

- Privilege model: mutating commands require root with the explicit error
  `cell: <cmd> requires root — run: sudo cell <cmd>`; read-only commands
  (`ps`, `logs`, `version`, `help`, `completion`, `__complete`) run without root.
- `docs/superpowers/` is tracked: feature specs are committed together with
  their feature (or in a dedicated docs commit); the master `spec.md` is
  reconciled to the code whenever behavior lands.
- Never commit runtime secrets or artifacts: `.filter/`, the `cell` binary,
  guest images and session data are gitignored.
