package ui

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Prompter struct {
	in             *bufio.Reader
	out            io.Writer
	accountDisplay AccountDisplayOptions
}

type AccountOption struct {
	Account string
	Balance string
}

type AccountDisplayOptions struct {
	ShowBalances bool
}

var ErrSelectionCancelled = errors.New("selection cancelled")
var ErrInterrupted = errors.New("interrupted")

var (
	dayOnlyPattern  = regexp.MustCompile(`^\d{1,2}$`)
	monthDayPattern = regexp.MustCompile(`^\d{2}-\d{2}$`)
)

func NewPrompter(in io.Reader, out io.Writer) Prompter {
	return Prompter{
		in:  bufio.NewReader(in),
		out: out,
		accountDisplay: AccountDisplayOptions{
			ShowBalances: true,
		},
	}
}

func (p Prompter) WithAccountDisplay(options AccountDisplayOptions) Prompter {
	p.accountDisplay = options
	return p
}

func (p Prompter) AskString(label string) (string, error) {
	fmt.Fprintf(p.out, "%s: ", label)
	text, err := p.in.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func (p Prompter) AskStringDefault(label, defaultValue string) (string, error) {
	fmt.Fprintf(p.out, "%s [%s]: ", label, defaultValue)
	text, err := p.in.ReadString('\n')
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return defaultValue, nil
	}
	return text, nil
}

func (p Prompter) AskRequiredString(label string) (string, error) {
	for {
		value, err := p.AskString(label)
		if err != nil {
			return "", err
		}
		if value != "" {
			return value, nil
		}
		fmt.Fprintf(p.out, "%s is required\n", label)
	}
}

func (p Prompter) AskDate(label string, defaultDate time.Time) (time.Time, error) {
	for {
		fmt.Fprintf(p.out, "%s [%s]: ", label, defaultDate.Format("2006-01-02"))
		text, err := p.in.ReadString('\n')
		if err != nil {
			return time.Time{}, err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return defaultDate, nil
		}
		date, inferred, err := ParseDateInput(text, defaultDate)
		if err != nil {
			fmt.Fprintf(p.out, "%s\n", err)
			continue
		}
		if inferred {
			fmt.Fprintf(p.out, "Resolved date: %s\n", date.Format("2006-01-02"))
		}
		return date, nil
	}
}

func ParseDateInput(text string, referenceDate time.Time) (time.Time, bool, error) {
	text = strings.TrimSpace(text)
	location := referenceDate.Location()

	if dayOnlyPattern.MatchString(text) {
		day, err := strconv.Atoi(text)
		if err != nil {
			return time.Time{}, false, invalidDateFormatError()
		}
		date, ok := validDate(referenceDate.Year(), referenceDate.Month(), day, location)
		if !ok {
			return time.Time{}, false, invalidDateFormatError()
		}
		return date, true, nil
	}

	if monthDayPattern.MatchString(text) {
		month, _ := strconv.Atoi(text[:2])
		day, _ := strconv.Atoi(text[3:])
		date, ok := validDate(referenceDate.Year(), time.Month(month), day, location)
		if !ok {
			return time.Time{}, false, invalidDateFormatError()
		}
		return date, true, nil
	}

	date, err := time.ParseInLocation("2006-01-02", text, location)
	if err != nil {
		return time.Time{}, false, invalidDateFormatError()
	}
	return date, false, nil
}

func validDate(year int, month time.Month, day int, location *time.Location) (time.Time, bool) {
	date := time.Date(year, month, day, 0, 0, 0, 0, location)
	return date, date.Year() == year && date.Month() == month && date.Day() == day
}

func invalidDateFormatError() error {
	return fmt.Errorf("date must use DD, MM-DD, or YYYY-MM-DD")
}

func (p Prompter) AskYesNo(label string, defaultYes bool) (bool, error) {
	suffix := "[y/N]"
	if defaultYes {
		suffix = "[Y/n]"
	}
	fmt.Fprintf(p.out, "%s %s: ", label, suffix)
	text, err := p.in.ReadString('\n')
	if err != nil {
		return false, err
	}
	text = strings.TrimSpace(strings.ToLower(text))
	if text == "" {
		return defaultYes, nil
	}
	return text == "y" || text == "yes", nil
}

