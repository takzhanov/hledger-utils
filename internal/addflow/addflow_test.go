package addflow

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/takzhanov/hledger-utils/internal/domain"
)

type stubEffects struct {
	responses map[EffectRequest]any
}

func (s stubEffects) ResolveEffect(request EffectRequest) (any, error) {
	switch value := request.(type) {
	case NeedBalanceByCommodity:
		for key, response := range s.responses {
			candidate, ok := key.(NeedBalanceByCommodity)
			if ok && candidate.Account == value.Account {
				return response, nil
			}
		}
	case NeedDeclaredCommodities:
		if response, ok := s.responses[NeedDeclaredCommodities{}]; ok {
			return response, nil
		}
	case NeedAccountOptions:
		if response, ok := s.responses[NeedAccountOptions{}]; ok {
			return response, nil
		}
	}
	return nil, nil
}

type stubGateway struct {
	balances map[string]map[string]decimal.Decimal
}

func (s stubGateway) BalanceByCommodity(account string) (map[string]decimal.Decimal, error) {
	if value, ok := s.balances[account]; ok {
		return value, nil
	}
	return map[string]decimal.Decimal{}, nil
}

func TestResidualSummaryDoesNotRepeatCurrencyAfterZeroCycle(t *testing.T) {
	residual := NewResidualState()
	residual.Add("RUB", decimal.NewFromInt(-1000))
	residual.Add("RUB", decimal.NewFromInt(1000))
	residual.Add("RUB", decimal.NewFromInt(-1000))

	if got, want := residual.Summary(domain.AmountFormat{}), "-1000 RUB"; got != want {
		t.Fatalf("Summary() = %q, want %q", got, want)
	}
}

func TestReduceInitialExpressionSetsResidual(t *testing.T) {
	state := NewState()
	state.PrimaryAccount = "assets:cash"
	next, err := Reduce(state, SetInitialExpression{Expression: "usd 90 @ 100"}, Settings{DefaultCurrency: "RUB"})
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if next.InitialPosting.Amount != "90 usd @ 100 RUB" {
		t.Fatalf("unexpected posting: %+v", next.InitialPosting)
	}
	if got, want := next.Residual.Summary(domain.AmountFormat{}), "-9000 RUB"; got != want {
		t.Fatalf("Residual.Summary() = %q, want %q", got, want)
	}
}

func TestZeroInitialPostingReturnsToAccountSelection(t *testing.T) {
	state := NewState()
	state.Stage = StageNeedInitialExpression
	state.PrimaryAccount = "liabilities:person-a"

	next, err := Reduce(state, SetInitialExpression{
		Expression: "0",
		Declared:   map[string]struct{}{"RUB": {}},
	}, Settings{DefaultCurrency: "RUB"})
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if next.Stage != StageNeedPrimaryAccount {
		t.Fatalf("stage = %q, want %q", next.Stage, StageNeedPrimaryAccount)
	}
	if prompt := NextPrompt(next, Settings{}); prompt.Kind != PromptPrimaryAccount {
		t.Fatalf("next prompt = %q, want %q", prompt.Kind, PromptPrimaryAccount)
	}
	if next.PrimaryAccount != "" {
		t.Fatalf("PrimaryAccount = %q, want empty", next.PrimaryAccount)
	}
	if next.InitialPosting != (domain.Posting{}) {
		t.Fatalf("InitialPosting = %+v, want empty", next.InitialPosting)
	}
}

func TestBalanceTargetUsesExistingCommodityBalance(t *testing.T) {
	state := NewState()
	state.PrimaryAccount = "assets:cash"
	gateway := stubGateway{
		balances: map[string]map[string]decimal.Decimal{
			"assets:cash": {"RUB": decimal.NewFromInt(-2000), "usdt": decimal.NewFromInt(33)},
		},
	}
	next, err := Reduce(state, SetInitialExpression{
		Expression: "=100",
		Balances:   gateway.balances["assets:cash"],
	}, Settings{DefaultCurrency: "RUB"})
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if next.InitialPosting.Amount != "2100 RUB" || next.InitialPosting.BalanceAssertion != "100 RUB" {
		t.Fatalf("unexpected posting: %+v", next.InitialPosting)
	}
	if got, want := next.Residual.Summary(domain.AmountFormat{}), "-2100 RUB"; got != want {
		t.Fatalf("Residual.Summary() = %q, want %q", got, want)
	}
}

