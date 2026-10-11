# UX-004: Reconciliation support: status marks and inclusive assertions

- **Type:** improvement
- **Priority:** P2
- **Status:** open
- **Area:** `internal/domain/domain.go`, `internal/journal/journal.go`,
  `internal/addflow`

## Problem

Balance reconciliation is the first product goal. The entries fin writes lack
two standard hledger tools for it:

- **Transaction status.** Reconciled entries are normally marked cleared (`*`)
  or pending (`!`). `domain.Transaction` has no status, and `RenderTransaction`
  cannot emit it.
- **Assertion kind.** fin always writes `=` (single commodity, subaccounts
  excluded). Reconciling a parent account with subaccounts needs `=*`
  (inclusive), and a full multi-commodity check needs `==`.

## Proposed behaviour

- `--status cleared|pending` flag, and a default of `*` for entries containing
  a balance target.
- Accept `=*target` and `==target` in amount expressions, with the delta
  computed from the matching balance (inclusive query for `=*`).

## Acceptance criteria

- Rendering tests for status and each assertion kind; the output passes
  `hledger check --strict`.

## Related

- BUG-002 (date-correct balance), PRODUCT.md next area 1 (reconcile recipe).