func (p Prompter) AskAccount(label string, options []AccountOption) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no account options available for %s", strings.ToLower(label))
	}
	if !canUseInteractiveFZF() {
		return "", fmt.Errorf("account selection requires an interactive terminal")
	}
	for {
		selected, err := p.askAccountWithFZF(label, options)
		if errors.Is(err, ErrSelectionCancelled) {
			continue
		}
		return selected, err
	}
}

func (p Prompter) askAccountWithFZF(label string, options []AccountOption) (string, error) {
	fzfPath, err := exec.LookPath("fzf")
	if err != nil {
		return "", err
	}

	lines := make([]string, 0, len(options))
	for _, option := range options {
		lines = append(lines, fmt.Sprintf("%s\t%s", styleSelected(option.Account), option.Account))
	}

	args := []string{
		"--ansi",
		"--delimiter=\t",
		"--with-nth=1",
		"--prompt=" + label + "> ",
		"--header=Type to search. Enter selects. Esc cancels.",
		"--height=40%",
		"--layout=reverse",
		"--expect=ctrl-d,ctrl-c",
	}
	cmd := exec.Command(fzfPath, args...)
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			switch exitErr.ExitCode() {
			case 1:
				return "", ErrSelectionCancelled
			case 130:
				return "", ErrInterrupted
			}
		}
		return "", err
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return "", ErrSelectionCancelled
	}
	linesOut := strings.Split(output, "\n")
	if len(linesOut) == 0 {
		return "", ErrSelectionCancelled
	}
	switch strings.TrimSpace(linesOut[0]) {
	case "ctrl-d":
		return "", io.EOF
	case "ctrl-c":
		return "", ErrInterrupted
	}
	selectedLine := linesOut[len(linesOut)-1]
	if strings.TrimSpace(selectedLine) == "" {
		return "", ErrSelectionCancelled
	}
	parts := strings.Split(selectedLine, "\t")
	account := strings.TrimSpace(parts[len(parts)-1])
	for _, option := range options {
		if option.Account == account {
			printSingleAccountOption(p.out, "Selected", option, p.accountDisplay)
			return account, nil
		}
	}
	return "", fmt.Errorf("selected account %q is unknown", account)
}

func printSingleAccountOption(out io.Writer, prefix string, option AccountOption, display AccountDisplayOptions) {
	fmt.Fprintf(out, "%s: %s", prefix, styleSelected(option.Account))
	printAccountMeta(out, option, display)
	fmt.Fprintln(out)
}

func printAccountMeta(out io.Writer, option AccountOption, display AccountDisplayOptions) {
	meta := make([]string, 0, 1)
	if display.ShowBalances && option.Balance != "" {
		meta = append(meta, "balance: "+option.Balance)
	}
	if len(meta) > 0 {
		fmt.Fprintf(out, " (%s)", strings.Join(meta, ", "))
	}
}

func EnsureFZF(out io.Writer) error {
	if _, err := exec.LookPath("fzf"); err == nil {
		return nil
	}
	brew, err := exec.LookPath("brew")
	if err != nil {
		return fmt.Errorf("fzf is required; install it and retry")
	}
	fmt.Fprintln(out, "fzf not found; installing with Homebrew...")
	cmd := exec.Command(brew, "install", "fzf")
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install fzf: %w", err)
	}
	if _, err := exec.LookPath("fzf"); err != nil {
		return fmt.Errorf("fzf was installed but is not available in PATH")
	}
	return nil
}

func styleSelected(text string) string {
	return "\033[1;36m" + text + "\033[0m"
}

func canUseInteractiveFZF() bool {
	stdinInfo, err := os.Stdin.Stat()
	if err != nil || stdinInfo.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	stdoutInfo, err := os.Stdout.Stat()
	if err != nil || stdoutInfo.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	return true
}
