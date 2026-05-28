package flag

import (
	"github.com/open-feature/go-sdk/openfeature"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
)

type Definition struct {
	FlagName string

	DefaultValue   any
	DefaultVariant string
	Category       string `bson:"category,omitempty"` // UI-only grouping in the flag editor

	Rules []rule.ConcreteRule `bson:"rules"`
}

// EvaluationMatch is the full outcome of evaluating a flag definition, including
// which top-level rule won (if any).
type EvaluationMatch struct {
	Value            any
	Detail           openfeature.ProviderResolutionDetail
	MatchedRuleIndex int // 0-based index into Definition.Rules; -1 when default
}

// Evaluate walks the rules in the definition and returns the highest priority rule that matches the context.
func (def *Definition) Evaluate(ctx map[string]any) (any, openfeature.ProviderResolutionDetail) {
	match := def.EvaluateWithMatch(ctx)
	return match.Value, match.Detail
}

// EvaluateWithMatch is like Evaluate but also returns the index of the winning
// top-level rule (-1 when the default value is used).
func (def *Definition) EvaluateWithMatch(ctx map[string]any) EvaluationMatch {
	var currentRule rule.ConcreteRule
	currentIndex := -1
	found := false

	currentIsOverride := false
	currentPriority := -1
	for i, r := range def.Rules {
		ruleOverride, rulePriority := r.IsOverride(), r.GetPriority()
		// Skip rules with lower priority than the current rule.
		if rulePriority < currentPriority {
			continue
		}
		// If they are equal, skip the rule if the current rule is an override or the rule is not an override.
		if rulePriority == currentPriority && (currentIsOverride || !ruleOverride) {
			continue
		}
		if r.Matches(ctx) {
			currentIsOverride = ruleOverride
			currentPriority = rulePriority
			currentRule = r
			currentIndex = i
			found = true
		}
	}
	if found {
		return EvaluationMatch{
			Value: currentRule.Value(),
			Detail: openfeature.ProviderResolutionDetail{
				Reason:  openfeature.TargetingMatchReason,
				Variant: currentRule.Variant(),
			},
			MatchedRuleIndex: currentIndex,
		}
	}

	return EvaluationMatch{
		Value: def.DefaultValue,
		Detail: openfeature.ProviderResolutionDetail{
			Reason:  openfeature.DefaultReason,
			Variant: def.DefaultVariant,
		},
		MatchedRuleIndex: -1,
	}
}
