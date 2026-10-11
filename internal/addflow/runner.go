package addflow

import (
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/takzhanov/hledger-utils/internal/domain"
)

type Prompts interface {
	PromptDate(spec PromptSpec) (time.Time, error)
	PromptAccount(spec PromptSpec) (string, error)
	PromptString(spec PromptSpec) (string, error)
	PromptYesNo(spec PromptSpec) (bool, error)
}

type Result struct {
	State State
}

func Run(initial State, prompts Prompts, effects EffectResolver, settings Settings) (Result, error) {
	state := initial

	for {
		spec, err := hydratePrompt(state, NextPrompt(state, settings), effects, settings)
		if err != nil {
			return Result{}, err
		}
		switch spec.Kind {
		case PromptDate:
			date, err := prompts.PromptDate(spec)
			if err != nil {
				return Result{}, err
			}
			state = apply(state, SetDate{Date: date}, settings)
		case PromptDescription:
			description, err := prompts.PromptString(spec)
			if err != nil {
				return Result{}, err
			}
			state = apply(state, SetDescription{Description: description}, settings)
		case PromptPrimaryAccount:
			account, err := prompts.PromptAccount(spec)
			if err != nil {
				return Result{}, err
			}
			state = apply(state, SetPrimaryAccount{Account: account}, settings)
		case PromptInitialExpression:
			expression, err := prompts.PromptString(spec)
			if err != nil {
				return Result{}, err
			}
			cmd := SetInitialExpression{Expression: expression}
			cmd, err = hydrateInitialExpression(state, cmd, effects)
			if err != nil {
				return Result{}, err
			}
			state = apply(state, cmd, settings)
		case PromptBalancingAccount:
			account, err := prompts.PromptAccount(spec)
			if err != nil {
				return Result{}, err
			}
			state = apply(state, SetBalancingAccount{Account: account}, settings)
		case PromptBalancingAmount:
			amount, err := prompts.PromptString(spec)
			if err != nil {
				return Result{}, err
			}
			cmd := AddPostingExpression{Expression: amount}
			cmd, err = hydrateAddPostingExpression(state, cmd, effects)
			if err != nil {
				return Result{}, err
			}
			state = apply(state, cmd, settings)
		case PromptCommodityDecision:
			yes, err := prompts.PromptYesNo(spec)
			if err != nil {
				return Result{}, err
			}
			state = apply(state, DecideCommodityDeclaration{Yes: yes}, settings)
			if yes {
				state, err = refreshDeclaredCommodities(state, effects, settings)
				if err != nil {
					return Result{}, err
				}
			}
		case PromptAddAnother:
			addMore, err := prompts.PromptYesNo(spec)
			if err != nil {
				return Result{}, err
			}
			if !addMore {
				return Result{State: state}, nil
			}
			state.Stage = StageNeedBalancingAccount
		default:
			return Result{State: state}, nil
		}
	}
}

// apply keeps a rejected value from ending the flow: the stage is unchanged, so
// the loop asks the same prompt again and shows why.
func apply(state State, cmd Command, settings Settings) State {
	next, err := Reduce(state, cmd, settings)
	if err != nil {
		state.Notice = err.Error()
		return state
	}
	return next
}

func (r Result) Transaction() domain.Transaction {
	return r.State.Transaction()
}

func hydratePrompt(state State, spec PromptSpec, effects EffectResolver, settings Settings) (PromptSpec, error) {
	switch spec.Kind {
	case PromptPrimaryAccount, PromptBalancingAccount:
		resolved, err := effects.ResolveEffect(NeedAccountOptions{})
		if err != nil {
			return spec, err
		}
		spec.AccountOptions = resolved.([]AccountOption)
	case PromptBalancingAmount:
		if state.PendingAccount == "" {
			return spec, nil
		}
		resolved, err := effects.ResolveEffect(NeedBalanceByCommodity{Account: state.PendingAccount})
		if err != nil {
			return spec, nil
		}
		effective := effectiveBalancesForAccount(state, state.PendingAccount, resolved.(map[string]decimal.Decimal))
		spec.EffectiveBalance = summarizeBalances(effective, settings.Format)
	}
	return spec, nil
}

