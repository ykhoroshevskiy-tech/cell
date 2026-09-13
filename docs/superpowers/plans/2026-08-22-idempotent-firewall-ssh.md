# Idempotent firewall + BatchMode SSH Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Second `cell launch` reuses existing `CELL_*` iptables chains; host net errors include command output; cell SSH never prompts for a password.

**Architecture:** `ensureChain` checks `iptables -nL` before `-N`, and still treats `already exists` as success. `run()` wraps exec errors with stdout/stderr. `sshBaseArgs` adds `BatchMode=yes`.

**Tech Stack:** Go 1.26, iptables, OpenSSH.

**Spec:** `docs/superpowers/specs/2026-08-22-idempotent-firewall-ssh-design.md`

## Global Constraints

- Do not change firewall policy (ports, MASQUERADE, isolation rules).
- Do not migrate to nftables-native syntax.
- Do not rewrite the e2e script except as needed to re-run smoke.
- Guests are key-only; password login is not a recovery path.
- Do not git commit unless the user explicitly asks.
- A draft of this work may already be in the tree; finish it against the spec, do not rewrite from scratch.

---

### Task 1: Idempotent ensureChain and command errors

**Files:**
- Modify: `internal/network/firewall.go`
- Modify: `internal/network/runner.go`
- Modify: `internal/network/reconcile_test.go`
- Modify: `internal/network/runner.go` fake runner only if the test needs `-nL` / `-N` chain state

**Interfaces:**
- Consumes: existing `EnsureFirewall`, `run`, `output`, fake `Runner`
- Produces: `chainExists(table, chain string) bool`; `ensureChain` skips `-N` when the chain exists; `run()` error format `"<cmd> <args>: %w\n<output>"`

- [ ] **Step 1: Strengthen the unit test**

`TestEnsureFirewallIdempotent` must fail on the old `ensureChain` (always `-N`, error is bare `exit status 1` with no `already exists`).

Make the fake runner track filter/nat chains:
- `iptables -nL CHAIN` / `iptables -t nat -nL CHAIN` succeeds only if the chain exists
- `iptables -N CHAIN` / `iptables -t nat -N CHAIN` creates the chain, or returns `fmt.Errorf("exit status 1")` with no `already exists` text if it already exists
- other iptables commands keep succeeding

If `TestEnsureFirewallIdempotent` is missing, add it: call `EnsureFirewall()` twice.

- [ ] **Step 2: Run the test (expect fail if implementation incomplete)**

Run: `go test ./internal/network -run TestEnsureFirewallIdempotent -count=1`

- [ ] **Step 3: Implement**

`chainExists`: `iptables -nL <chain>` for filter; `iptables -t <table> -nL <chain>` otherwise.

`ensureChain`: if `chainExists`, return nil; else `-N`; if `-N` errors and `err.Error()` contains `already exists`, return nil.

`run()`: on error, include combined output:

```go
return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
```

(omit the output line when `out` is empty).

- [ ] **Step 4: Re-run**

Run: `go test ./internal/network -count=1`

Expected: PASS

- [ ] **Step 5: Do not commit**

---

### Task 2: SSH BatchMode and full test suite

**Files:**
- Modify: `internal/ssh/attach.go` (`sshBaseArgs`)
- Modify: `internal/ssh/attach_test.go` (`TestTunnelSSHArgs`)

**Interfaces:**
- Consumes: `sshBaseArgs` used by `ServerReady`, `TunnelSSHArgs`, `ShellAttachArgs`
- Produces: `-o BatchMode=yes` in `sshBaseArgs`

- [ ] **Step 1: Update `TestTunnelSSHArgs` to require `BatchMode=yes` after `IdentitiesOnly=yes`**

- [ ] **Step 2: Add `-o BatchMode=yes` to `sshBaseArgs`**

- [ ] **Step 3: Run tests**

Run: `go test ./internal/ssh ./internal/network ./...`

Expected: PASS

- [ ] **Step 4: Do not commit**

Manual (human): `go build -o cell ./cmd/cell && sudo scripts/e2e-network.sh ./cell` — must get past VM B without a password prompt.
