package addflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/takzhanov/hledger-utils/internal/domain"
)

type Settings struct {
	DefaultCurrency string
	// Format is how the user punctuates amounts; it comes from the journal.
	Format domain.AmountFormat
}

type Command interface{ isAddCommand() }

type SetDate struct{ Date time.Time }
type SetDescription struct{ Description string }
type SetPrimaryAccount struct{ Account string }
type SetDeclaredCommodities struct{ Declared map[string]struct{} }
type SetInitialExpression struct {
	Expression string
	Balances   map[string]decimal.Decimal
	Declared   map[string]struct{}
}
type DecideCommodityDeclaration struct{ Yes bool }
type SetBalancingAccount struct{ Account string }
type AddPostingExpression struct {
	Account    string
	Expression string
	Balances   map[string]decimal.Decimal
	Declared   map[string]struct{}
}

func (SetDate) isAddCommand()                    {}
func (SetDescription) isAddCommand()             {}
func (SetPrimaryAccount) isAddCommand()          {}
func (SetDeclaredCommodities) isAddCommand()     {}
func (SetInitialExpression) isAddCommand()       {}
func (DecideCommodityDeclaration) isAddCommand() {}
func (SetBalancingAccount) isAddCommand()        {}
func (AddPostingExpression) isAddCommand()       {}

func Reduce(state State, cmd Command, settings Settings) (State, error) {
	state.Notice = ""
	switch value := cmd.(type) {
	case SetDate:
		state.Date = value.Date
		state.Stage = StageNeedDescription
		return state, nil
	case SetDescription:
		state.Description = strings.TrimSpace(value.Description)
		state.Stage = StageNeedPrimaryAccount
		return state, nil
	case SetPrimaryAccount:
		state.PrimaryAccount = value.Account
		state.Stage = StageNeedInitialExpression
		return state, nil
	case SetDeclaredCommodities:
		if state.PendingCommodity == "" {
			return state, nil
		}
		if missing := firstUndeclaredInState(value.Declared, state); missing != "" {
			state.PendingCommodity = missing
			state.Stage = StageNeedCommodityDecision
			return state, nil
		}
		state.PendingCommodity = ""
		if state.Residual.IsZero() {
			state.Stage = StageNeedAnotherPosting
		} else {
			state.Stage = StageNeedBalancingAccount
		}
		return state, nil
	case SetInitialExpression:
		if isZeroPostingExpression(settings, value.Expression) {
			state.PrimaryAccount = ""
			state.Stage = StageNeedPrimaryAccount
			return state, nil
		}
		posting, effect, err := buildPosting(settings, state.PrimaryAccount, value.Expression, value.Balances)
		if err != nil {
			return state, err
		}
		state.InitialPosting = posting
		state.BalancePostings = nil
		state.Residual = NewResidualState()
		state.Residual.MergeNegated(effect)
		if missing := firstUndeclaredInState(value.Declared, state); missing != "" {
			state.PendingCommodity = missing
			state.Stage = StageNeedCommodityDecision
			return state, nil
		}
		state.PendingCommodity = ""
		if state.Residual.IsZero() {
			state.Stage = StageNeedAnotherPosting
		} else {
			state.Stage = StageNeedBalancingAccount
		}
		return state, nil
	case DecideCommodityDeclaration:
		if state.PendingCommodity == "" {
			return state, fmt.Errorf("no pending commodity declaration")
		}
		if !value.Yes {
			return revertPostingWithCommodity(state, state.PendingCommodity), nil
		}
		if !contains(state.NewCommodities, state.PendingCommodity) {
			state.NewCommodities = append(state.NewCommodities, state.PendingCommodity)
		}
		state.Stage = StageNeedCommodityDecision
		return state, nil
	case SetBalancingAccount:
		state.PendingAccount = value.Account
		state.Stage = StageNeedBalancingAmount
		return state, nil
	case AddPostingExpression:
		account := value.Account
		if account == "" {
			account = state.PendingAccount
		}
		if isZeroPostingExpression(settings, value.Expression) {
			state.PendingAccount = ""
			if state.Residual.IsZero() {
				state.Stage = StageNeedAnotherPosting
			} else {
				state.Stage = StageNeedBalancingAccount
			}
			return state, nil
		}
		posting, effect, err := buildPosting(settings, account, value.Expression, value.Balances)
		if err != nil {
			return state, err
		}
		state.BalancePostings = append(state.BalancePostings, posting)
		state.Residual.MergeNegated(effect)
		state.PendingAccount = ""
		if missing := firstUndeclaredInState(value.Declared, state); missing != "" {
			state.PendingCommodity = missing
			state.Stage = StageNeedCommodityDecision
			return state, nil
		}
		state.PendingCommodity = ""
		if state.Residual.IsZero() {
			state.Stage = StageNeedAnotherPosting
		} else {
			state.Stage = StageNeedBalancingAccount
		}
		return state, nil
	default:
		return state, fmt.Errorf("unsupported add command %T", cmd)
	}
}

