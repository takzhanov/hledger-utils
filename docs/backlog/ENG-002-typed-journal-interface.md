# ENG-002: Replace `any`-typed effect requests with a typed interface

- **Type:** engineering
- **Priority:** P3 — do it when touching the runner, not as a standalone refactor
- **Status:** open
- **Area:** `internal/addflow/effect.go`, `internal/addflow/runner.go`,
  `internal/app/app.go`, `internal/ui/ui.go`

## Problems

- `EffectResolver.ResolveEffect(EffectRequest) (any, error)` plus type
  assertions such as `resolved.([]AccountOption)` panic when a resolver returns
  `nil`. The test stub `stubEffects` does exactly that for unknown keys.
- `hydratePrompt` swallows `NeedBalanceByCommodity` errors silently (no hint and
  no message).
- `hydrateInitialExpression` and `hydrateAddPostingExpression` are
  line-for-line duplicates.
- `AccountOption` exists in both `ui` and `addflow`, and `app` copies fields
  back and forth twice.

## Proposed change

```go
type Journal interface {
    AccountOptions() ([]AccountOption, error)
    BalanceAt(account string, date time.Time) (map[string]decimal.Decimal, error)
    DeclaredCommodities() (map[string]struct{}, error)
}
```

This is three methods with compile-time types, and the effect-request structs go
away. Keep a single `AccountOption` type (in `addflow`, or a small shared
package) and let `ui` accept it.

## Acceptance criteria

- No `any` or type assertions in `addflow`.
- Hydration functions merged.
- Existing tests pass with a typed stub.
