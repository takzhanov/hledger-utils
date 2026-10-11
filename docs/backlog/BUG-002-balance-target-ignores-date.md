# BUG-002: `=target` computes the delta from the balance on all dates

- **Type:** bug
- **Priority:** P2 — backdated reconciliation fails and the entry is lost
- **Status:** open
- **Area:** `internal/addflow/reducer.go` (`buildBalanceTargetPosting`),
  `internal/addflow/effect.go` (`NeedBalanceByCommodity`),
  `internal/hledger/hledger.go` (`BalanceByCommodity`)

## Symptom

A balance-target entry (`=1000 RUB`) dated before existing transactions for the
same account produces a wrong posting amount and is rejected by validation.

## Reproduction (verified with hledger 1.51.2)

```journal
commodity 1,000.00 RUB
account assets:cash
account expenses:food and drink
account equity:opening

2026-09-01 open
    assets:cash    1000 RUB
    equity:opening

2026-09-10 lunch
    expenses:food and drink    300 RUB
    assets:cash
```

1. `fin add --date 2026-09-05`, account `assets:cash`, amount `=1000 RUB`.
2. fin asks `hledger balance acct:^assets:cash$`, which returns `700 RUB`
   (it already includes the 09-10 lunch).
3. fin writes `assets:cash    300 RUB = 1000 RUB`.
4. On 09-05 the real balance is 1000, so after the posting it is 1300.
   `hledger bs --strict` fails: `Balance assertion failed in assets:cash`.
5. The whole interaction is discarded (see UX-001).

The "Current effective balance" hint shown before the balancing amount has the
same problem.

## Root cause

`NeedBalanceByCommodity` carries only the account, and `BalanceByCommodity`
runs `hledger balance` without an end date, so the balance includes every
posting, including later and future-dated ones.

## Proposed fix

- Add the transaction date to `NeedBalanceByCommodity`, and in
  `BalanceByCommodity` pass `-e <date + 1 day>` (the end date is exclusive).
  hledger checks assertions in date order, and on the same date in parse order.
  fin appends at the end of the main file, so "all postings dated on or before
  the transaction date" is the correct base.
- If a later assertion on the same account then fails, report it explicitly
  ("this entry shifts the balance checked by the assertion on 2026-09-20").
  That failure is correct accounting, not a fin bug.

Alternative considered: write a native hledger balance assignment
(`assets:cash    = 1000 RUB` with no amount). `hledger check --strict` accepts
it and computes the amount in date order. It is always correct, but the amount
is invisible in the file, and fin still needs the date-correct delta to balance
the remaining postings. It is therefore not a replacement for the fix above.

## Acceptance criteria

- The reproduction above writes `0 RUB = 1000 RUB` (or the correct delta) and
  passes validation.
- A unit test asserts that the date reaches the balance query (the argument list
  contains `-e`).

## Related

- UX-004 (reconciliation workflow).
