# Product direction

## Purpose

`fin` is an interactive transaction-entry layer for hledger. It is not a
reporting application, ledger database, or replacement parser for the hledger
journal format.

The main product goal is to make complex but common entries fast to capture:

- balance reconciliation with assertions;
- transfers and multi-posting transactions;
- currency exchange and fees;
- priced asset purchases and sales;
- temporary uncategorized postings that can be refined later.

## Principles

1. hledger is authoritative for parsing, balancing, and strict validation.
2. Journal files remain plain text and human-editable.
3. The common path is interactive and optimized for frequent entry.
4. Advanced hledger amount syntax should remain available instead of being
   hidden behind many specialized forms.
5. A failed or cancelled interaction must not modify the journal.
6. Keep the public command surface small. Add a command only when it provides a
   distinct workflow that cannot remain a mode or shortcut of `add`.

## Current scope

- `fin add`: interactive smart transaction builder.
- `fin check`: strict journal validation.
- Journal selection identical to hledger: `--journal`/`-f`, `LEDGER_FILE`,
  `~/.hledger.journal`.
- Account selection through mandatory `fzf`.
- Exact decimal arithmetic, balance assertions, priced amounts, comments, and
  deferred commodity declarations.

## Non-goals for the current version

- Editing or rewriting historical transactions.
- Reimplementing the hledger parser.
- Reports already expressed clearly by a direct hledger command.
- A permanent application-specific configuration format.

## Next development areas

Concrete bugs and improvements are tracked in [`backlog/`](backlog/README.md).

1. Transaction recipes on top of `add`: reconcile, transfer, exchange, and
   investment trade, without duplicating the core state machine.
2. Better recovery inside the interactive flow: retry invalid values, go back,
   remove a posting, and confirm before append.
3. Account aliases and recent-account ranking derived from the journal rather
   than from a separate config file.
4. Integration tests with a fake hledger binary plus a small real-journal test
   fixture.
5. Optional atomic replacement and backup for journal writes if concurrent or
   external journal editing becomes a real workflow.
