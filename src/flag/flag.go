package flag

import (
	"github.com/open-feature/go-sdk/openfeature"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
)

type Definition struct {
	FlagName string

	DefaultValue   any
	DefaultVariant string

	Rules []rule.ConcreteRule `bson:"rules"`
}

// Evaluate walks the rules in the definition and returns the highest priority rule that matches the context.
func (def *Definition) Evaluate(ctx map[string]any) (any, openfeature.ProviderResolutionDetail) {
	var currentRule rule.ConcreteRule
	found := false

	currentIsOverride := false
	currentPriority := -1
	for _, rule := range def.Rules {
		ruleOverride, rulePriority := rule.IsOverride(), rule.GetPriority()
		// Skip rules with lower priority than the current rule.
		if rulePriority < currentPriority {
			continue
		}
		// If they are equal, skip the rule if the current rule is an override or the rule is not an override.
		if rulePriority == currentPriority && (currentIsOverride || !ruleOverride) {
			continue
		}
		if rule.Matches(ctx) {
			currentIsOverride = ruleOverride
			currentPriority = rulePriority
			currentRule = rule
			found = true
		}
	}
	if found {
		return currentRule.Value(), openfeature.ProviderResolutionDetail{
			Reason:  openfeature.TargetingMatchReason,
			Variant: currentRule.Variant(),
		}
	}

	return def.DefaultValue, openfeature.ProviderResolutionDetail{
		Reason:  openfeature.DefaultReason,
		Variant: def.DefaultVariant,
	}
}