// revertPostingWithCommodity drops the posting that introduced an undeclared
// commodity and asks for its amount again, so declining the declaration costs
// one posting instead of the whole transaction.
func revertPostingWithCommodity(state State, commodity string) State {
	state.PendingCommodity = ""
	state.Notice = fmt.Sprintf("commodity %q stays undeclared; enter the amount in a declared commodity", commodity)
	for i := len(state.BalancePostings) - 1; i >= 0; i-- {
		if !postingUsesCommodity(state.BalancePostings[i], commodity) {
			continue
		}
		state.PendingAccount = state.BalancePostings[i].Account
		remaining := make([]domain.Posting, 0, len(state.BalancePostings)-1)
		remaining = append(remaining, state.BalancePostings[:i]...)
		remaining = append(remaining, state.BalancePostings[i+1:]...)
		state.BalancePostings = remaining
		state.Residual = recomputeResidual(state)
		state.Stage = StageNeedBalancingAmount
		return state
	}
	if postingUsesCommodity(state.InitialPosting, commodity) {
		state.InitialPosting = domain.Posting{}
		state.Residual = recomputeResidual(state)
		state.Stage = StageNeedInitialExpression
		return state
	}
	state.Residual = recomputeResidual(state)
	state.Stage = stageForResidual(state.Residual)
	return state
}

func postingUsesCommodity(posting domain.Posting, commodity string) bool {
	return contains(postingCurrencies(posting), commodity)
}

func stageForResidual(residual ResidualState) Stage {
	if residual.IsZero() {
		return StageNeedAnotherPosting
	}
	return StageNeedBalancingAccount
}

// recomputeResidual rebuilds the remainder from the postings themselves, which
// is what makes removing a posting possible.
func recomputeResidual(state State) ResidualState {
	residual := NewResidualState()
	residual.MergeNegated(postingEffect(state.InitialPosting))
	for _, posting := range state.BalancePostings {
		residual.MergeNegated(postingEffect(posting))
	}
	return residual
}

// postingEffect is how much a rendered posting contributes to the balance. fin
// wrote the amount itself, so it is always parsed in the canonical format.
func postingEffect(posting domain.Posting) ResidualState {
	effect := NewResidualState()
	amount := strings.TrimSpace(posting.Amount)
	if amount == "" {
		return effect
	}
	if strings.Contains(amount, "@") {
		priced, err := domain.ParsePricedAmountInput(amount, "")
		if err != nil {
			return effect
		}
		sign := decimal.NewFromInt(1)
		if priced.Amount.Value.IsNegative() {
			sign = decimal.NewFromInt(-1)
		}
		effect.Add(priced.Cost.Currency, sign.Mul(priced.TotalCostValue()))
		return effect
	}
	simple, err := domain.ParseAmountInput(amount, "", false)
	if err != nil {
		return effect
	}
	effect.Add(simple.Currency, simple.Value)
	return effect
}

func firstUndeclaredInState(declared map[string]struct{}, state State) string {
	effective := make(map[string]struct{}, len(declared)+len(state.NewCommodities))
	for commodity := range declared {
		effective[commodity] = struct{}{}
	}
	for _, commodity := range state.NewCommodities {
		effective[commodity] = struct{}{}
	}
	if missing := FirstUndeclaredPostingCommodity(effective, state.InitialPosting); missing != "" {
		return missing
	}
	for _, posting := range state.BalancePostings {
		if missing := FirstUndeclaredPostingCommodity(effective, posting); missing != "" {
			return missing
		}
	}
	return ""
}

func buildPosting(settings Settings, account, expression string, balances map[string]decimal.Decimal) (domain.Posting, ResidualState, error) {
	expression, comment := splitPostingComment(expression)
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return domain.Posting{}, ResidualState{}, fmt.Errorf("expression is required")
	}
	if strings.HasPrefix(expression, "=") {
		posting, effect, err := buildBalanceTargetPosting(settings, account, expression, balances)
		if err != nil {
			return domain.Posting{}, ResidualState{}, err
		}
		posting.Comment = comment
		return posting, effect, nil
	}
	posting, effect, err := parsePostingExpression(settings, account, expression)
	if err != nil {
		return domain.Posting{}, ResidualState{}, err
	}
	posting.Comment = comment
	return posting, effect, nil
}

func buildBalanceTargetPosting(settings Settings, account, expression string, balances map[string]decimal.Decimal) (domain.Posting, ResidualState, error) {
	target, err := settings.Format.ParseAmount(strings.TrimSpace(strings.TrimPrefix(expression, "=")), settings.DefaultCurrency, false)
	if err != nil {
		return domain.Posting{}, ResidualState{}, err
	}
	if balances == nil {
		balances = map[string]decimal.Decimal{}
	}
	delta := target.Value.Sub(balances[target.Currency])
	posting := domain.Posting{
		Account:          account,
		Amount:           domain.FormatAmount(delta, target.Currency),
		BalanceAssertion: domain.FormatAmount(target.Value, target.Currency),
	}
	return posting, postingEffect(posting), nil
}

