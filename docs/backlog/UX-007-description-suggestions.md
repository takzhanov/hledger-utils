# UX-007: Suggest descriptions and postings from history

- **Type:** improvement
- **Priority:** P2 — main speed-up for frequent entry
- **Status:** open
- **Area:** `internal/addflow`, `internal/ui`, `internal/hledger`

## Problem

- The description prompt defaults to the literal `by_fin`. Pressing Enter
  writes a meaningless description into the journal.
- There is no completion from past descriptions. `hledger add` offers it, and
  it also pre-fills postings from the most similar past transaction.

## Proposed behaviour

- Remove the `by_fin` default; the description is required, or defaults to
  the last used one.
- Offer past descriptions through fzf (`hledger descriptions`, most recent
  first).
- When a chosen description matches a past transaction, offer its accounts as
  the default primary and balancing accounts (amounts are still asked).

## Acceptance criteria

- No entry is written with `by_fin`.
- Selecting a past description pre-selects the accounts of the last matching
  transaction.

## Related

- UX-009 (positioning against `hledger add`), PRODUCT.md next area 3.