func refreshDeclaredCommodities(state State, effects EffectResolver, settings Settings) (State, error) {
	resolved, err := effects.ResolveEffect(NeedDeclaredCommodities{})
	if err != nil {
		return state, err
	}
	declared := resolved.(map[string]struct{})
	for _, commodity := range state.NewCommodities {
		declared[commodity] = struct{}{}
	}
	return Reduce(state, SetDeclaredCommodities{Declared: declared}, settings)
}

func hydrateInitialExpression(state State, cmd SetInitialExpression, effects EffectResolver) (SetInitialExpression, error) {
	resolved, err := effects.ResolveEffect(NeedDeclaredCommodities{})
	if err != nil {
		return cmd, err
	}
	cmd.Declared = resolved.(map[string]struct{})
	for _, request := range RequiredEffectsForCommand(state, cmd) {
		resolved, err := effects.ResolveEffect(request)
		if err != nil {
			return cmd, err
		}
		switch value := request.(type) {
		case NeedBalanceByCommodity:
			cmd.Balances = effectiveBalancesForAccount(state, value.Account, resolved.(map[string]decimal.Decimal))
		}
	}
	return cmd, nil
}

func hydrateAddPostingExpression(state State, cmd AddPostingExpression, effects EffectResolver) (AddPostingExpression, error) {
	resolved, err := effects.ResolveEffect(NeedDeclaredCommodities{})
	if err != nil {
		return cmd, err
	}
	cmd.Declared = resolved.(map[string]struct{})
	for _, request := range RequiredEffectsForCommand(state, cmd) {
		resolved, err := effects.ResolveEffect(request)
		if err != nil {
			return cmd, err
		}
		switch value := request.(type) {
		case NeedBalanceByCommodity:
			cmd.Balances = effectiveBalancesForAccount(state, value.Account, resolved.(map[string]decimal.Decimal))
		}
	}
	return cmd, nil
}

func effectiveBalancesForAccount(state State, account string, ledger map[string]decimal.Decimal) map[string]decimal.Decimal {
	result := make(map[string]decimal.Decimal, len(ledger))
	for currency, value := range ledger {
		result[currency] = value
	}
	mergePostingDelta(result, state.InitialPosting, account)
	for _, posting := range state.BalancePostings {
		mergePostingDelta(result, posting, account)
	}
	return result
}

func summarizeBalances(items map[string]decimal.Decimal, format domain.AmountFormat) string {
	if len(items) == 0 {
		return ""
	}
	residual := NewResidualState()
	currencies := make([]string, 0, len(items))
	for currency := range items {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	for _, currency := range currencies {
		value := items[currency]
		residual.Add(currency, value)
	}
	return residual.Summary(format)
}

func mergePostingDelta(target map[string]decimal.Decimal, posting domain.Posting, account string) {
	if posting.Account != account {
		return
	}
	amount := strings.TrimSpace(posting.Amount)
	if amount == "" {
		return
	}
	if strings.Contains(amount, "@") {
		priced, err := domain.ParsePricedAmountInput(amount, "")
		if err != nil {
			return
		}
		target[priced.Amount.Currency] = target[priced.Amount.Currency].Add(priced.Amount.Value)
		if target[priced.Amount.Currency].IsZero() {
			delete(target, priced.Amount.Currency)
		}
		return
	}
	simple, err := domain.ParseAmountInput(amount, "", false)
	if err != nil {
		return
	}
	target[simple.Currency] = target[simple.Currency].Add(simple.Value)
	if target[simple.Currency].IsZero() {
		delete(target, simple.Currency)
	}
}
