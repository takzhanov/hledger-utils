package app

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/takzhanov/hledger-utils/internal/addflow"
	"github.com/takzhanov/hledger-utils/internal/domain"
	"github.com/takzhanov/hledger-utils/internal/hledger"
	"github.com/takzhanov/hledger-utils/internal/journal"
	"github.com/takzhanov/hledger-utils/internal/ui"
)

func Run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	journalFile, rest, err := parseRootArgs(args)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		return usageError()
	}
	if rest[0] == "help" {
		printRootUsage(os.Stdout)
		return nil
	}

	runner := hledger.NewRunner()
	style, err := runner.JournalStyle(journalFile)
	if err != nil {
		return err
	}

	switch rest[0] {
	case "check":
		if len(rest) != 1 {
			return fmt.Errorf("usage: fin [--journal path] check")
		}
		return runner.CheckStrict(journalFile)
	case "add":
		return runAdd(journalFile, runner, style, rest[1:])
	default:
		return usageError()
	}
}

func runAdd(journalFile string, runner hledger.Runner, style hledger.JournalStyle, args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	dateFlag := fs.String("date", "", "transaction date (DD, MM-DD, or YYYY-MM-DD)")
	descriptionFlag := fs.String("description", "", "transaction description")
	dryRun := fs.Bool("dry-run", false, "print transaction without appending")
	showBalances := fs.Bool("show-balances", true, "show account balances in selectors")
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: fin [--journal PATH] add [options] [account]")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Interactively creates, validates, and appends one transaction.")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Options:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("usage: fin add [--date DATE] [--dry-run] [account]")
	}
	if err := ui.EnsureFZF(os.Stdout); err != nil {
		return err
	}

	prompter := ui.NewPrompter(os.Stdin, os.Stdout).WithAccountDisplay(ui.AccountDisplayOptions{
		ShowBalances: *showBalances,
	})
	prompts := addPrompts{
		prompter:        prompter,
		runner:          runner,
		dateFlag:        *dateFlag,
		descriptionFlag: *descriptionFlag,
		argAccount: func() string {
			if fs.NArg() == 1 {
				return fs.Arg(0)
			}
			return ""
		}(),
	}
	fmt.Printf("Journal: %s\n", journalFile)
	format := domain.AmountFormat{DecimalMark: style.DecimalMark}
	result, err := addflow.Run(addflow.NewState(), prompts, addEffects{runner: runner, journalFile: journalFile}, addflow.Settings{
		DefaultCurrency: style.DefaultCommodity,
		Format:          format,
	})
	if err != nil {
		return err
	}
	return finalizeTransaction(runner, journalFile, result.Transaction(), result.State.NewCommodities, format, *dryRun)
}

func finalizeTransaction(runner hledger.Runner, targetFile string, tx domain.Transaction, newCommodities []string, format domain.AmountFormat, dryRun bool) error {
	entry, err := journal.RenderTransaction(tx, format)
	if err != nil {
		return err
	}
	directives := make([]string, 0, len(newCommodities))
	for _, commodity := range newCommodities {
		directives = append(directives, journal.CommodityDirective(commodity, format))
	}
	fmt.Println("Generated journal:")
	fmt.Println()
	for _, directive := range directives {
		fmt.Println(directive)
	}
	if len(directives) > 0 {
		fmt.Println()
	}
	fmt.Print(entry)

	if dryRun {
		return nil
	}
	candidate, err := candidateAppend(targetFile, directives, entry)
	if err != nil {
		return err
	}
	if err := validateCandidateContent(runner, targetFile, candidate); err != nil {
		return err
	}
	return journal.Replace(targetFile, candidate)
}

func candidateAppend(targetFile string, directives []string, entry string) ([]byte, error) {
	original, err := os.ReadFile(targetFile)
	if err != nil {
		return nil, fmt.Errorf("read target file: %w", err)
	}
	return journal.AppendDirectivesAndEntry(original, directives, entry), nil
}

func validateCandidateContent(runner hledger.Runner, targetFile string, content []byte) error {
	tempFile, err := os.CreateTemp(filepath.Dir(targetFile), ".fin-hledger-candidate-*")
	if err != nil {
		return fmt.Errorf("create candidate journal: %w", err)
	}
	tempTarget := tempFile.Name()
	defer os.Remove(tempTarget)
	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		return fmt.Errorf("write candidate journal: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close candidate journal: %w", err)
	}

	if err := runner.StrictCheck(tempTarget); err != nil {
		return fmt.Errorf("journal validation failed: %w", err)
	}
	return nil
}

func accountOptions(runner hledger.Runner, journalFile string) ([]ui.AccountOption, error) {
	known, err := runner.Accounts(journalFile)
	if err != nil {
		return nil, err
	}
	balances, err := runner.AccountBalances(journalFile)
	if err != nil {
		return nil, err
	}
	options := make([]ui.AccountOption, 0, len(known))
	for _, account := range known {
		options = append(options, ui.AccountOption{
			Account: account,
			Balance: balances[account],
		})
	}
	return options, nil
}

type addEffects struct {
	runner      hledger.Runner
	journalFile string
}

