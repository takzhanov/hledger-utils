# BUG-006: Commodity symbols are handled differently from hledger

- **Type:** bug
- **Priority:** P3 — edge cases, but they end in a lost entry or a duplicate directive
- **Status:** open
- **Area:** `internal/hledger/hledger.go` (`extractCommodityToken`,
  `DeclaredCommodities`, `DefaultCommodity`), `internal/domain/amount.go`
  (`ParseAmountInput`)

## hledger rules (verified with hledger 1.51.2)

- An **unquoted** symbol cannot contain digits or `-`:
  `1 BTC2` → `unexpected '2'`; `commodity 1,000.00 T-BILL2` → `unexpected '-'`.
- A **quoted** symbol may contain spaces, digits and punctuation:
  `commodity 1.00 "VANGUARD 500"` is valid.
- `hledger commodities --declared` lists declared symbols, quoted ones
  included (printed without quotes: `VANGUARD 500`).

## Defects in fin

1. **fin accepts symbols hledger rejects.** `20 BTC2` or `5 T-BILL` pass
   `ParseAmountInput`. fin then asks to declare the commodity, and the final
   strict validation fails with a parse error. The whole entry is lost.
2. **Quoted symbols with spaces cannot be entered.** `2 "VANGUARD 500"` splits
   into three fields and is rejected as an invalid expression.
3. **fin's own directive reader mangles symbols.** `extractCommodityToken`
   deletes digits, `,`, `.` and `-` **anywhere** in the text, so a declared
   `"S&P500"` is read as `"S&P"`. A posting in `"S&P500"` then looks
   undeclared, and fin offers to append a second `commodity` directive for an
   already declared symbol. (Whether hledger rejects the duplicate was not
   verified.)
4. The reader re-implements journal parsing, which contradicts product
   principle 1 ("hledger is authoritative for parsing").

## Proposed fix

- Take declared commodities from `hledger commodities --declared -f <journal>`
  and drop `extractCommodityToken` for that purpose. Normalise quotes when
  comparing with posting symbols.
- In `ParseAmountInput`, accept a quoted symbol as one token. Reject an unquoted
  symbol with digits or `-` with a hint (`use "BTC2"`), or quote it
  automatically when rendering.
- `DefaultCommodity` (the `D` directive) has no hledger command. Keep the text
  reader for it, but tokenise the amount properly instead of deleting characters.

## Acceptance criteria

- Tests: `"VANGUARD 500"` and `"S&P500"` are recognised as declared; `BTC2` is
  rejected (or quoted) at input; no duplicate directive is generated.

## Related

- ENG-002.
