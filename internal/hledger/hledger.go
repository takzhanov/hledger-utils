package hledger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

type Runner struct {
	Binary string
}

func NewRunner() Runner {
	return Runner{Binary: "hledger"}
}

func (r Runner) CheckStrict(journalFile string) error {
	_, stderr, err := r.run(filepath.Dir(journalFile), "check", "--strict", "-f", journalFile)
	if err != nil {
		if stderr != "" {
			return fmt.Errorf("%s", stderr)
		}
		return err
	}
	return nil
}

func (r Runner) StrictCheck(journalFile string) error {
	_, stderr, err := r.run(filepath.Dir(journalFile), "bs", "-tSV", "--strict", "-f", journalFile)
	if err != nil {
		if stderr != "" {
			return fmt.Errorf("%s", stderr)
		}
		return err
	}
	return nil
}

func (r Runner) BalanceByCommodity(journalFile, account string) (map[string]decimal.Decimal, error) {
	stdout, stderr, err := r.run(filepath.Dir(journalFile), "balance", ExactAccountQuery(account), "--flat", "--no-total", "-O", "json", "-f", journalFile)
	if err != nil {
		if stderr != "" {
			return nil, fmt.Errorf("%s", stderr)
		}
		return nil, err
	}
	return ParseBalanceByCommodityJSON(stdout)
}

func (r Runner) Accounts(journalFile string) ([]string, error) {
	stdout, stderr, err := r.run(filepath.Dir(journalFile), "accounts", "-f", journalFile)
	if err != nil {
		if stderr != "" {
			return nil, fmt.Errorf("%s", stderr)
		}
		return nil, err
	}
	if strings.TrimSpace(stdout) == "" {
		return nil, nil
	}
	return strings.Split(strings.TrimSpace(stdout), "\n"), nil
}

func (r Runner) AccountBalances(journalFile string) (map[string]string, error) {
	stdout, stderr, err := r.run(filepath.Dir(journalFile), "balance", "--flat", "--no-total", "-f", journalFile)
	if err != nil {
		if stderr != "" {
			return nil, fmt.Errorf("%s", stderr)
		}
		return nil, err
	}
	return ParseAccountBalances(stdout)
}