func (g addEffects) ResolveEffect(request addflow.EffectRequest) (any, error) {
	switch value := request.(type) {
	case addflow.NeedDeclaredCommodities:
		return g.runner.DeclaredCommodities(g.journalFile)
	case addflow.NeedAccountOptions:
		options, err := accountOptions(g.runner, g.journalFile)
		if err != nil {
			return nil, err
		}
		result := make([]addflow.AccountOption, 0, len(options))
		for _, option := range options {
			result = append(result, addflow.AccountOption{
				Account: option.Account,
				Balance: option.Balance,
			})
		}
		return result, nil
	case addflow.NeedBalanceByCommodity:
		return g.runner.BalanceByCommodity(g.journalFile, value.Account)
	default:
		return nil, fmt.Errorf("unsupported add effect %T", request)
	}
}

type addPrompts struct {
	prompter        ui.Prompter
	runner          hledger.Runner
	dateFlag        string
	descriptionFlag string
	argAccount      string
}

// announce explains a repeated prompt before asking again.
func (p addPrompts) announce(spec addflow.PromptSpec) {
	if notice := strings.TrimSpace(spec.Notice); notice != "" {
		fmt.Printf("! %s\n", notice)
	}
}

func (p addPrompts) PromptDate(spec addflow.PromptSpec) (time.Time, error) {
	p.announce(spec)
	return parseOrPromptDate(p.prompter, p.dateFlag)
}

func (p addPrompts) PromptAccount(spec addflow.PromptSpec) (string, error) {
	p.announce(spec)
	if spec.Remaining != "" {
		fmt.Printf("Remaining: %s\n", spec.Remaining)
	}
	options := make([]ui.AccountOption, 0, len(spec.AccountOptions))
	for _, option := range spec.AccountOptions {
		options = append(options, ui.AccountOption{
			Account: option.Account,
			Balance: option.Balance,
		})
	}
	if spec.Kind == addflow.PromptPrimaryAccount && p.argAccount != "" {
		return selectProvidedAccount(p.argAccount, options)
	}
	input, err := p.prompter.AskAccount(spec.Label, options)
	if err != nil {
		return "", err
	}
	return selectProvidedAccount(input, options)
}

func (p addPrompts) PromptString(spec addflow.PromptSpec) (string, error) {
	p.announce(spec)
	if spec.Kind == addflow.PromptDescription && strings.TrimSpace(p.descriptionFlag) != "" {
		return strings.TrimSpace(p.descriptionFlag), nil
	}
	if spec.EffectiveBalance != "" {
		fmt.Printf("Current effective balance: %s\n", spec.EffectiveBalance)
	}
	if spec.Default == "" {
		return p.prompter.AskRequiredString(spec.Label)
	}
	return p.prompter.AskStringDefault(spec.Label, spec.Default)
}

func (p addPrompts) PromptYesNo(spec addflow.PromptSpec) (bool, error) {
	p.announce(spec)
	return p.prompter.AskYesNo(spec.Label, false)
}

func parseOrPromptDate(prompter ui.Prompter, dateFlag string) (time.Time, error) {
	if strings.TrimSpace(dateFlag) != "" {
		date, _, err := ui.ParseDateInput(dateFlag, time.Now())
		return date, err
	}
	return prompter.AskDate("Date", time.Now())
}

func selectProvidedAccount(input string, options []ui.AccountOption) (string, error) {
	resolved := strings.TrimSpace(input)
	for _, option := range options {
		if option.Account == resolved {
			return resolved, nil
		}
	}
	matches := make([]string, 0)
	for _, option := range options {
		if strings.Contains(strings.ToLower(option.Account), strings.ToLower(resolved)) {
			matches = append(matches, option.Account)
		}
	}
	if len(matches) > 0 {
		return "", fmt.Errorf("account %q was not found exactly; close matches: %s", resolved, strings.Join(matches, ", "))
	}
	return "", fmt.Errorf("account %q was not found in journal accounts", resolved)
}

func parseRootArgs(args []string) (string, []string, error) {
	fs := flag.NewFlagSet("fin", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() { printRootUsage(fs.Output()) }
	var journalFlag string
	fs.StringVar(&journalFlag, "journal", "", "main journal file")
	fs.StringVar(&journalFlag, "f", "", "main journal file")
	if err := fs.Parse(args); err != nil {
		return "", nil, err
	}
	journalFile, err := resolveJournalFile(journalFlag)
	if err != nil {
		return "", nil, err
	}
	return journalFile, fs.Args(), nil
}

func resolveJournalFile(explicit string) (string, error) {
	if explicit != "" {
		return existingAbsolutePath(expandHome(explicit))
	}
	if envPath := os.Getenv("LEDGER_FILE"); envPath != "" {
		return existingAbsolutePath(expandHome(envPath))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("journal file not found; use --journal PATH or LEDGER_FILE: %w", err)
	}
	path, err := existingAbsolutePath(filepath.Join(home, ".hledger.journal"))
	if err != nil {
		return "", fmt.Errorf("journal file not found; use --journal PATH or LEDGER_FILE: %w", err)
	}
	return path, nil
}

// expandHome mirrors hledger, which expands a leading ~ in -f and LEDGER_FILE.
func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

func existingAbsolutePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve journal path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("open journal %q: %w", abs, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("journal path %q is a directory", abs)
	}
	return abs, nil
}

func usageError() error {
	return fmt.Errorf("usage: fin [--journal path] <add|check> ...")
}

func printRootUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: fin [--journal PATH] <command> [options]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Smart interactive transaction entry for hledger journals.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Commands:")
	fmt.Fprintln(out, "  add      Create, validate, and append a transaction")
	fmt.Fprintln(out, "  check    Run hledger check --strict")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Journal selection (same as hledger): --journal/-f, LEDGER_FILE, ~/.hledger.journal.")
}
