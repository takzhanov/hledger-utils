# UX-003: Offer an exchange conversion when two commodities remain

- **Type:** improvement
- **Priority:** P2 — exchange is a stated product goal
- **Status:** open
- **Area:** `internal/addflow/reducer.go`, `internal/addflow/prompt.go`

## Problem

A natural exchange entry:

```text
assets:usd   -100 USD
assets:rub   9000 RUB
```

never balances in fin. The residual stays `{USD: 100, RUB: -9000}` and fin keeps
asking for another balancing account. hledger 1.51.2 also rejects it
(`This multi-commodity transaction is unbalanced. Automatic commodity conversion
is not enabled.`), so a cost annotation is required.

Today the only way is to know upfront that the first amount must be typed as
`-100 USD @@ 9000 RUB`. Nothing in the flow suggests it.

## Proposed behaviour

When the residual contains exactly two commodities with opposite signs and no
other residual, offer:

```text
Remaining: 100 USD, -9000 RUB
Record as exchange (-100 USD @@ 9000 RUB, rate 90)? [Y/n]
```

On yes, rewrite the posting that introduced the first commodity with an `@@`
cost, recompute the residual (now zero), and continue. A fee posting afterwards
still works as usual.

## Acceptance criteria

- Reducer test: the two-commodity residual plus acceptance gives a balanced
  transaction that passes `hledger check --strict`.
- The rendered rate is informational only; exact amounts use `@@`, so no
  rounding is introduced.

## Related

- PRODUCT.md next area 1 (exchange recipe).
