# UX-008: Reduce hledger invocations per entry

- **Type:** improvement
- **Priority:** P3
- **Status:** open
- **Area:** `internal/app/app.go` (`addEffects`, `accountOptions`),
  `internal/addflow/runner.go`

## Measurement

On a synthetic 20,000-transaction journal (hledger 1.51.2) one invocation takes
about 0.5 s. A typical two-posting entry currently runs:

| Step                        | hledger calls                     |
|-----------------------------|-----------------------------------|
| primary account prompt      | 2 (`accounts`, `balance`)         |
| initial amount with `=`     | +1 (`balance acct:`)              |
| balancing account prompt    | 2 (`accounts`, `balance`)         |
| balancing amount hint       | 1 (`balance acct:`)               |
| validation                  | 1                                 |

That is 6–7 calls, or about 3 s of waiting spread across prompts. Declared
commodities are also re-read from disk for every amount.

## Proposed behaviour

- Load accounts, balances and declared commodities once per `fin add` session.
  The journal does not change during the interaction.
- Query a single account's balance only for `=target` and hints (date-bounded,
  BUG-002).

## Acceptance criteria

- A fake-hledger test counts invocations: at most 3 for a two-posting entry
  without `=`.

## Related

- BUG-010, ENG-002.