func TestBalanceTargetWithoutDefaultCurrencyStaysBareNumber(t *testing.T) {
	state := NewState()
	state.PrimaryAccount = "assets:cash"
	next, err := Reduce(state, SetInitialExpression{
		Expression: "=100",
		Balances:   map[string]decimal.Decimal{"RUB": decimal.NewFromInt(-2000), "usdt": decimal.NewFromInt(33)},
	}, Settings{})
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if next.InitialPosting.Amount != "100" || next.InitialPosting.BalanceAssertion != "100" {
		t.Fatalf("unexpected posting: %+v", next.InitialPosting)
	}
	if got, want := next.Residual.Summary(domain.AmountFormat{}), "-100"; got != want {
		t.Fatalf("Residual.Summary() = %q, want %q", got, want)
	}
}

func TestNextPromptUsesStateStage(t *testing.T) {
	state := NewState()
	if got := NextPrompt(state, Settings{}).Kind; got != PromptDate {
		t.Fatalf("NextPrompt() = %q, want %q", got, PromptDate)
	}
	state.Stage = StageNeedDescription
	spec := NextPrompt(state, Settings{})
	if spec.Kind != PromptDescription {
		t.Fatalf("NextPrompt() kind = %q, want %q", spec.Kind, PromptDescription)
	}
	if spec.Default != "by_fin" {
		t.Fatalf("NextPrompt() default = %q", spec.Default)
	}
	state.Stage = StageNeedBalancingAmount
	state.Residual.Add("RUB", decimal.NewFromInt(-1000))
	spec = NextPrompt(state, Settings{})
	if spec.Kind != PromptBalancingAmount {
		t.Fatalf("NextPrompt() kind = %q, want %q", spec.Kind, PromptBalancingAmount)
	}
	if spec.Default != "-1000 RUB" {
		t.Fatalf("NextPrompt() default = %q", spec.Default)
	}
}

func TestReduceDescriptionMovesToPrimaryAccount(t *testing.T) {
	state := NewState()
	state.Stage = StageNeedDescription

	next, err := Reduce(state, SetDescription{Description: "by_fin"}, Settings{})
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if next.Description != "by_fin" {
		t.Fatalf("Description = %q, want %q", next.Description, "by_fin")
	}
	if next.Stage != StageNeedPrimaryAccount {
		t.Fatalf("stage = %q, want %q", next.Stage, StageNeedPrimaryAccount)
	}
}

func TestTransactionUsesProvidedDescription(t *testing.T) {
	state := NewState()
	state.Description = "by_fin"
	state.Date = time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)
	state.InitialPosting = domainPosting("assets:cash", "100 RUB", "")
	state.BalancePostings = []domain.Posting{
		domainPosting("income:salary", "-100 RUB", ""),
	}

	tx := state.Transaction()
	if tx.Description != "by_fin" {
		t.Fatalf("Description = %q, want %q", tx.Description, "by_fin")
	}
}

func TestReduceInitialExpressionMovesToCommodityDecisionWhenUndeclared(t *testing.T) {
	state := NewState()
	state.PrimaryAccount = "assets:cash"

	next, err := Reduce(state, SetInitialExpression{
		Expression: "10 usdc @@ 6000 RUB",
		Declared:   map[string]struct{}{"RUB": {}},
	}, Settings{DefaultCurrency: "RUB"})
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if next.Stage != StageNeedCommodityDecision {
		t.Fatalf("stage = %q, want %q", next.Stage, StageNeedCommodityDecision)
	}
	if next.PendingCommodity != "usdc" {
		t.Fatalf("PendingCommodity = %q, want %q", next.PendingCommodity, "usdc")
	}
}

func TestCommodityDeclarationIsDeferredInState(t *testing.T) {
	state := NewState()
	state.Stage = StageNeedCommodityDecision
	state.PendingCommodity = "usdc"

	next, err := Reduce(state, DecideCommodityDeclaration{Yes: true}, Settings{})
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if len(next.NewCommodities) != 1 || next.NewCommodities[0] != "usdc" {
		t.Fatalf("NewCommodities = %v, want [usdc]", next.NewCommodities)
	}
}

