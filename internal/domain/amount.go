package domain

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

type AmountInput struct {
	Value    decimal.Decimal
	Currency string
}

type PricedAmountInput struct {
	Amount AmountInput
	Cost   AmountInput
	Unit   bool
}

// AmountFormat describes how a typed amount is punctuated. The decimal mark
// comes from the journal; the other of "." and "," may then only group
// thousands, and anything else is rejected instead of being reinterpreted.
// Amounts fin renders itself always use CanonicalFormat.
type AmountFormat struct {
	DecimalMark rune
}

var CanonicalFormat = AmountFormat{DecimalMark: '.'}

var numberPatterns = map[rune]*regexp.Regexp{
	'.': numberPattern('.', ','),
	',': numberPattern(',', '.'),
}

func numberPattern(mark, group rune) *regexp.Regexp {
	m := regexp.QuoteMeta(string(mark))
	g := regexp.QuoteMeta(string(group))
	return regexp.MustCompile(`^[-+]?(?:(?:\d+|\d{1,3}(?:` + g + `\d{3})+)(?:` + m + `\d+)?|` + m + `\d+)$`)
}

func (f AmountFormat) mark() rune {
	if f.DecimalMark == ',' {
		return ','
	}
	return '.'
}

func (f AmountFormat) group() rune {
	if f.mark() == ',' {
		return '.'
	}
	return ','
}

// SampleAmount is the amount written into a generated commodity directive; it
// declares the journal's own punctuation.
func (f AmountFormat) SampleAmount() string {
	if f.mark() == ',' {
		return "1.000,00"
	}
	return "1,000.00"
}

func ParseAmountInput(text, defaultCurrency string, requireCurrency bool) (AmountInput, error) {
	return CanonicalFormat.ParseAmount(text, defaultCurrency, requireCurrency)
}

func ParsePricedAmountInput(text, defaultCurrency string) (PricedAmountInput, error) {
	return CanonicalFormat.ParsePricedAmount(text, defaultCurrency)
}

func (f AmountFormat) ParseAmount(text, defaultCurrency string, requireCurrency bool) (AmountInput, error) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return AmountInput{}, fmt.Errorf("amount is required")
	}
	if len(fields) > 2 {
		return AmountInput{}, expressionError(text, fields)
	}

	var (
		value    decimal.Decimal
		currency = defaultCurrency
		err      error
	)
	if len(fields) == 1 {
		value, err = f.parseNumber(fields[0])
		if err != nil {
			return AmountInput{}, err
		}
	} else {
		switch {
		case looksNumeric(fields[0]) && !looksNumeric(fields[1]):
			value, err = f.parseNumber(fields[0])
			currency = fields[1]
		case !looksNumeric(fields[0]) && looksNumeric(fields[1]):
			value, err = f.parseNumber(fields[1])
			currency = fields[0]
		default:
			return AmountInput{}, fmt.Errorf("invalid amount expression %q", text)
		}
		if err != nil {
			return AmountInput{}, err
		}
	}
	if requireCurrency && strings.TrimSpace(currency) == "" {
		return AmountInput{}, fmt.Errorf("currency is required in %q", text)
	}
	return AmountInput{Value: value, Currency: currency}, nil
}

func (f AmountFormat) ParsePricedAmount(text, defaultCurrency string) (PricedAmountInput, error) {
	var (
		parts []string
		unit  bool
	)
	switch {
	case strings.Contains(text, "@@"):
		parts = strings.Split(text, "@@")
	case strings.Contains(text, "@"):
		parts = strings.Split(text, "@")
		unit = true
	default:
		return PricedAmountInput{}, fmt.Errorf("priced amount must use '<amount> @@ <cost>' or '<amount> @ <price>'")
	}
	if len(parts) != 2 {
		return PricedAmountInput{}, fmt.Errorf("invalid priced amount expression %q", text)
	}
	amount, err := f.ParseAmount(strings.TrimSpace(parts[0]), defaultCurrency, false)
	if err != nil {
		return PricedAmountInput{}, err
	}
	if strings.TrimSpace(amount.Currency) == "" {
		return PricedAmountInput{}, fmt.Errorf("currency is required in %q", strings.TrimSpace(parts[0]))
	}
	cost, err := f.ParseAmount(strings.TrimSpace(parts[1]), defaultCurrency, false)
	if err != nil {
		return PricedAmountInput{}, err
	}
	return PricedAmountInput{Amount: amount, Cost: cost, Unit: unit}, nil
}

func (p PricedAmountInput) TotalCostValue() decimal.Decimal {
	if p.Unit {
		amount := p.Amount.Value
		if amount.IsNegative() {
			amount = amount.Neg()
		}
		return amount.Mul(p.Cost.Value)
	}
	return p.Cost.Value
}

// RenderAmount writes a value the way the journal punctuates amounts. fin keeps
// amounts canonical in memory and converts only where text reaches the journal
// or the user, because hledger reads "10.5" in a comma-mark journal as 105.
func (f AmountFormat) RenderAmount(value decimal.Decimal, currency string) string {
	text := value.String()
	if f.mark() != '.' {
		text = strings.ReplaceAll(text, ".", string(f.mark()))
	}
	if currency == "" {
		return text
	}
	return text + " " + currency
}

// RenderAmountText converts a canonical amount, priced amounts included, into
// the journal's punctuation.
func (f AmountFormat) RenderAmountText(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	if strings.Contains(text, "@") {
		priced, err := ParsePricedAmountInput(text, "")
		if err != nil {
			return "", err
		}
		operator := "@@"
		if priced.Unit {
			operator = "@"
		}
		return fmt.Sprintf("%s %s %s", f.RenderAmount(priced.Amount.Value, priced.Amount.Currency), operator, f.RenderAmount(priced.Cost.Value, priced.Cost.Currency)), nil
	}
	amount, err := ParseAmountInput(text, "", false)
	if err != nil {
		return "", err
	}
	return f.RenderAmount(amount.Value, amount.Currency), nil
}

func (f AmountFormat) parseNumber(text string) (decimal.Decimal, error) {
	if !numberPatterns[f.mark()].MatchString(text) {
		return decimal.Decimal{}, f.numberError(text)
	}
	normalized := strings.ReplaceAll(text, string(f.group()), "")
	if f.mark() != '.' {
		normalized = strings.ReplaceAll(normalized, string(f.mark()), ".")
	}
	value, err := decimal.NewFromString(normalized)
	if err != nil {
		return decimal.Decimal{}, f.numberError(text)
	}
	return value, nil
}

func (f AmountFormat) numberError(text string) error {
	mark, group := string(f.mark()), string(f.group())
	if strings.ContainsRune(text, f.mark()) || strings.ContainsRune(text, f.group()) {
		return fmt.Errorf("invalid amount %q: this journal writes the decimal mark as %q, so %q may only group thousands (1%s000%s50)", text, mark, group, group, mark)
	}
	return fmt.Errorf("invalid amount %q", text)
}

func expressionError(text string, fields []string) error {
	numeric := 0
	for _, field := range fields {
		if looksNumeric(field) {
			numeric++
		}
	}
	if numeric > 1 {
		return fmt.Errorf("invalid amount expression %q: spaces cannot group digits", text)
	}
	return fmt.Errorf("invalid amount expression %q", text)
}

func looksNumeric(text string) bool {
	digits := false
	for _, r := range text {
		switch {
		case r >= '0' && r <= '9':
			digits = true
		case r == '-' || r == '+' || r == '.' || r == ',':
		default:
			return false
		}
	}
	return digits
}