func RequiredEffectsForCommand(state State, cmd Command) []EffectRequest {
	switch value := cmd.(type) {
	case SetInitialExpression:
		if strings.HasPrefix(strings.TrimSpace(value.Expression), "=") {
			return []EffectRequest{NeedBalanceByCommodity{Account: state.PrimaryAccount}}
		}
	case AddPostingExpression:
		if strings.HasPrefix(strings.TrimSpace(value.Expression), "=") {
			account := value.Account
			if account == "" {
				account = state.PendingAccount
			}
			return []EffectRequest{NeedBalanceByCommodity{Account: account}}
		}
	}
	return nil
}

func isZeroPostingExpression(settings Settings, expression string) bool {
	expression, _ = splitPostingComment(expression)
	expression = strings.TrimSpace(expression)
	if expression == "" || strings.HasPrefix(expression, "=") || strings.Contains(expression, "@") {
		return false
	}
	amount, err := settings.Format.ParseAmount(expression, settings.DefaultCurrency, false)
	if err != nil {
		return false
	}
	return amount.Value.IsZero()
}

func parsePostingExpression(settings Settings, account, expression string) (domain.Posting, ResidualState, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return domain.Posting{}, ResidualState{}, fmt.Errorf("amount must be non-zero")
	}
	if strings.Contains(expression, "@") {
		return buildPricedPosting(settings, account, expression)
	}
	amount, err := settings.Format.ParseAmount(expression, settings.DefaultCurrency, false)
	if err != nil {
		return domain.Posting{}, ResidualState{}, err
	}
	if amount.Currency == "" {
		amount.Currency = settings.DefaultCurrency
	}
	if amount.Value.IsZero() {
		return domain.Posting{}, ResidualState{}, fmt.Errorf("amount must be non-zero")
	}
	posting := domain.Posting{
		Account: account,
		Amount:  domain.FormatAmount(amount.Value, amount.Currency),
	}
	return posting, postingEffect(posting), nil
}

func buildPricedPosting(settings Settings, account, expression string) (domain.Posting, ResidualState, error) {
	priced, err := settings.Format.ParsePricedAmount(expression, settings.DefaultCurrency)
	if err != nil {
		return domain.Posting{}, ResidualState{}, err
	}
	if priced.Amount.Value.IsZero() || priced.Cost.Value.IsZero() {
		return domain.Posting{}, ResidualState{}, fmt.Errorf("amount must be non-zero")
	}
	operator := "@@"
	if priced.Unit {
		operator = "@"
	}
	posting := domain.Posting{
		Account: account,
		Amount:  fmt.Sprintf("%s %s %s", domain.FormatAmount(priced.Amount.Value, priced.Amount.Currency), operator, domain.FormatAmount(priced.Cost.Value, priced.Cost.Currency)),
	}
	return posting, postingEffect(posting), nil
}

func FirstUndeclaredPostingCommodity(declared map[string]struct{}, posting domain.Posting) string {
	for _, commodity := range postingCurrencies(posting) {
		if commodity == "" {
			continue
		}
		if _, ok := declared[commodity]; !ok {
			return commodity
		}
	}
	return ""
}

func postingCurrencies(posting domain.Posting) []string {
	commodities := []string{}
	if strings.Contains(posting.Amount, "@") {
		if priced, err := domain.ParsePricedAmountInput(posting.Amount, ""); err == nil {
			commodities = appendCurrency(commodities, priced.Amount.Currency)
			commodities = appendCurrency(commodities, priced.Cost.Currency)
		}
	} else if amount, err := domain.ParseAmountInput(posting.Amount, "", false); err == nil {
		commodities = appendCurrency(commodities, amount.Currency)
	}
	if posting.BalanceAssertion != "" {
		if amount, err := domain.ParseAmountInput(posting.BalanceAssertion, "", false); err == nil {
			commodities = appendCurrency(commodities, amount.Currency)
		}
	}
	return commodities
}

func appendCurrency(values []string, currency string) []string {
	if currency == "" || contains(values, currency) {
		return values
	}
	return append(values, currency)
}

func splitPostingComment(text string) (string, string) {
	text = strings.TrimRight(text, " \t")
	for i := 0; i < len(text); i++ {
		if text[i] != ';' {
			continue
		}
		if i < 2 {
			continue
		}
		if text[i-1] != ' ' || text[i-2] != ' ' {
			continue
		}
		return strings.TrimRight(text[:i], " \t"), strings.TrimSpace(text[i+1:])
	}
	return text, ""
}
