package rule

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConcreteRule_Dispatch(t *testing.T) {
	ctx := map[string]any{
		"test_key":  "test_value",
		"other_key": "other_value",
	}

	testCases := []struct {
		name             string
		rule             *ConcreteRule
		expectedValue    any
		expectedVariant  string
		expectedPriority int
		expectedMatch    bool
	}{
		{
			name: "DispatchToExactMatchRule",
			rule: &ConcreteRule{
				ExactMatchRule: &ExactMatchRule{
					Key:       "test_key",
					KeyValue:  "test_value",
					VariantID: "variant_exact",
					ValueData: "data_exact",
					Priority:  10,
				},
			},
			expectedValue:    "data_exact",
			expectedVariant:  "variant_exact",
			expectedPriority: 10,
			expectedMatch:    true,
		},
		{
			name: "DispatchToAndRule",
			rule: &ConcreteRule{
				AndRule: &AndRule{
					Rules: []ConcreteRule{
						{ExistsRule: &ExistsRule{Key: "test_key", VariantID: "v1"}},
						{ExistsRule: &ExistsRule{Key: "other_key", VariantID: "v2"}},
					},
					ValueData: "data_and",
					Priority:  20,
				},
			},
			expectedValue:    "data_and",
			expectedVariant:  "&(v1+v2)",
			expectedPriority: 20,
			expectedMatch:    true,
		},
		{
			name: "DispatchToOverrideRule",
			rule: &ConcreteRule{
				OverrideRule: &OverrideRule{
					ValueData: "data_override",
					VariantID: "variant_override",
				},
			},
			expectedValue:   "data_override",
			expectedVariant: "variant_override",
			expectedMatch:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Assert that Unwrap returns a non-nil inner rule.
			assert.NotNil(t, tc.rule.Unwrap())

			// Assert that all methods on the wrapper produce the expected result.
			assert.Equal(t, tc.expectedMatch, tc.rule.Matches(ctx))
			assert.Equal(t, tc.expectedValue, tc.rule.Value())
			assert.Equal(t, tc.expectedVariant, tc.rule.Variant())
			assert.Equal(t, tc.expectedPriority, tc.rule.GetPriority())
		})
	}
}

func TestConcreteRule_Empty(t *testing.T) {
	emptyRule := &ConcreteRule{}
	ctx := map[string]any{"any_key": "any_value"}

	assert.False(t, emptyRule.Matches(ctx), "Empty rule should not match")
	assert.Nil(t, emptyRule.Value(), "Empty rule should have a nil value")
	assert.Equal(t, "", emptyRule.Variant(), "Empty rule should have an empty variant")
	assert.Equal(t, 0, emptyRule.GetPriority(), "Empty rule should have zero priority")
	assert.False(t, emptyRule.IsOverride(), "Empty rule should not be an override")
	assert.Nil(t, emptyRule.Unwrap(), "Unwrap on an empty rule should return nil")
}

func TestConcreteRule_IsOverride(t *testing.T) {
	testCases := []struct {
		name     string
		rule     *ConcreteRule
		expected bool
	}{
		{
			name:     "IsOverrideReturnsTrueForOverrideRule",
			rule:     &ConcreteRule{OverrideRule: &OverrideRule{}},
			expected: true,
		},
		{
			name:     "IsOverrideReturnsFalseForStandardRule",
			rule:     &ConcreteRule{ExistsRule: &ExistsRule{Key: "some_key"}},
			expected: false,
		},
		{
			name:     "IsOverrideReturnsFalseForControlRule",
			rule:     &ConcreteRule{AndRule: &AndRule{}},
			expected: false,
		},
		{
			name:     "IsOverrideReturnsFalseForEmptyRule",
			rule:     &ConcreteRule{},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.rule.IsOverride())
		})
	}
}
