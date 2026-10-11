# ENG-001: Integration tests for the write path with a fake hledger

- **Type:** engineering
- **Priority:** P2
- **Status:** open
- **Area:** `internal/app`, `internal/hledger`

## Problem

The code with the highest risk, from `finalizeTransaction` through
`candidateAppend`, `validateCandidateContent` and `journal.Replace`, has no
tests. `internal/app` tests cover only journal resolution. The runner loop in
`addflow.Run` is also untested; reducer tests call `Reduce` directly.

## Proposed approach

- Reuse the pattern from `internal/ui/ui_test.go`: write a fake `hledger` shell
  script into `t.TempDir()` and point `PATH` at it. The script records its
  arguments and exits 0 or 1 with chosen stderr.
- Scenarios:
  - successful add appends the exact rendered entry and directives;
  - validation failure leaves the journal byte-identical;
  - `--dry-run` never invokes validation or writes;
  - interruption during validation leaves no candidate file (BUG-004);
  - invocation count per entry (UX-008).
- Drive `addflow.Run` with scripted `Prompts` to test full flows, including
  re-prompting (UX-001).
- Optionally, one test against real hledger, skipped when it is not in `PATH`.

## Related

- BUG-003, BUG-004, UX-001, UX-008; PRODUCT.md next area 4.
