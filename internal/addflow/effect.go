package addflow

type EffectRequest interface{ isEffectRequest() }

type AccountOption struct {
	Account string
	Balance string
}

type NeedDeclaredCommodities struct{}
type NeedAccountOptions struct{}
type NeedBalanceByCommodity struct {
	Account string
}

func (NeedDeclaredCommodities) isEffectRequest() {}
func (NeedAccountOptions) isEffectRequest()      {}
func (NeedBalanceByCommodity) isEffectRequest()  {}

type EffectResolver interface {
	ResolveEffect(request EffectRequest) (any, error)
}
