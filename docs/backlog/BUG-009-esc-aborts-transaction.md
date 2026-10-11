# BUG-009: Esc in the account selector aborts the whole transaction

- **Type:** bug (UX)
- **Priority:** P2 — one keypress discards everything entered
- **Status:** open
- **Area:** `internal/ui/ui.go` (`AskAccount`, `askAccountWithFZF`)

## Symptom

The fzf header says `Esc cancels`, which reads as "cancel this selection".
Pressing Esc actually ends `fin add` with `error: interrupted`. The date,
description and all postings entered so far are lost.

## Root cause

fzf 0.65.1 (`man fzf`, EXIT STATUS) exits with:

- `0` normal exit (including keys listed in `--expect`)
- `1` no match
- `130` interrupted with CTRL-C **or ESC**

fin passes `--expect=ctrl-d,ctrl-c`, so Ctrl-C and Ctrl-D exit with `0` plus a
key line. Esc is not in `--expect` and exits with `130`, which fin maps to
`ErrInterrupted`. That error propagates out of `addflow.Run` and aborts the
command. Separately, exit `1` (Enter with a non-matching query) maps to
`ErrSelectionCancelled`, and `AskAccount` silently reopens fzf in a loop.
Ctrl-D maps to `io.EOF` and prints `error: EOF`.

(Based on the fzf man page and code reading. The interactive keypress itself
was not automated.)

## Proposed fix

Define key semantics explicitly and put them in the header:

- **Esc:** go back one step (needs UX-002). Until then, re-show the selector.
- **Ctrl-C:** abort with a clear message, `cancelled; journal unchanged`, exit
  code 130, no `error:` prefix.
- **Enter with no match:** keep the query and explain, e.g. "no such account"
  (see UX-005 for creating one).

## Acceptance criteria

- Esc never discards entered data.
- Cancelling prints an explicit "journal unchanged" message.
- Unit tests with a fake `fzf` returning exit codes 1 and 130.

## Related

- UX-001, UX-002, UX-005.
