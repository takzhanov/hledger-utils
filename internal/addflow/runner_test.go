package addflow

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/takzhanov/hledger-utils/internal/domain"
)

// scriptedPrompts answers prompts from queued values and records every spec it
// was asked with, so tests can assert what the user would have seen.
type scriptedPrompts struct {
	dates    []time.Time
	accounts []string
	texts    []string
	answers  []bool
	seen     []PromptSpec
}

var errScriptExhausted = errors.New("no scripted answer left")

func (p *scriptedPrompts) PromptDate(spec PromptSpec) (time.Time, error) {
	p.seen = append(p.seen, spec)
	if len(p.dates) == 0 {
		return time.Time{}, errScriptExhausted
	}
	value := p.dates[0]
	p.dates = p.dates[1:]
	return value, nil
}

func (p *scriptedPrompts) PromptAccount(spec PromptSpec) (string, error) {
	p.seen = append(p.seen, spec)
	if len(p.accounts) == 0 {
		return "", errScriptExhausted
	}
	value := p.accounts[0]
	p.accounts = p.accounts[1:]
	return value, nil
}

func (p *scriptedPrompts) PromptString(spec PromptSpec) (string, error) {
	p.seen = append(p.seen, spec)
	if len(p.texts) == 0 {
		return "", errScriptExhausted
	}
	value := p.texts[0]
	p.texts = p.texts[1:]
	return value, nil
}

func (p *scriptedPrompts) PromptYesNo(spec PromptSpec) (bool, error) {
	p.seen = append(p.seen, spec)
	if len(p.answers) == 0 {
		return false, errScriptExhausted
	}
	value := p.answers[0]
	p.answers = p.answers[1:]
	return value, nil
}

func (p *scriptedPrompts) specsOfKind(kind PromptKind) []PromptSpec {
	specs := make([]PromptSpec, 0, len(p.seen))
	for _, spec := range p.seen {
		if spec.Kind == kind {
			specs = append(specs, spec)
		}
	}
	return specs
}

func journalEffects() stubEffects {
	return stubEffects{responses: map[EffectRequest]any{
		NeedDeclaredCommodities{}: map[string]struct{}{"RUB": {}},
		NeedAccountOptions{}: []AccountOption{
			{Account: "expenses:food"},
			{Account: "assets:cash"},
		},
		NeedBalanceByCommodity{Account: "assets:cash"}:   map[string]decimal.Decimal{},
		NeedBalanceByCommodity{Account: "expenses:food"}: map[string]decimal.Decimal{},
	}}
}

func TestRunReAsksAfterRejectedAmount(t *testing.T) {
	prompts := &scriptedPrompts{
		dates:    []time.Time{time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)},
		accounts: []string{"expenses:food", "assets:cash"},
		texts:    []string{"lunch", "1000,50", "1000", "-1000"},
		answers:  []bool{false},
	}

	result, err := Run(NewState(), prompts, journalEffects(), Settings{
		DefaultCurrency: "RUB",
		Format:          domain.AmountFormat{DecimalMark: '.'},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	asked := prompts.specsOfKind(PromptInitialExpression)
	if len(asked) != 2 {
		t.Fatalf("initial expression asked %d times, want 2", len(asked))
	}
	if asked[0].Notice != "" {
		t.Fatalf("first prompt notice = %q, want empty", asked[0].Notice)
	}
	if asked[1].Notice == "" {
		t.Fatal("repeated prompt should explain why the amount was rejected")
	}
	tx := result.Transaction()
	if len(tx.Postings) != 2 {
		t.Fatalf("postings = %d, want 2", len(tx.Postings))
	}
	if tx.Postings[0].Amount != "1000 RUB" {
		t.Fatalf("primary amount = %q, want %q", tx.Postings[0].Amount, "1000 RUB")
	}
	if tx.Description != "lunch" {
		t.Fatalf("description = %q, want %q", tx.Description, "lunch")
	}
}

func TestRunKeepsEnteredDataWhenPromptFails(t *testing.T) {
	prompts := &scriptedPrompts{
		dates:    []time.Time{time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)},
		accounts: []string{"expenses:food"},
		texts:    []string{"lunch"},
	}

	if _, err := Run(NewState(), prompts, journalEffects(), Settings{DefaultCurrency: "RUB"}); !errors.Is(err, errScriptExhausted) {
		t.Fatalf("Run() error = %v, want %v", err, errScriptExhausted)
	}
}

