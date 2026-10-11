# UX-001: Re-prompt on invalid input instead of aborting the transaction

- **Type:** improvement
- **Priority:** P1 — the biggest everyday pain
- **Status:** fixed (2026-09-22), with the exceptions below
- **Area:** `internal/addflow/runner.go`, `internal/ui/ui.go`

## Problem

Any error in any step ends `fin add` and discards everything entered:

- an amount typo (`10o`);
- an empty required description (`AskRequiredString`);
- a malformed date;
- answering "no" to "Commodity X is undeclared. Add to strict journal?";
- an account typed via the argument that is not found exactly.

A five-posting entry can be lost at the last step.

## Why it is cheap

`Reduce` returns `(state, error)` and a failed command leaves the logical state
unchanged. The loop in `addflow.Run` only has to show the error and ask the same
prompt again instead of returning. Prompter-level validation errors (date,
required string) can loop inside `ui` the same way.

## Proposed behaviour

- Show `✗ <error>` and repeat the prompt, pre-filling the previous input where
  the terminal allows it.
- Answering "no" to the commodity question returns to the amount prompt of the
  same posting.
- Only explicit cancellation (Ctrl-C, BUG-009) or an infrastructure failure
  (hledger missing, journal unreadable) aborts the command.

## Acceptance criteria

- Runner tests with scripted prompts: an invalid value followed by a valid one
  yields a completed transaction.
- The journal is never modified by an aborted run (existing guarantee).

## Related

- BUG-001, BUG-002, BUG-006, BUG-009, UX-002.

## Resolution (2026-09-22)

- `addflow.apply` turns a rejected command into a repeated prompt: the stage is
  unchanged and `State.Notice` says why. `NextPrompt` copies it into
  `PromptSpec.Notice`, and `internal/app` prints it as `! <reason>`.
- Declining a commodity declaration now drops only the posting that introduced
  the commodity and re-asks that amount (`revertPostingWithCommodity`), instead
  of ending the transaction.
- `ui.Prompter` re-asks after its own validation errors: an invalid date and an
  empty required value no longer end the command.
- Covered by `internal/addflow/runner_test.go` and the new `internal/ui` tests.

### Remaining

- A bad `--date` value or an account argument that matches nothing still aborts;
  both come from the command line rather than from a prompt.
- Esc in the account selector still aborts the entry (BUG-009), and there is no
  way back to an earlier step (UX-002).
