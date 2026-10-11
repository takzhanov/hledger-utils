package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type Transaction struct {
	Date        time.Time
	Description string
	Postings    []Posting
}

type Posting struct {
	Account          string
	Amount           string
	BalanceAssertion string
	Comment          string
}

func (t Transaction) Validate() error {
	if t.Date.IsZero() {
		return errors.New("transaction date is required")
	}
	if strings.TrimSpace(t.Description) == "" {
		return errors.New("transaction description is required")
	}
	if len(t.Postings) == 0 {
		return errors.New("transaction must have at least one posting")
	}
	if len(t.Postings) == 1 && !isZeroDeltaBalanceAssertion(t.Postings[0]) {
		return errors.New("transaction must have at least two postings unless it is a zero-delta balance assertion")
	}
	for _, posting := range t.Postings {
		if strings.TrimSpace(posting.Account) == "" {
			return errors.New("posting account is required")
		}
		if strings.TrimSpace(posting.Amount) == "" {
			return fmt.Errorf("posting amount is required for account %q", posting.Account)
		}
	}
	return nil
}

func isZeroDeltaBalanceAssertion(posting Posting) bool {
	if strings.TrimSpace(posting.BalanceAssertion) == "" {
		return false
	}
	amount, err := ParseAmountInput(posting.Amount, "", false)
	return err == nil && amount.Value.IsZero()
}

func FormatAmount(value decimal.Decimal, currency string) string {
	text := value.String()
	if currency == "" {
		return text
	}
	return fmt.Sprintf("%s %s", text, currency)
}