func TestDeferredCommodityDoesNotPromptAgainForBalancingPosting(t *testing.T) {
	state := NewState()
	state.Stage = StageNeedBalancingAmount
	state.PendingAccount = "assets:bank"
	state.InitialPosting = domainPosting("assets:cash", "10 aple", "")
	state.NewCommodities = []string{"aple"}
	state.Residual.Add("aple", decimal.NewFromInt(-10))

	next, err := Reduce(state, AddPostingExpression{
		Expression: "-10 aple",
		Declared:   map[string]struct{}{},
	}, Settings{})
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if next.Stage != StageNeedAnotherPosting {
		t.Fatalf("stage = %q, want %q", next.Stage, StageNeedAnotherPosting)
	}
	if next.PendingCommodity != "" {
		t.Fatalf("PendingCommodity = %q, want empty", next.PendingCommodity)
	}
}

func TestZeroAdditionalPostingIsIgnored(t *testing.T) {
	state := NewState()
	state.Stage = StageNeedBalancingAmount
	state.PendingAccount = "liabilities:person-b"
	state.InitialPosting = domainPosting("liabilities:person-c", "40000 RUB", "")
	state.BalancePostings = []domain.Posting{
		domainPosting("assets:bank:checking", "-40000 RUB", ""),
	}

	next, err := Reduce(state, AddPostingExpression{
		Expression: "0",
		Declared:   map[string]struct{}{"RUB": {}},
	}, Settings{DefaultCurrency: "RUB"})
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	if next.Stage != StageNeedAnotherPosting {
		t.Fatalf("stage = %q, want %q", next.Stage, StageNeedAnotherPosting)
	}
	if next.PendingAccount != "" {
		t.Fatalf("PendingAccount = %q, want empty", next.PendingAccount)
	}
	if len(next.BalancePostings) != 1 {
		t.Fatalf("BalancePostings length = %d, want 1", len(next.BalancePostings))
	}
}

func TestBuildPostingParsesInlineComment(t *testing.T) {
	posting, _, err := buildPosting(Settings{DefaultCurrency: "RUB"}, "assets:cash", "100 RUB  ; lunch", nil)
	if err != nil {
		t.Fatalf("buildPosting() error = %v", err)
	}
	if posting.Amount != "100 RUB" {
		t.Fatalf("Amount = %q", posting.Amount)
	}
	if posting.Comment != "lunch" {
		t.Fatalf("Comment = %q", posting.Comment)
	}
}

func TestEffectiveBalancesForAccountIncludesInFlightPostings(t *testing.T) {
	state := NewState()
	state.PrimaryAccount = "assets:cash"
	state.InitialPosting = domainPosting("assets:cash", "80 RUB", "100 RUB")
	state.BalancePostings = []domain.Posting{
		domainPosting("assets:cash", "100 RUB", "120 RUB"),
		domainPosting("income:salary", "-180 RUB", ""),
	}

	got := effectiveBalancesForAccount(state, "assets:cash", map[string]decimal.Decimal{"RUB": decimal.NewFromInt(20), "usdt": decimal.NewFromInt(33)})
	if !got["RUB"].Equal(decimal.NewFromInt(200)) {
		t.Fatalf("effective RUB balance = %s, want %s", got["RUB"], decimal.NewFromInt(200))
	}
	if !got["usdt"].Equal(decimal.NewFromInt(33)) {
		t.Fatalf("effective usdt balance = %s, want %s", got["usdt"], decimal.NewFromInt(33))
	}
}

func TestHydratePromptAddsEffectiveBalanceHint(t *testing.T) {
	state := NewState()
	state.Stage = StageNeedBalancingAmount
	state.PrimaryAccount = "assets:cash"
	state.PendingAccount = "assets:cash"
	state.InitialPosting = domainPosting("assets:cash", "80 RUB", "100 RUB")

	spec, err := hydratePrompt(state, NextPrompt(state, Settings{}), stubEffects{
		responses: map[EffectRequest]any{
			NeedBalanceByCommodity{Account: "assets:cash"}: map[string]decimal.Decimal{"RUB": decimal.NewFromInt(20), "usdt": decimal.NewFromInt(33)},
		},
	}, Settings{})
	if err != nil {
		t.Fatalf("hydratePrompt() error = %v", err)
	}
	if got, want := spec.EffectiveBalance, "100 RUB, 33 usdt"; got != want {
		t.Fatalf("EffectiveBalance = %q, want %q", got, want)
	}
}

func domainPosting(account, amount, assertion string) domain.Posting {
	return domain.Posting{
		Account:          account,
		Amount:           amount,
		BalanceAssertion: assertion,
	}
}
