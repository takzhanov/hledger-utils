# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```sh
go build -o fin ./cmd/fin   # build (the ./fin binary is gitignored)
go test ./...               # all tests
go test ./internal/addflow  # one package
go test ./internal/addflow -run TestReduceInitialExpressionSetsResidual
go vet ./...
gofmt -l .
```

There is no Makefile, CI config, or lint config. Tests are pure Go with no
external dependencies: `hledger` and `fzf` are never invoked from tests — the
subprocess-touching code is tested by parsing captured output strings
(`ParseBalanceByCommodityJSON`, `ParseAccountBalances`) or by writing fake
executables into a `t.TempDir()` and `t.Setenv("PATH", ...)` (see
`internal/ui/ui_test.go`).

## Architecture

`fin` is an interactive front-end for entering one transaction into an existing
hledger journal. hledger stays authoritative for parsing, balancing, and
validation; `fin` only assembles text and asks hledger whether it is valid.

Dependency direction: `cmd/fin` → `internal/app` → `internal/addflow` →
`internal/domain`. `app` also wires `hledger`, `journal`, and `ui`.

### The addflow state machine (the core of the codebase)

`internal/addflow` is deliberately pure: it knows nothing about hledger, fzf,
stdin, or files. It is structured as three separable pieces:

- `state.go` — `State` with a `Stage` enum, plus `ResidualState`, the
  per-commodity running remainder that drives what still needs balancing.
  `ResidualState` preserves insertion order and deletes zeroed commodities, so
  a commodity that cycles to zero and back does not duplicate in `Summary()`.
- `prompt.go` — `NextPrompt(state)` maps a `Stage` to a `PromptSpec` (label,
  default, hints). This is the only place prompt wording lives.
- `reducer.go` — `Reduce(state, cmd, settings)` is the transition function. It is
  meant to be pure, but `State` copies currently share `ResidualState`'s map and
  the posting slices (docs/backlog/BUG-007) — clone before keeping state history. All
  business rules live here: balance-target (`=`) resolution, zero-amount
  postings returning to account selection, deferred commodity declarations.

Side effects are inverted through two interfaces rather than called directly:

- `Prompts` (asks the user) — implemented by `addPrompts` in `internal/app`.
- `EffectResolver` (asks the journal) — implemented by `addEffects` in
  `internal/app`, which forwards `NeedDeclaredCommodities`,
  `NeedAccountOptions`, and `NeedBalanceByCommodity` to the hledger runner.

`runner.go` drives the loop: hydrate a prompt with effect data → ask → build a
`Command` (hydrating it with balances/declared commodities via
`RequiredEffectsForCommand`) → `Reduce`. A `Reduce` error never ends the flow:
`apply` keeps the stage unchanged and puts the reason in `State.Notice`, which
`NextPrompt` passes to `PromptSpec.Notice` so the same prompt is asked again.
Only prompt and effect failures (EOF, interrupt, hledger errors) abort. **When adding a step, add a `Stage`, a
`PromptKind`, a `Command`, and the `Reduce` case; do not call hledger or read
stdin from inside `addflow`.** Tests stub both interfaces (`stubEffects`) and
assert on states, never on I/O.

Balance hints account for in-flight postings: `effectiveBalancesForAccount`
takes hledger's stored balance and folds in postings already staged in the
current unwritten transaction, so `=target` expressions and the displayed
"current effective balance" reflect the transaction being built.

### Atomicity of the write

A cancelled or failed interaction must never modify the journal. `app.go`
enforces this by never appending in place: it reads the original file, builds a
full candidate byte slice (`journal.AppendDirectivesAndEntry` — new `commodity`
directives *and* the transaction together), writes that candidate to a temp file
next to the target, runs `hledger bs -tSV --strict` on it, and only then
`journal.Replace`s the real file. New commodity declarations accumulate in
`State.NewCommodities` during the flow and are committed in that same single
write, which is why the commodity question is deferred rather than answered by
editing the journal mid-flow.

### hledger adapter

`internal/hledger` shells out to the `hledger` binary, always with
`cmd.Dir = filepath.Dir(journalFile)` so relative `include` paths resolve.
Note two separate check methods: `CheckStrict` (`hledger check --strict`, the
`fin check` command) and `StrictCheck` (`hledger bs -tSV --strict`, the
pre-write candidate validation).

`JournalStyle` (default commodity plus decimal mark) and `DeclaredCommodities`
do *not* shell out — they read
journal text directly via `walkIncludedJournals`, which follows only `include`
directives (glob-expanded, cycle-guarded, comment-stripped) from the main file.
It errors when an include pattern matches nothing. Do not replace this with a
directory scan: traversal is intentionally limited to actually-included files.

### Amount expressions

`internal/domain/amount.go` parses the user-facing amount syntax (`1000`,
`1000 USD`, `USD 1000`, `20 AAPL @ 150 USD`, `@@` total cost) with
`shopspring/decimal` — exact decimal arithmetic, no floats anywhere in amount
handling. `=target` and inline `  ; comment` are handled in `addflow/reducer.go`,
not in the amount parser.

`AmountFormat` carries the journal's decimal mark (from `hledger.JournalStyle`).
**Amounts are canonical (`.`, no grouping) everywhere inside fin**, and are
converted only where text reaches the journal (`journal.RenderTransaction`,
`journal.CommodityDirective`) or the user (`ResidualState.Summary` /
`DefaultPrompt`). Keep that invariant: re-parsing a rendered posting uses
`ParseAmountInput`/`ParsePricedAmountInput` (canonical), while user input goes
through `settings.Format`. It matters because hledger reads `10.5` as `105` in a
journal whose declared decimal mark is `,`.

`domain.Transaction.Validate` allows a single-posting transaction only when it
is a zero-delta balance assertion.

## Conventions

- Journal files are private data and live outside the repository; `.gitignore`
  rejects `*.journal` except under `testdata/`. Any journal a test needs must be a
  clearly synthetic fixture in `t.TempDir()` or a package-level `testdata/`.
- Journal resolution deliberately mirrors hledger: `--journal`/`-f`,
  `LEDGER_FILE`, `~/.hledger.journal`, never the working directory. `fin`
  validates with `hledger -f`, so any divergence would mean writing a file other
  than the one `hledger` reports on.
- No application config file exists or should be added — non-default behavior is
  passed as flags.
- Known bugs and planned improvements live in `docs/backlog/`, one file per item
  with the index in its `README.md`. Update an item's status when fixing it.
- Keep the command surface small (`add`, `check`); new workflows should be modes
  or shortcuts of `add` rather than new commands. See [docs/PRODUCT.md](docs/PRODUCT.md)
  for product boundaries and the planned next areas.
- Update README usage examples when command behavior or flags change.
- Commit messages use Conventional Commits (`feat:`, `fix:`, `refactor:`,
  `docs:`, `chore:`).
- `AGENTS.md` holds the repository working-style rules (small focused diffs, no
  broad refactors or renames unless required, avoid new dependencies, run
  targeted tests first, never commit or branch unless asked).
