package addflow

type PromptKind string

const (
	PromptDate              PromptKind = "date"
	PromptDescription       PromptKind = "description"
	PromptPrimaryAccount    PromptKind = "primary_account"
	PromptInitialExpression PromptKind = "initial_expression"
	PromptCommodityDecision PromptKind = "commodity_decision"
	PromptBalancingAccount  PromptKind = "balancing_account"
	PromptBalancingAmount   PromptKind = "balancing_amount"
	PromptAddAnother        PromptKind = "add_another"
)

type PromptSpec struct {
	Kind             PromptKind
	Label            string
	Default          string
	Remaining        string
	EffectiveBalance string
	Notice           string
	AccountOptions   []AccountOption
}

func NextPrompt(state State, settings Settings) PromptSpec {
	spec := promptForStage(state, settings)
	spec.Notice = state.Notice
	return spec
}

func promptForStage(state State, settings Settings) PromptSpec {
	switch state.Stage {
	case StageNeedDate:
		return PromptSpec{Kind: PromptDate, Label: "Date"}
	case StageNeedDescription:
		return PromptSpec{Kind: PromptDescription, Label: "Description", Default: "by_fin"}
	case StageNeedPrimaryAccount:
		return PromptSpec{Kind: PromptPrimaryAccount, Label: "Account"}
	case StageNeedInitialExpression:
		return PromptSpec{Kind: PromptInitialExpression, Label: "Amount (=target, amount, asset @ price, or asset @@ cost)"}
	case StageNeedCommodityDecision:
		return PromptSpec{Kind: PromptCommodityDecision, Label: `Commodity "` + state.PendingCommodity + `" is undeclared. Add to strict journal?`}
	case StageNeedBalancingAccount:
		return PromptSpec{
			Kind:      PromptBalancingAccount,
			Label:     "Balance account",
			Remaining: state.Residual.Summary(settings.Format),
		}
	case StageNeedBalancingAmount:
		return PromptSpec{
			Kind:      PromptBalancingAmount,
			Label:     "Amount",
			Default:   state.Residual.DefaultPrompt(settings.Format),
			Remaining: state.Residual.Summary(settings.Format),
		}
	case StageNeedAnotherPosting:
		return PromptSpec{Kind: PromptAddAnother, Label: "Add another posting?"}
	default:
		return PromptSpec{}
	}
}