func (r Runner) DeclaredCommodities(journalFile string) (map[string]struct{}, error) {
	result := map[string]struct{}{}
	err := walkIncludedJournals(journalFile, func(path string) error {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "commodity ") {
				continue
			}
			fields := strings.Fields(strings.TrimPrefix(line, "commodity "))
			for _, field := range fields {
				commodity := extractCommodityToken(field)
				if commodity != "" {
					result[commodity] = struct{}{}
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// JournalStyle is what fin needs in order to read and write amounts the way the
// journal already does.
type JournalStyle struct {
	DefaultCommodity string
	DecimalMark      rune
}

func (r Runner) JournalStyle(journalFile string) (JournalStyle, error) {
	style := JournalStyle{}
	var explicitMark, defaultMark, commodityMark rune
	err := walkIncludedJournals(journalFile, func(path string) error {
		content, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for _, raw := range strings.Split(string(content), "\n") {
			line := stripJournalComment(strings.TrimSpace(raw))
			switch {
			case strings.HasPrefix(line, "decimal-mark "):
				token := strings.TrimSpace(strings.TrimPrefix(line, "decimal-mark "))
				if token == "." || token == "," {
					explicitMark = rune(token[0])
				}
			case strings.HasPrefix(line, "D "):
				sample := strings.TrimSpace(strings.TrimPrefix(line, "D"))
				if commodity := extractCommodityToken(sample); commodity != "" {
					style.DefaultCommodity = commodity
				}
				if mark := decimalMarkFromSample(sample); mark != 0 {
					defaultMark = mark
				}
			case strings.HasPrefix(line, "commodity "):
				if commodityMark == 0 {
					commodityMark = decimalMarkFromSample(strings.TrimPrefix(line, "commodity "))
				}
			}
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return JournalStyle{}, nil
		}
		return JournalStyle{}, err
	}
	switch {
	case explicitMark != 0:
		style.DecimalMark = explicitMark
	case defaultMark != 0:
		style.DecimalMark = defaultMark
	default:
		style.DecimalMark = commodityMark
	}
	return style, nil
}

// decimalMarkFromSample reads the punctuation of a sample amount such as
// "1,000.00", "1.000,00" or "$1,000.00". With both separators present the last
// one is the decimal mark; a lone separator followed by exactly three digits
// groups thousands.
func decimalMarkFromSample(text string) rune {
	lastDot := strings.LastIndex(text, ".")
	lastComma := strings.LastIndex(text, ",")
	switch {
	case lastDot >= 0 && lastComma >= 0:
		if lastDot > lastComma {
			return '.'
		}
		return ','
	case lastDot >= 0:
		return markForSingleSeparator(text, lastDot, '.', ',')
	case lastComma >= 0:
		return markForSingleSeparator(text, lastComma, ',', '.')
	}
	return 0
}

func markForSingleSeparator(text string, index int, separator, other rune) rune {
	if strings.Count(text, string(separator)) > 1 {
		return other
	}
	if leadingDigits(text[index+1:]) == 3 {
		return other
	}
	return separator
}

func leadingDigits(text string) int {
	digits := 0
	for _, r := range text {
		if r < '0' || r > '9' {
			break
		}
		digits++
	}
	return digits
}

func walkIncludedJournals(mainFile string, visit func(path string) error) error {
	seen := map[string]struct{}{}
	var walk func(path string) error
	walk = func(path string) error {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		absolute = filepath.Clean(absolute)
		if _, ok := seen[absolute]; ok {
			return nil
		}
		seen[absolute] = struct{}{}

		content, err := os.ReadFile(absolute)
		if err != nil {
			return err
		}
		if err := visit(absolute); err != nil {
			return err
		}
		for _, raw := range strings.Split(string(content), "\n") {
			line := stripJournalComment(strings.TrimSpace(raw))
			if !strings.HasPrefix(line, "include ") {
				continue
			}
			pattern := strings.TrimSpace(strings.TrimPrefix(line, "include "))
			if pattern == "" {
				continue
			}
			if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(filepath.Dir(absolute), pattern)
			}
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return fmt.Errorf("invalid include pattern %q: %w", pattern, err)
			}
			if len(matches) == 0 {
				return fmt.Errorf("include %q matched no files", pattern)
			}
			for _, match := range matches {
				if err := walk(match); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(mainFile)
}

func (r Runner) run(workdir string, args ...string) (string, string, error) {
	cmd := exec.Command(r.Binary, args...)
	cmd.Dir = workdir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

func ParseBalanceByCommodityJSON(output string) (map[string]decimal.Decimal, error) {
	type quantity struct {
		DecimalMantissa json.Number `json:"decimalMantissa"`
		DecimalPlaces   int32       `json:"decimalPlaces"`
	}
	type amount struct {
		Commodity string   `json:"acommodity"`
		Quantity  quantity `json:"aquantity"`
	}

	var payload []json.RawMessage
	decoder := json.NewDecoder(strings.NewReader(output))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload) < 2 {
		return nil, fmt.Errorf("unexpected balance json payload")
	}

	var totals []amount
	if err := json.Unmarshal(payload[1], &totals); err != nil {
		return nil, err
	}

	result := map[string]decimal.Decimal{}
	for _, item := range totals {
		mantissa, err := item.Quantity.DecimalMantissa.Int64()
		if err != nil {
			return nil, fmt.Errorf("parse decimal mantissa for %q: %w", item.Commodity, err)
		}
		result[item.Commodity] = result[item.Commodity].Add(decimal.New(mantissa, -item.Quantity.DecimalPlaces))
	}
	return result, nil
}

func ParseAccountBalances(output string) (map[string]string, error) {
	result := map[string]string{}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	pending := make([]string, 0)
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		last := fields[len(fields)-1]
		if strings.Contains(last, ":") {
			account := last
			parts := append([]string{}, pending...)
			parts = append(parts, strings.Join(fields[:len(fields)-1], " "))
			result[account] = strings.Join(parts, ", ")
			pending = pending[:0]
			continue
		}
		pending = append(pending, line)
	}
	return result, nil
}

func ExactAccountQuery(account string) string {
	return "acct:^" + regexp.QuoteMeta(account) + "$"
}

func extractCommodityToken(text string) string {
	var builder strings.Builder
	for _, r := range text {
		if (r >= '0' && r <= '9') || r == ',' || r == '.' || r == '-' {
			continue
		}
		builder.WriteRune(r)
	}
	return strings.TrimSpace(builder.String())
}

func stripJournalComment(line string) string {
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
		return ""
	}
	commentIdx := len(line)
	if idx := strings.Index(line, "#"); idx >= 0 && idx < commentIdx {
		commentIdx = idx
	}
	if idx := strings.Index(line, ";"); idx >= 0 && idx < commentIdx {
		commentIdx = idx
	}
	if commentIdx < len(line) {
		line = line[:commentIdx]
	}
	return strings.TrimSpace(line)
}
