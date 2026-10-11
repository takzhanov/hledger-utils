# BUG-004: Pre-write validation differs from `fin check`; interrupted validation leaves a temp file

- **Type:** bug
- **Priority:** P3 — inconsistency and leftover files; no wrong write known yet
- **Status:** open
- **Area:** `internal/hledger/hledger.go` (`CheckStrict`, `StrictCheck`),
  `internal/app/app.go` (`validateCandidateContent`)

## Symptom

There are two validation methods with near-identical names:

| Method        | Command                     | Used by               |
|---------------|-----------------------------|-----------------------|
| `CheckStrict` | `hledger check --strict`    | `fin check`           |
| `StrictCheck` | `hledger bs -tSV --strict`  | candidate before write|

The promise "whatever `fin add` writes passes `fin check`" therefore holds only
because both commands happen to run hledger's strict parse. The `bs` variant
also builds a balance-sheet report with tree, sort and valuation flags that are
irrelevant to validation. Validation thus depends on report behaviour and does
extra work. No concrete diverging journal has been found, hence P3.

Measured on a synthetic 20,000-transaction journal (hledger 1.51.2): `bs -tSV`
0.56 s vs `check` 0.53 s, so there is no performance reason to keep `bs`.

## Second problem: leftover candidate file

`validateCandidateContent` creates `.fin-hledger-candidate-XXXX` next to the
journal and removes it with `defer`. On Ctrl-C during validation Go's default
SIGINT handling exits without running deferred calls, so the file stays in the
journal directory (by code reading). An `include *` glob in the user's journal
would then pick it up.

## Proposed fix

- Keep one method, `CheckStrict` (`hledger check --strict`), and use it in both
  places. Delete `StrictCheck`.
- Use `signal.NotifyContext` + `exec.CommandContext` in the add flow, so an
  interrupt cancels hledger and cleanup still runs.
- After BUG-003 the candidate is either renamed into place or removed. There is
  no third outcome.

## Acceptance criteria

- One validation command exists and is used by both `fin check` and `fin add`.
- An interrupt during validation leaves no candidate file (test with a fake
  `hledger` that sleeps, plus a sent signal).

## Related

- BUG-003, ENG-001.
