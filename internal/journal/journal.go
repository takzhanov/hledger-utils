package journal

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/takzhanov/hledger-utils/internal/domain"
)

func RenderTransaction(tx domain.Transaction, format domain.AmountFormat) (string, error) {
	if err := tx.Validate(); err != nil {
		return "", err
	}

	postings, err := renderAmounts(tx.Postings, format)
	if err != nil {
		return "", err
	}

	maxAccountLen := 0
	for _, posting := range postings {
		if len(posting.Account) > maxAccountLen {
			maxAccountLen = len(posting.Account)
		}
	}

	var buffer bytes.Buffer
	buffer.WriteString(tx.Date.Format("2006-01-02"))
	buffer.WriteByte(' ')
	buffer.WriteString(tx.Description)
	buffer.WriteByte('\n')
	for _, posting := range postings {
		padding := maxAccountLen - len(posting.Account) + 4
		buffer.WriteString("    ")
		buffer.WriteString(posting.Account)
		buffer.WriteString(strings.Repeat(" ", padding))
		buffer.WriteString(posting.Amount)
		if posting.BalanceAssertion != "" {
			buffer.WriteString(" = ")
			buffer.WriteString(posting.BalanceAssertion)
		}
		if strings.TrimSpace(posting.Comment) != "" {
			buffer.WriteString("  ;")
			buffer.WriteString(strings.TrimSpace(posting.Comment))
		}
		buffer.WriteByte('\n')
	}
	buffer.WriteByte('\n')
	return buffer.String(), nil
}

func renderAmounts(postings []domain.Posting, format domain.AmountFormat) ([]domain.Posting, error) {
	rendered := make([]domain.Posting, 0, len(postings))
	for _, posting := range postings {
		amount, err := format.RenderAmountText(posting.Amount)
		if err != nil {
			return nil, err
		}
		assertion, err := format.RenderAmountText(posting.BalanceAssertion)
		if err != nil {
			return nil, err
		}
		posting.Amount = amount
		posting.BalanceAssertion = assertion
		rendered = append(rendered, posting)
	}
	return rendered, nil
}

func Replace(path string, content []byte) error {
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write journal file: %w", err)
	}
	return nil
}

func AppendEntry(existing []byte, entry string) []byte {
	if len(existing) == 0 {
		return []byte(entry)
	}
	result := append([]byte{}, existing...)
	if !bytes.HasSuffix(result, []byte("\n\n")) {
		if !bytes.HasSuffix(result, []byte("\n")) {
			result = append(result, '\n')
		}
		result = append(result, '\n')
	}
	return append(result, []byte(entry)...)
}

// CommodityDirective declares a commodity with a sample amount punctuated the
// way the journal already punctuates amounts.
func CommodityDirective(commodity string, format domain.AmountFormat) string {
	return fmt.Sprintf("commodity %s %s", format.SampleAmount(), commodity)
}

func AppendDirectivesAndEntry(existing []byte, directives []string, entry string) []byte {
	result := append([]byte{}, existing...)
	if len(directives) > 0 {
		result = AppendEntry(result, strings.Join(directives, "\n")+"\n\n")
	}
	return AppendEntry(result, entry)
}
