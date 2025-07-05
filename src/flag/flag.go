package flag

import (
	"github.com/open-feature/go-sdk/openfeature"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
)

type Definition struct {
	FlagName string

	DefaultValue   any
	DefaultVariant string

	Rules []rule.ConcreteRule `bson:"rules,omitempty"`
}

// Evaluate walks the Rules in order, returns the first match’s (value,detail),
// or the default if none match.
func (def *Definition) Evaluate(ctx map[string]any) (any, openfeature.ProviderResolutionDetail) {
	for _, rule := range def.Rules {
		if rule.Matches(ctx) {
			return rule.Value(), openfeature.ProviderResolutionDetail{
				Reason:  openfeature.TargetingMatchReason,
				Variant: rule.Variant(),
			}
		}
	}
	return def.DefaultValue, openfeature.ProviderResolutionDetail{
		Reason:  openfeature.DefaultReason,
		Variant: def.DefaultVariant,
	}
}
