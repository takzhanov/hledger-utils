# UX-006: Choose which file receives the entry

- **Type:** improvement
- **Priority:** P3
- **Status:** open — needs a design decision
- **Area:** `internal/app/app.go`, `internal/journal`

## Problem

fin always appends to the main journal. A common layout is a small main file
with `include 2026.journal` and similar, where new entries belong in the current
year's file.

## Design constraint

Validation must see the candidate under its **real path**, because the main
journal includes that file by name. Options:

1. **Copy the include tree into a temp dir** and validate there. This breaks
   for absolute include paths.
2. **Write, validate, restore on failure.** This briefly violates "a failed
   interaction does not modify the journal".
3. **Validate the included file standalone, then the whole journal after the
   atomic write**, with automatic rollback. This gives stronger guarantees than
   option 2, but is still not pure.

## Proposed behaviour

`--append-to PATH` (a flag, consistent with the no-config-file principle).
Pick one of the validation strategies above first.

## Related

- BUG-003.
