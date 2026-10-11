# BUG-007: `Reduce` mutates its input through shared map and slices

- **Type:** bug
- **Priority:** P2 — no visible failure today; blocks undo (UX-002)
- **Status:** open
- **Area:** `internal/addflow/state.go` (`State`, `ResidualState`),
  `internal/addflow/reducer.go` (`Reduce`)

## Symptom

`Reduce(state, cmd, settings)` takes and returns `State` by value, so it looks
pure. It is not: copies of `State` share mutable internals.

## Reproduction (probe test)

```go
s1 := /* state after initial "100 RUB" and SetBalancingAccount */
before := s1.Residual.Summary()                      // "-100 RUB"
_, _ = Reduce(s1, AddPostingExpression{Expression: "-40 RUB", ...}, settings)
after := s1.Residual.Summary()                       // "-60 RUB", s1 itself changed
```

## Root cause

- `ResidualState.items` is a map. Copying `State` copies the map header, so
  `state.Residual.MergeNegated(effect)` in `Reduce` writes into the caller's map.
- `state.BalancePostings = append(...)` and `state.NewCommodities = append(...)`
  write into the shared backing array whenever capacity allows. Two states
  derived from the same parent can then overwrite each other's last element.

## Impact

- The current runner is linear and never looks at an old state, so users are
  not affected yet.
- Any "go back" or undo built as a history of states (the natural design with a
  reducer, UX-002) will show corrupted residuals and postings.
- Tests that reuse a base state across subtests can contaminate each other.
- The code comments and `CLAUDE.md` describe `Reduce` as pure, which invites
  exactly that design.

## Proposed fix

Add `func (s State) Clone() State` that deep-copies `Residual.items`,
`Residual.order`, `BalancePostings` and `NewCommodities`, and call it at the top
of `Reduce`. Alternatively make `ResidualState` operations return new values.

## Acceptance criteria

- A test runs every command type against a parent state and asserts the parent
  is deep-equal to a snapshot taken before the call.
- Two branches derived from one parent do not affect each other.
- `CLAUDE.md` note about aliasing is removed once fixed.

## Related

- UX-002.
