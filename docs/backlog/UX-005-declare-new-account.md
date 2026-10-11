# UX-005: Declare a new account inline

- **Type:** improvement
- **Priority:** P3
- **Status:** open
- **Area:** `internal/addflow`, `internal/ui/ui.go`, `internal/journal/journal.go`

## Problem

Account selection offers only existing accounts (`hledger accounts`). In a
strict journal a new account needs an `account` directive. fin supports exactly
this deferred-declaration flow for commodities, but not for accounts, so a new
account means leaving fin, editing the journal, and starting over.

## Proposed behaviour

- In the selector, Enter on a query with no match (fzf `--print-query`) offers
  `Account "expenses:pets" is undeclared. Add to strict journal? [y/N]`.
- Accepted accounts accumulate in state like `NewCommodities` and are written
  with the entry in the same atomic write (`account expenses:pets`).
- Use `hledger accounts --declared` (available in 1.51.2) to decide what is
  declared.

## Acceptance criteria

- The new directive and the entry are validated and written together;
  cancelling writes neither.

## Related

- BUG-009 (no-match handling), BUG-003.
