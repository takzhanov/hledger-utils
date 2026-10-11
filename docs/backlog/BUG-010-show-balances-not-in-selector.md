# BUG-010: `--show-balances` does not show balances in the selector

- **Type:** bug (UX)
- **Priority:** P3
- **Status:** open
- **Area:** `internal/ui/ui.go` (`askAccountWithFZF`), `internal/app/app.go`
  (`accountOptions`)

## Symptom

The flag help says "show account balances in selectors". In fzf only the
account name is visible. The balance appears in the `Selected: …` line after
the choice is made, when it no longer helps the choice.

`--show-balances=false` does not save any work: `hledger balance` still runs on
every account prompt.

## Root cause

- Lines fed to fzf are `<styled account>\t<account>` with `--with-nth=1`, and
  the balance is not part of the line.
- `accountOptions` always calls both `runner.Accounts` and
  `runner.AccountBalances`, independent of the flag.

## Proposed fix

- Render `account<TAB>balance` as the visible columns (for example `--with-nth=1,2`
  with padding), and keep a hidden field with the raw account name for the
  result.
- Skip `AccountBalances` when balances are disabled.
- Depends on BUG-005, because wrong balances must not be shown more prominently.

## Acceptance criteria

- Balances are visible while searching when the flag is on.
- No `hledger balance` call happens when it is off (fake-hledger test).

## Related

- BUG-005, UX-008.
