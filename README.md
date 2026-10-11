# fin-hledger-util

`fin` is a focused CLI for smart, interactive entry of transactions into an
existing hledger journal. hledger remains the accounting engine and source of
truth; `fin` helps select accounts, calculate balancing postings, validate the
result, and append it safely.

## Requirements

- Go 1.25 or newer for building from source.
- `hledger` available in `PATH`.
- `fzf` for interactive account selection.

`fin add` checks for `fzf` before starting. If it is missing and Homebrew is
available, `fin` installs it with `brew install fzf`. Without Homebrew, install
`fzf` manually and retry.

## Install

```sh
go install github.com/takzhanov/hledger-utils/cmd/fin@latest
```

## Build from source

```sh
go build -o fin ./cmd/fin
```

## Journal selection

`fin` opens the same journal as `hledger`, in this order:

1. `--journal PATH` or `-f PATH`.
2. The `LEDGER_FILE` environment variable.
3. `~/.hledger.journal`.

A leading `~/` is expanded in both the flag and `LEDGER_FILE`. The current
directory is never searched, so the journal `fin add` writes is always the one
that `hledger` reports on.

Examples:

```sh
./fin check
./fin -f ~/finance/main.journal check
LEDGER_FILE=~/finance/main.journal ./fin add
```

There is no separate `fin` configuration file. Non-default behavior is passed
through command-line flags.

Journal files are private data and live outside this repository. `.gitignore`
rejects `*.journal` everywhere except `testdata/` directories, which may hold
only clearly synthetic fixtures.

## Commands

### `fin check`

Runs `hledger check --strict` for the selected journal.

### `fin add`

Starts an interactive transaction builder:

```sh
./fin add
./fin add assets:cash
./fin add --date 2026-06-19 --description "cash reconciliation"
./fin add --dry-run
./fin add --show-balances=false
```

The flow asks for a date, description, primary account and amount expression.
It then shows the unbalanced remainder and collects additional postings until
the transaction is balanced. Before writing, `fin` renders the complete entry
and validates a temporary copy with `hledger bs -tSV --strict`.

Dates can be entered as `DD` (the current month and year), `MM-DD` (the current
year), or the full `YYYY-MM-DD`. After an abbreviated interactive date, `fin`
prints the resolved full date on the next line. The same formats are accepted
by `--date`; an empty interactive value keeps today's displayed default.

Supported amount expressions:

```text
1000                  amount in the journal's default commodity
1000 USD              explicit commodity
USD 1000              commodity-first form
=2000 USD             make the selected account end at 2000 USD
20 AAPL @ 150 USD     20 units at a unit price of 150 USD
20 AAPL @@ 3000 USD   20 units with a total cost of 3000 USD
1000 USD  ; note      posting with an inline comment
```

An inline comment is separated from the expression by two spaces followed by
`;`. The default commodity comes from the journal's `D` directive. Bare numbers
remain commodity-less when no default is declared.

If an expression uses an undeclared commodity, `fin` asks whether to add its
`commodity` directive. New directives and the transaction are validated and
written together; cancelling or failing later does not leave a partial change.

### Number punctuation

`fin` reads and writes numbers the way the journal already does. The decimal
mark comes from the journal's `decimal-mark` directive, or from the sample
amount of its `D` and `commodity` directives, and defaults to `.`. The other
separator may then only group thousands:

| Journal decimal mark | `1,000.50` | `1000,50` | `1.000,50` |
|----------------------|------------|-----------|------------|
| `.` (default)        | 1000.50    | rejected  | rejected   |
| `,`                  | rejected   | 1000.50   | 1000.50    |

Nothing is reinterpreted silently: an amount that does not match the journal's
punctuation is rejected with an explanation instead of being read as a
different number. Generated entries and `commodity` directives are written in
the same punctuation, because hledger reads `10.5` in a comma-mark journal as
`105`.

### Invalid input

A rejected value does not discard the transaction. An amount `fin` cannot read,
an invalid date and an empty description repeat their own prompt with the
reason, and answering "no" to the commodity question returns to the amount of
that posting. Ctrl-C still cancels, leaving the journal unchanged.

Run `./fin --help` or `./fin add --help` for the command reference.

## Project structure

- `cmd/fin`: executable entry point.
- `internal/app`: command parsing and workflow orchestration.
- `internal/addflow`: state machine for interactive transaction construction.
- `internal/domain`: amounts and transaction model.
- `internal/hledger`: hledger process adapter and JSON parsing.
- `internal/journal`: journal rendering, candidate assembly, and file operations.
- `internal/ui`: terminal prompts and fzf account selection.

Product boundaries and the next development areas are documented in
[`docs/PRODUCT.md`](docs/PRODUCT.md).

## License

[MIT](LICENSE)
