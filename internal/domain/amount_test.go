package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestParsePricedAmountInputUsesDefaultCurrencyForAmount(t *testing.T) {
	parsed, err := ParsePricedAmountInput("3000 @@ 10 apl", "RUB")
	if err != nil {
		t.Fatalf("ParsePricedAmountInput() error = %v", err)
	}
	if !parsed.Amount.Value.Equal(decimal.NewFromInt(3000)) || parsed.Amount.Currency != "RUB" {
		t.Fatalf("unexpected amount: %+v", parsed.Amount)
	}
	if !parsed.Cost.Value.Equal(decimal.NewFromInt(10)) || parsed.Cost.Currency != "apl" {
		t.Fatalf("unexpected cost: %+v", parsed.Cost)
	}
}

func TestParsePricedAmountInputParsesUnitPrice(t *testing.T) {
	parsed, err := ParsePricedAmountInput("usd 90 @ 100", "RUB")
	if err != nil {
		t.Fatalf("ParsePricedAmountInput() error = %v", err)
	}
	if !parsed.Unit {
		t.Fatal("expected unit price operator")
	}
	if !parsed.Amount.Value.Equal(decimal.NewFromInt(90)) || parsed.Amount.Currency != "usd" {
		t.Fatalf("unexpected amount: %+v", parsed.Amount)
	}
	if !parsed.Cost.Value.Equal(decimal.NewFromInt(100)) || parsed.Cost.Currency != "RUB" {
		t.Fatalf("unexpected cost: %+v", parsed.Cost)
	}
	if !parsed.TotalCostValue().Equal(decimal.NewFromInt(9000)) {
		t.Fatalf("unexpected total cost: %d", parsed.TotalCostValue())
	}
}

func TestParsePricedAmountInputParsesNegativeUnitPrice(t *testing.T) {
	parsed, err := ParsePricedAmountInput("-20 usd @ 100", "RUB")
	if err != nil {
		t.Fatalf("ParsePricedAmountInput() error = %v", err)
	}
	if !parsed.TotalCostValue().Equal(decimal.NewFromInt(2000)) {
		t.Fatalf("unexpected total cost: %d", parsed.TotalCostValue())
	}
}

func TestParseAmountRejectsAmbiguousSeparators(t *testing.T) {
	dot := AmountFormat{DecimalMark: '.'}
	comma := AmountFormat{DecimalMark: ','}
	tests := []struct {
		name    string
		format  AmountFormat
		input   string
		want    string
		wantErr bool
	}{
		{name: "decimal comma under dot mark", format: dot, input: "1000,50 RUB", wantErr: true},
		{name: "short comma group under dot mark", format: dot, input: "1,5", wantErr: true},
		{name: "european amount under dot mark", format: dot, input: "1.000,00 EUR", wantErr: true},
		{name: "thousands group under dot mark", format: dot, input: "1,000.50 RUB", want: "1000.5"},
		{name: "plain decimal under dot mark", format: dot, input: "1000.50", want: "1000.5"},
		{name: "decimal comma under comma mark", format: comma, input: "1000,50 RUB", want: "1000.5"},
		{name: "thousands group under comma mark", format: comma, input: "1.000,50", want: "1000.5"},
		{name: "decimal dot under comma mark", format: comma, input: "1000.50", wantErr: true},
		{name: "spaces cannot group digits", format: dot, input: "1 000 RUB", wantErr: true},
		{name: "negative value", format: dot, input: "-1,000.50", want: "-1000.5"},
		{name: "leading mark", format: dot, input: ".5", want: "0.5"},
		{name: "not a number", format: dot, input: "10o", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.format.ParseAmount(tt.input, "RUB", false)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseAmount(%q) = %s, want error", tt.input, got.Value)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAmount(%q) error = %v", tt.input, err)
			}
			if got.Value.String() != tt.want {
				t.Fatalf("ParseAmount(%q) = %s, want %s", tt.input, got.Value, tt.want)
			}
		})
	}
}

func TestSampleAmountFollowsDecimalMark(t *testing.T) {
	if got := (AmountFormat{DecimalMark: '.'}).SampleAmount(); got != "1,000.00" {
		t.Fatalf("SampleAmount() = %q", got)
	}
	if got := (AmountFormat{DecimalMark: ','}).SampleAmount(); got != "1.000,00" {
		t.Fatalf("SampleAmount() = %q", got)
	}
}

func TestCanonicalFormatParsesRenderedAmounts(t *testing.T) {
	for _, input := range []string{"-3000 RUB", "97000 RUB", "1000.5 RUB", "0 RUB"} {
		if _, err := ParseAmountInput(input, "", false); err != nil {
			t.Fatalf("ParseAmountInput(%q) error = %v", input, err)
		}
	}
}
