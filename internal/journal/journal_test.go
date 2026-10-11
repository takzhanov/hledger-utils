package journal

import (
	"strings"
	"testing"
	"time"

	"github.com/takzhanov/hledger-utils/internal/domain"
)

func TestRenderTransaction(t *testing.T) {
	entry, err := RenderTransaction(domain.Transaction{
		Date:        time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		Description: "reconcile assets:card",
		Postings: []domain.Posting{
			{Account: "expenses:food", Amount: "3000 RUB"},
			{Account: "assets:card", Amount: "-3000 RUB", BalanceAssertion: "97000 RUB"},
		},
	}, domain.AmountFormat{})
	if err != nil {
		t.Fatalf("RenderTransaction() error = %v", err)
	}
	if !strings.Contains(entry, "2026-03-15 reconcile assets:card") {
		t.Fatalf("entry missing header: %q", entry)
	}
	if !strings.HasSuffix(entry, "\n\n") {
		t.Fatalf("entry should end with blank line: %q", entry)
	}
	if !strings.Contains(entry, "assets:card      -3000 RUB = 97000 RUB") {
		t.Fatalf("entry missing balance assertion: %q", entry)
	}
}

func TestRenderZeroDeltaBalanceAssertion(t *testing.T) {
	entry, err := RenderTransaction(domain.Transaction{
		Date:        time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC),
		Description: "by_fin",
		Postings: []domain.Posting{
			{Account: "assets:ozon", Amount: "0 RUB", BalanceAssertion: "0 RUB"},
		},
	}, domain.AmountFormat{})
	if err != nil {
		t.Fatalf("RenderTransaction() error = %v", err)
	}
	if !strings.Contains(entry, "assets:ozon    0 RUB = 0 RUB") {
		t.Fatalf("entry missing zero-delta balance assertion: %q", entry)
	}
}

func TestAppendEntry(t *testing.T) {
	existing := []byte("2026-03-10 reconcile assets:card\n    expenses:reconcile    2500 RUB\n    assets:card           -2500 RUB\n")
	entry := "2026-03-15 reconcile assets:card\n    expenses:reconcile    4500 RUB\n    assets:card           -4500 RUB\n\n"
	combined := string(AppendEntry(existing, entry))
	if !strings.Contains(combined, "-2500 RUB\n\n2026-03-15") {
		t.Fatalf("transactions were not separated correctly: %q", combined)
	}
}

func TestAppendDirectivesAndEntryKeepsChangesTogether(t *testing.T) {
	existing := []byte("D 1,000.00 RUB\n")
	entry := "2026-06-19 buy\n    assets:broker    1 AAPL @@ 100 USD\n    assets:cash     -100 USD\n\n"

	directives := []string{
		CommodityDirective("AAPL", domain.AmountFormat{DecimalMark: '.'}),
		CommodityDirective("USD", domain.AmountFormat{DecimalMark: '.'}),
	}
	combined := string(AppendDirectivesAndEntry(existing, directives, entry))
	want := "D 1,000.00 RUB\n\ncommodity 1,000.00 AAPL\ncommodity 1,000.00 USD\n\n2026-06-19 buy\n"
	if !strings.Contains(combined, want) {
		t.Fatalf("combined journal = %q, want block %q", combined, want)
	}
}

func TestCommodityDirectiveFollowsJournalDecimalMark(t *testing.T) {
	if got := CommodityDirective("EUR", domain.AmountFormat{DecimalMark: ','}); got != "commodity 1.000,00 EUR" {
		t.Fatalf("CommodityDirective() = %q", got)
	}
	if got := CommodityDirective("RUB", domain.AmountFormat{}); got != "commodity 1,000.00 RUB" {
		t.Fatalf("CommodityDirective() = %q", got)
	}
}

func TestRenderTransactionUsesJournalDecimalMark(t *testing.T) {
	entry, err := RenderTransaction(domain.Transaction{
		Date:        time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC),
		Description: "lunch",
		Postings: []domain.Posting{
			{Account: "expenses:food", Amount: "1000.5 RUB"},
			{Account: "assets:cash", Amount: "-1000.5 RUB", BalanceAssertion: "2500.25 RUB"},
		},
	}, domain.AmountFormat{DecimalMark: ','})
	if err != nil {
		t.Fatalf("RenderTransaction() error = %v", err)
	}
	for _, want := range []string{"1000,5 RUB", "-1000,5 RUB", "= 2500,25 RUB"} {
		if !strings.Contains(entry, want) {
			t.Fatalf("entry %q missing %q", entry, want)
		}
	}
	if strings.Contains(entry, ".") {
		t.Fatalf("entry %q still contains a dot decimal mark", entry)
	}
}

func TestRenderTransactionKeepsPricedAmounts(t *testing.T) {
	entry, err := RenderTransaction(domain.Transaction{
		Date:        time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC),
		Description: "buy",
		Postings: []domain.Posting{
			{Account: "assets:broker", Amount: "20 AAPL @ 150.5 USD"},
			{Account: "assets:cash", Amount: "-3010 USD"},
		},
	}, domain.AmountFormat{DecimalMark: ','})
	if err != nil {
		t.Fatalf("RenderTransaction() error = %v", err)
	}
	if !strings.Contains(entry, "20 AAPL @ 150,5 USD") {
		t.Fatalf("entry %q missing converted priced amount", entry)
	}
}
