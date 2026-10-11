# BUG-008: `fin help` requires a journal; `fin check` runs fin's own include parser first

- **Type:** bug
- **Priority:** P3
- **Status:** open
- **Area:** `internal/app/app.go` (`Run`, `parseRootArgs`)

## Symptom

1. In an environment without a journal, `fin help` prints
   `error: journal file not found …` and exits with 1 (verified by running
   the binary). The same happens for `fin add --help`.
2. `fin check` on a journal whose `include` pattern matches nothing prints
   fin's own `include "…" matched no files`, not hledger's diagnostic with
   file and line context.

## Root cause

`Run` resolves the journal inside `parseRootArgs` **before** looking at the
subcommand. It then always calls `runner.DefaultCommodity(journalFile)`, which
walks includes with fin's own parser, before dispatching to `check` or `add`.
`check` never uses the default commodity.

## Proposed fix

- Dispatch `help` (and `-h` of subcommands) before resolving the journal.
- Resolve the journal lazily per command.
- Compute the default commodity only in `runAdd`.

## Acceptance criteria

- `fin help` and `fin add --help` work with no journal present.
- `fin check` on a broken include shows hledger's error message.
