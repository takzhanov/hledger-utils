# UX-009: Document positioning against `hledger add`

- **Type:** product / docs
- **Priority:** P3
- **Status:** open
- **Area:** `docs/PRODUCT.md`

## Problem

The closest alternative to `fin add` is the built-in `hledger add`, which
already has description completion, postings pre-filled from similar
transactions, amount defaults and `<` undo. PRODUCT.md does not mention it, so
roadmap items risk re-implementing what users already have instead of
deepening what is unique.

## What is unique to fin today

- fzf account selection with balances.
- `=target` balance entry that computes the delta.
- Strict validation of the complete candidate before writing.
- Deferred commodity declaration written atomically with the entry.

## Proposed change

Add a "Compared to `hledger add`" section to PRODUCT.md: the parity features
fin must have (UX-001, UX-002, UX-007) and the differentiators it invests in
(recipes, reconciliation UX-004, exchange UX-003).
