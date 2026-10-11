package addflow

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/takzhanov/hledger-utils/internal/domain"
)

type Stage string

const (
	StageNeedDate              Stage = "need_date"
	StageNeedDescription       Stage = "need_description"
	StageNeedPrimaryAccount    Stage = "need_primary_account"
	StageNeedInitialExpression Stage = "need_initial_expression"
	StageNeedCommodityDecision Stage = "need_commodity_decision"
	StageNeedBalancingAccount  Stage = "need_balancing_account"
	StageNeedBalancingAmount   Stage = "need_balancing_amount"
	StageNeedAnotherPosting    Stage = "need_another_posting"
	StageDone                  Stage = "done"
)

type State struct {
	Stage            Stage
	Date             time.Time
	Description      string
	PrimaryAccount   string
	InitialPosting   domain.Posting
	BalancePostings  []domain.Posting
	Residual         ResidualState
	PendingAccount   string
	PendingCommodity string
	NewCommodities   []string
	// Notice explains to the user why the next prompt is being asked again.
	Notice string
}

func NewState() State {
	return State{Stage: StageNeedDate}
}

func (s State) Transaction() domain.Transaction {
	return domain.Transaction{
		Date:        s.Date,
		Description: s.Description,
		Postings:    append([]domain.Posting{s.InitialPosting}, s.BalancePostings...),
	}
}

type ResidualState struct {
	order []string
	items map[string]decimal.Decimal
	last  string
}

func NewResidualState() ResidualState {
	return ResidualState{items: map[string]decimal.Decimal{}}
}

func (r *ResidualState) Add(currency string, value decimal.Decimal) {
	if value.IsZero() {
		return
	}
	if r.items == nil {
		r.items = map[string]decimal.Decimal{}
	}
	if _, ok := r.items[currency]; !ok && !contains(r.order, currency) {
		r.order = append(r.order, currency)
	}
	r.items[currency] = r.items[currency].Add(value)
	if r.items[currency].IsZero() {
		delete(r.items, currency)
	}
	r.last = currency
}

func (r *ResidualState) MergeNegated(other ResidualState) {
	for _, currency := range other.order {
		if value, ok := other.items[currency]; ok {
			r.Add(currency, value.Neg())
		}
	}
}

func (r ResidualState) IsZero() bool {
	return len(r.items) == 0
}

func (r ResidualState) DefaultPrompt(format domain.AmountFormat) string {
	if len(r.items) == 0 {
		return ""
	}
	if r.last != "" {
		if value, ok := r.items[r.last]; ok {
			return format.RenderAmount(value, r.last)
		}
	}
	for _, currency := range r.order {
		if value, ok := r.items[currency]; ok {
			return format.RenderAmount(value, currency)
		}
	}
	return ""
}

func (r ResidualState) Summary(format domain.AmountFormat) string {
	parts := make([]string, 0, len(r.items))
	for _, currency := range r.order {
		if value, ok := r.items[currency]; ok {
			parts = append(parts, format.RenderAmount(value, currency))
		}
	}
	return strings.Join(parts, ", ")
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
