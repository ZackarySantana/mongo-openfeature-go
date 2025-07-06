package flag

import (
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/assert"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
)

func TestDefinition(t *testing.T) {
	// A standard context that will cause all test rules to match.
	matchingCtx := map[string]any{"user_id": "test-user-123"}

	// Define reusable rules to keep tests clean.
	standardRule1 := rule.ConcreteRule{ExistsRule: &rule.ExistsRule{
		Key: "user_id", ValueData: "standard_1", VariantID: "v_standard_1", Priority: 10,
	}}
	standardRule2 := rule.ConcreteRule{ExistsRule: &rule.ExistsRule{
		Key: "user_id", ValueData: "standard_2", VariantID: "v_standard_2", Priority: 20,
	}}
	overrideRule1 := rule.ConcreteRule{OverrideRule: &rule.OverrideRule{
		ValueData: "override_1", VariantID: "v_override_1", Priority: 15,
	}}
	overrideRule2 := rule.ConcreteRule{OverrideRule: &rule.OverrideRule{
		ValueData: "override_2", VariantID: "v_override_2", Priority: 15,
	}}
	overrideRule3 := rule.ConcreteRule{OverrideRule: &rule.OverrideRule{
		ValueData: "override_3", VariantID: "v_override_3", Priority: 25,
	}}

	t.Run("FirstMatchingRuleSelectedWhenPrioritiesAreEqual", func(t *testing.T) {
		// 1. No overrides, multiple rules, first one is picked.
		def := &Definition{
			Rules: []rule.ConcreteRule{
				{ExistsRule: &rule.ExistsRule{Key: "user_id", ValueData: "first", VariantID: "v_first", Priority: 5}},
				{ExistsRule: &rule.ExistsRule{Key: "user_id", ValueData: "second", VariantID: "v_second", Priority: 5}},
			},
		}

		val, detail := def.Evaluate(matchingCtx)

		assert.Equal(t, "first", val)
		assert.Equal(t, "v_first", detail.Variant)
		assert.Equal(t, openfeature.TargetingMatchReason, detail.Reason)
	})

	t.Run("OverrideSelectedOverStandardRulesWithEqualPriority", func(t *testing.T) {
		// 2. One override, multiple rules, the override one is picked (it's in the middle).
		def := &Definition{
			Rules: []rule.ConcreteRule{
				{ExistsRule: &rule.ExistsRule{Key: "user_id", ValueData: "standard", VariantID: "v_standard", Priority: 15}},
				overrideRule1, // The override rule to be selected.
				{ExistsRule: &rule.ExistsRule{Key: "user_id", ValueData: "standard_other", VariantID: "v_standard_other", Priority: 15}},
			},
		}

		val, detail := def.Evaluate(matchingCtx)

		assert.Equal(t, "override_1", val)
		assert.Equal(t, "v_override_1", detail.Variant)
	})

	t.Run("FirstOverrideSelectedWhenMultipleOverridesHaveEqualPriority", func(t *testing.T) {
		// 3. Two overrides, multiple other rules, the first override is picked.
		def := &Definition{
			Rules: []rule.ConcreteRule{
				standardRule1,
				overrideRule1, // This is the first override and should be selected.
				overrideRule2, // This override has the same priority but comes later.
			},
		}

		val, detail := def.Evaluate(matchingCtx)

		assert.Equal(t, "override_1", val)
		assert.Equal(t, "v_override_1", detail.Variant)
	})

	t.Run("HighestPriorityRuleSelectedWhenNoOverridesExist", func(t *testing.T) {
		// 4. No overrides, multiple rules with priority, highest priority is picked.
		def := &Definition{
			Rules: []rule.ConcreteRule{
				standardRule1, // Priority 10
				standardRule2, // Priority 20 (should be selected)
				{ExistsRule: &rule.ExistsRule{Key: "user_id", ValueData: "standard_3", VariantID: "v_standard_3", Priority: 5}},
			},
		}

		val, detail := def.Evaluate(matchingCtx)

		assert.Equal(t, "standard_2", val)
		assert.Equal(t, "v_standard_2", detail.Variant)
	})

	t.Run("HigherPriorityStandardRuleSelectedOverLowerPriorityOverride", func(t *testing.T) {
		// 5. One override, multiple rules with one having priority, the priority is picked.
		def := &Definition{
			Rules: []rule.ConcreteRule{
				overrideRule1, // Priority 15
				standardRule2, // Priority 20 (should be selected)
			},
		}

		val, detail := def.Evaluate(matchingCtx)

		assert.Equal(t, "standard_2", val)
		assert.Equal(t, "v_standard_2", detail.Variant)
	})

	t.Run("HighestPriorityOverrideSelected", func(t *testing.T) {
		// 6. Two overrides, multiple rules, override with highest priority is picked.
		def := &Definition{
			Rules: []rule.ConcreteRule{
				standardRule1, // Priority 10
				overrideRule1, // Priority 15
				overrideRule3, // Priority 25 (should be selected)
			},
		}

		val, detail := def.Evaluate(matchingCtx)

		assert.Equal(t, "override_3", val)
		assert.Equal(t, "v_override_3", detail.Variant)
	})
}
