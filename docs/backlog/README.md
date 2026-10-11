# Backlog

Known bugs and planned improvements, one file per item. Update `Status` in the
item and in this table when work starts or finishes.

Priority scale:

- **P1:** silent data damage, or the most frequent daily blocker.
- **P2:** breaks or loses work in a common workflow.
- **P3:** edge cases, inconsistencies, internal quality.

## Bugs

| ID | Title | Priority | Status |
|----|-------|----------|--------|
| [BUG-001](BUG-001-decimal-comma.md) | Decimal comma is silently read as a thousands separator | P1 | fixed 2026-09-22 |
| [BUG-002](BUG-002-balance-target-ignores-date.md) | `=target` computes the delta from the balance on all dates | P2 | open |
| [BUG-003](BUG-003-non-atomic-journal-write.md) | Journal write is not atomic | P1 | open |
| [BUG-004](BUG-004-inconsistent-validation.md) | Pre-write validation differs from `fin check`; temp file left on interrupt | P3 | open |
| [BUG-005](BUG-005-account-balances-text-parsing.md) | Account balances parsed from text drop or misattribute accounts | P2 | open |
| [BUG-006](BUG-006-commodity-symbols.md) | Commodity symbols are handled differently from hledger | P3 | open |
| [BUG-007](BUG-007-reduce-state-aliasing.md) | `Reduce` mutates its input through shared map and slices | P2 | open |
| [BUG-008](BUG-008-help-requires-journal.md) | `fin help` requires a journal | P3 | open |
| [BUG-009](BUG-009-esc-aborts-transaction.md) | Esc in the account selector aborts the whole transaction | P2 | open |
| [BUG-010](BUG-010-show-balances-not-in-selector.md) | `--show-balances` does not show balances in the selector | P3 | open |

## Improvements

| ID | Title | Priority | Status |
|----|-------|----------|--------|
| [UX-001](UX-001-reprompt-on-invalid-input.md) | Re-prompt on invalid input instead of aborting | P1 | fixed 2026-09-22 (see item) |
| [UX-002](UX-002-undo-and-review.md) | Go back, remove a posting, confirm before append | P2 | blocked by BUG-007 |
| [UX-003](UX-003-currency-exchange-shortcut.md) | Offer an exchange conversion when two commodities remain | P2 | open |
| [UX-004](UX-004-reconciliation-marks.md) | Reconciliation: status marks and inclusive assertions | P2 | open |
| [UX-005](UX-005-declare-new-account.md) | Declare a new account inline | P3 | open |
| [UX-006](UX-006-append-target-file.md) | Choose which file receives the entry | P3 | needs design |
| [UX-007](UX-007-description-suggestions.md) | Suggest descriptions and postings from history | P2 | open |
| [UX-008](UX-008-fewer-hledger-calls.md) | Reduce hledger invocations per entry | P3 | open |
| [UX-009](UX-009-positioning-vs-hledger-add.md) | Document positioning against `hledger add` | P3 | open |
| [ENG-001](ENG-001-write-path-integration-tests.md) | Integration tests for the write path with a fake hledger | P2 | open |
| [ENG-002](ENG-002-typed-journal-interface.md) | Replace `any`-typed effects with a typed interface | P3 | open |

## Suggested order

1. ~~BUG-001 decimal comma, then UX-001 re-prompt~~ — done 2026-09-22.
2. BUG-003 atomic write, together with BUG-004 single validation.
3. BUG-002 date-bounded balance, BUG-005 JSON balances.
4. BUG-009 Esc semantics, BUG-007 state cloning, then UX-002 undo.
5. ENG-001 alongside each of the above, as the write-path safety net.
