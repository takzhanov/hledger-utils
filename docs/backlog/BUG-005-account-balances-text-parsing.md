# BUG-005: Account balances parsed from text drop or misattribute accounts

- **Type:** bug
- **Priority:** P2 — wrong balance shown next to an account
- **Status:** open
- **Area:** `internal/hledger/hledger.go` (`ParseAccountBalances`, `AccountBalances`)

## Symptom

Balances shown for accounts (the `Selected: … (balance: …)` line) are missing
for some accounts and can be attached to the wrong account.

## Reproduction (probe test)

Input, in the shape of `hledger balance --flat --no-total` output:

```text
          100 RUB  assets:cash
           50 RUB  expenses:food and drink
          -10 RUB  equity
```

Result: `map[assets:cash:100 RUB]`. Both other accounts are lost.

## Root cause

The parser treats a line as "amount + account" only when its **last
whitespace-separated field contains `:`**. Otherwise the line is taken as an
extra amount line of a multi-commodity balance and kept in `pending`.

- Account names may contain single spaces in hledger (`expenses:food and drink`).
  The last field `drink` has no colon.
- Top-level accounts (`equity`, `income`) have no colon at all.

Such a line is not just dropped. Following the code, it is prepended to the
**next** account's balance, e.g. `liabilities:card` would be shown with
`50 RUB expenses:food and drink, -10 RUB equity, 20 RUB`.

## Proposed fix

Use `hledger balance --flat --no-total -O json`, as `BalanceByCommodity` already
does. Shape verified with hledger 1.51.2: `[rows, totals]`, where each row is
`[fullName, displayName, indent, [amount…]]` and each amount has `acommodity`
and `aquantity.decimalMantissa` / `decimalPlaces`. Share the amount-decoding
code with `ParseBalanceByCommodityJSON`, and format with `domain.FormatAmount`.

## Acceptance criteria

- Tests with an account name containing spaces, a top-level account, and a
  multi-commodity account; every account maps to exactly its own balance.
- The text parser is deleted.

## Related

- BUG-010 (balances in the selector), UX-008 (fewer hledger calls).