func TestDecliningCommodityDeclarationRevertsInitialPosting(t *testing.T) {
	state := NewState()
	state.Stage = StageNeedInitialExpression
	state.PrimaryAccount = "assets:cash"
	state.Date = time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	state.Description = "lunch"
	settings := Settings{DefaultCurrency: "RUB"}

	state, err := Reduce(state, SetInitialExpression{
		Expression: "100 USD",
		Declared:   map[string]struct{}{"RUB": {}},
	}, settings)
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if state.Stage != StageNeedCommodityDecision {
		t.Fatalf("stage = %q, want %q", state.Stage, StageNeedCommodityDecision)
	}

	state, err = Reduce(state, DecideCommodityDeclaration{Yes: false}, settings)
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if state.Stage != StageNeedInitialExpression {
		t.Fatalf("stage = %q, want %q", state.Stage, StageNeedInitialExpression)
	}
	if state.InitialPosting != (domain.Posting{}) {
		t.Fatalf("InitialPosting = %+v, want empty", state.InitialPosting)
	}
	if !state.Residual.IsZero() {
		t.Fatalf("residual = %q, want empty", state.Residual.Summary(domain.AmountFormat{}))
	}
	if state.PrimaryAccount != "assets:cash" || state.Description != "lunch" {
		t.Fatalf("declining a declaration must keep the rest of the entry: %+v", state)
	}
	if state.Notice == "" {
		t.Fatal("declining a declaration should explain what happens next")
	}
	if NextPrompt(state, Settings{}).Notice != state.Notice {
		t.Fatal("the notice should reach the next prompt")
	}
}

func TestDecliningCommodityDeclarationRevertsBalancingPosting(t *testing.T) {
	settings := Settings{DefaultCurrency: "RUB"}
	state := NewState()
	state.Stage = StageNeedBalancingAmount
	state.PrimaryAccount = "expenses:food"
	state.InitialPosting = domain.Posting{Account: "expenses:food", Amount: "1000 RUB"}
	state.Residual = NewResidualState()
	state.Residual.Add("RUB", decimal.NewFromInt(-1000))
	state.PendingAccount = "assets:cash"

	state, err := Reduce(state, AddPostingExpression{
		Expression: "-10 USD",
		Declared:   map[string]struct{}{"RUB": {}},
	}, settings)
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if state.Stage != StageNeedCommodityDecision {
		t.Fatalf("stage = %q, want %q", state.Stage, StageNeedCommodityDecision)
	}

	state, err = Reduce(state, DecideCommodityDeclaration{Yes: false}, settings)
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if state.Stage != StageNeedBalancingAmount {
		t.Fatalf("stage = %q, want %q", state.Stage, StageNeedBalancingAmount)
	}
	if state.PendingAccount != "assets:cash" {
		t.Fatalf("PendingAccount = %q, want %q", state.PendingAccount, "assets:cash")
	}
	if len(state.BalancePostings) != 0 {
		t.Fatalf("BalancePostings = %+v, want empty", state.BalancePostings)
	}
	if got, want := state.Residual.Summary(domain.AmountFormat{}), "-1000 RUB"; got != want {
		t.Fatalf("residual = %q, want %q", got, want)
	}
}

func TestJournalFormatReachesAmountParsing(t *testing.T) {
	state := NewState()
	state.Stage = StageNeedInitialExpression
	state.PrimaryAccount = "assets:cash"
	settings := Settings{DefaultCurrency: "RUB", Format: domain.AmountFormat{DecimalMark: ','}}

	next, err := Reduce(state, SetInitialExpression{
		Expression: "1000,50",
		Declared:   map[string]struct{}{"RUB": {}},
	}, settings)
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if got, want := next.InitialPosting.Amount, "1000.5 RUB"; got != want {
		t.Fatalf("amount = %q, want %q", got, want)
	}
}
