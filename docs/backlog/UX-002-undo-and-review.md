# UX-002: Go back, remove a posting, confirm before append

- **Type:** improvement
- **Priority:** P2
- **Status:** open (blocked by BUG-007)
- **Area:** `internal/addflow`

## Problem

Input only moves forward. A wrong account or amount can be fixed only by
aborting the whole entry. The entry is appended right after rendering, without
an explicit confirmation. `hledger add` already supports `<` to undo a step, so
fin is behind its baseline here.

## Proposed behaviour

- `<` (or Esc in the selector, see BUG-009) steps back to the previous prompt
  with the previous value as the default.
- Before writing, show the rendered entry with actions: `[y]es append`,
  `[e]dit posting N`, `[d]elete posting N`, `[n]o cancel`.
- Implement as a stack of `State` snapshots in the runner. The reducer stays
  unchanged. This requires states to be independent copies (BUG-007).

## Acceptance criteria

- Runner tests: back-step restores the previous state exactly; deleting a
  posting recomputes the residual; declining confirmation leaves the journal
  untouched.

## Related

- BUG-007, BUG-009, UX-001; PRODUCT.md next area 2.
