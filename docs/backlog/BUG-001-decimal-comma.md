# BUG-001: Decimal comma is silently read as a thousands separator

- **Type:** bug
- **Priority:** P1 — silently wrong amounts
- **Status:** fixed (2026-09-22)
- **Area:** `internal/domain/amount.go` (`parseDecimalToken`)

## Symptom

Amounts typed with a comma as the decimal mark are accepted without any error,
but the value is multiplied by 10–1000:

| Input          | Parsed value | Expected                    |
|----------------|--------------|-----------------------------|
| `1000,50 RUB`  | `100050`     | `1000.50` or an error       |
| `1,5`          | `15`         | `1.5` or an error           |
| `1.000,00 EUR` | `1`          | `1000.00` or an error       |
| `1 000 RUB`    | error        | (acceptable, not silent)    |

Verified with a probe test calling `domain.ParseAmountInput(input, "RUB", false)`.

## Root cause

`parseDecimalToken` removes every `,`, NBSP (`U+00A0`), narrow NBSP (`U+202F`)
and space from the token before calling `decimal.NewFromString`. Separators are
discarded regardless of their position, so a decimal comma becomes part of the
integer digits.

## Impact

This is the most dangerous defect in the project because nothing downstream
catches it:

- The balancing-amount default is derived from the same wrong value. Accepting
  the default yields a balanced transaction.
- `hledger --strict` validation passes, because the transaction balances.
- The error surfaces only later in reports, possibly after more entries were
  built on top of it.

Comma decimals are the normal way of typing amounts in RUB/EUR locales, which is
the primary use case.

A related inconsistency: the generated directive `commodity 1,000.00 X`
(`journal.AppendDirectivesAndEntry`) always declares `.` as the decimal mark,
even in a journal that uses `,`.

## Proposed fix

1. Never drop a separator silently. Accept `,` only as a digit-group separator
   in well-formed groups (`^-?\d{1,3}(,\d{3})+(\.\d+)?$`); any other comma is
   an error such as `ambiguous amount "1000,50": use "." as the decimal mark`.
2. Better: honour the journal's decimal mark. hledger exposes it
   (`decimal-mark` directive, and `astyle.asdecimalmark` in `-O json` output of
   commodity-bearing reports). When the journal mark is `,`, swap the roles of
   `,` and `.`.
3. Apply the same group validation to NBSP / narrow-NBSP grouping.
4. Generate `commodity` directives in the journal's decimal-mark style.

## Acceptance criteria

- Table test in `internal/domain` covering all inputs above for both decimal
  marks; no input is reinterpreted silently.
- `1,000.50` still parses as `1000.50` under the `.` mark.
- A generated `commodity` directive round-trips through `hledger check --strict`
  in a comma-decimal journal.

## Related

- UX-001 (a rejected amount should re-prompt instead of aborting).

## Resolution (2026-09-22)

- `domain.AmountFormat` carries the journal's decimal mark. The other separator
  is accepted only in well-formed thousands groups; anything else is an explicit
  error, so no input is reinterpreted.
- `hledger.JournalStyle` reads the mark from `decimal-mark`, then from the
  sample amount of `D`, then from the first `commodity` directive.
- Amounts stay canonical in memory and are converted where they reach the
  journal (`journal.RenderTransaction`, `journal.CommodityDirective`) or the
  user (`ResidualState.Summary`/`DefaultPrompt`). This was necessary, not
  cosmetic: hledger 1.51.2 parses `10.5` as `105` in a comma-mark journal,
  whether the style comes from `decimal-mark` or from a `commodity` directive.
- Verified end to end against hledger 1.51.2: `=1000,50 RUB` in a comma-mark
  journal writes `0 RUB = 1000,5 RUB`, leaves the reported balance unchanged and
  passes `hledger check --strict`; a new commodity is declared as
  `commodity 1.000,00 BTC`; the same input in a dot-mark journal is rejected and
  re-asked.
