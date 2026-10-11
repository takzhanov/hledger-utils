package hledger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shopspring/decimal"
)

func TestExactAccountQuery(t *testing.T) {
	got := ExactAccountQuery("assets:card")
	want := "acct:^assets:card$"
	if got != want {
		t.Fatalf("ExactAccountQuery() = %q, want %q", got, want)
	}
}

func TestParseBalanceByCommodityJSONUsesExactQuantity(t *testing.T) {
	output := `[
  [
    [
      "assets:bog:savings",
      "assets:bog:savings",
      0,
      [
        {
          "acommodity": "usd",
          "acost": null,
          "aquantity": {
            "decimalMantissa": 5345462,
            "decimalPlaces": 2,
            "floatingPoint": 53454.62
          },
          "astyle": {
            "asprecision": 0,
            "asrounding": "HardRounding"
          }
        }
      ]
    ]
  ],
  [
    {
      "acommodity": "usd",
      "acost": null,
      "aquantity": {
        "decimalMantissa": 5345462,
        "decimalPlaces": 2,
        "floatingPoint": 53454.62
      },
      "astyle": {
        "asprecision": 0,
        "asrounding": "HardRounding"
      }
    }
  ]
]`

	got, err := ParseBalanceByCommodityJSON(output)
	if err != nil {
		t.Fatalf("ParseBalanceByCommodityJSON() error = %v", err)
	}
	if want := decimal.RequireFromString("53454.62"); !got["usd"].Equal(want) {
		t.Fatalf("got usd %s, want %s", got["usd"], want)
	}
}

func TestParseAccountBalancesStackedOutput(t *testing.T) {
	got, err := ParseAccountBalances("20.00 RUB\n20.00 apl\n33.00 usdt  assets:cash\n92,180.00 RUB\n-20.00 usd  assets:card")
	if err != nil {
		t.Fatalf("ParseAccountBalances() error = %v", err)
	}
	if got["assets:cash"] != "20.00 RUB, 20.00 apl, 33.00 usdt" {
		t.Fatalf("unexpected assets:cash balance: %q", got["assets:cash"])
	}
	if got["assets:card"] != "92,180.00 RUB, -20.00 usd" {
		t.Fatalf("unexpected assets:card balance: %q", got["assets:card"])
	}
}

func TestJournalStyleReadsDefaultCommodity(t *testing.T) {
	tempDir := t.TempDir()
	journalFile := filepath.Join(tempDir, "main.journal")
	includeFile := filepath.Join(tempDir, ".hledger.journal")
	content := "include .hledger.journal\n"
	if err := os.WriteFile(journalFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(includeFile, []byte("D $1,000.00\nD 1,000.00 RUB ; default commodity\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "unrelated.journal"), []byte("D 1,000.00 BAD\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := NewRunner().JournalStyle(journalFile)
	if err != nil {
		t.Fatalf("JournalStyle() error = %v", err)
	}
	if got.DefaultCommodity != "RUB" {
		t.Fatalf("DefaultCommodity = %q, want %q", got.DefaultCommodity, "RUB")
	}
	if got.DecimalMark != '.' {
		t.Fatalf("DecimalMark = %q, want %q", got.DecimalMark, '.')
	}
}

func TestJournalStyleReadsDecimalMark(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    rune
	}{
		{name: "directive wins", content: "decimal-mark ,\ncommodity 1,000.00 RUB\n", want: ','},
		{name: "european default commodity", content: "D 1.000,00 EUR\n", want: ','},
		{name: "commodity sample", content: "commodity 1.000,00 EUR\n", want: ','},
		{name: "anglo commodity sample", content: "commodity 1,000.00 RUB\n", want: '.'},
		{name: "decimal only sample", content: "commodity 1.00 RUB\n", want: '.'},
		{name: "no punctuation", content: "commodity RUB\n", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			journalFile := filepath.Join(t.TempDir(), "main.journal")
			if err := os.WriteFile(journalFile, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := NewRunner().JournalStyle(journalFile)
			if err != nil {
				t.Fatalf("JournalStyle() error = %v", err)
			}
			if got.DecimalMark != tt.want {
				t.Fatalf("DecimalMark = %q, want %q", got.DecimalMark, tt.want)
			}
		})
	}
}

func TestDeclaredCommoditiesOnlyReadsIncludedFiles(t *testing.T) {
	tempDir := t.TempDir()
	journalFile := filepath.Join(tempDir, "main.journal")
	if err := os.WriteFile(journalFile, []byte("include included.journal\ncommodity 1,000.00 RUB\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "included.journal"), []byte("commodity 1,000.00 USD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "unrelated.journal"), []byte("commodity 1,000.00 SECRET\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := NewRunner().DeclaredCommodities(journalFile)
	if err != nil {
		t.Fatalf("DeclaredCommodities() error = %v", err)
	}
	if _, ok := got["RUB"]; !ok {
		t.Fatalf("declared commodities = %v, missing RUB", got)
	}
	if _, ok := got["USD"]; !ok {
		t.Fatalf("declared commodities = %v, missing USD", got)
	}
	if _, ok := got["SECRET"]; ok {
		t.Fatalf("declared commodities = %v, included unrelated file", got)
	}
}
