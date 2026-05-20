package rule

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectContextKeys(t *testing.T) {
	rules := []ConcreteRule{
		{ExactMatchRule: &ExactMatchRule{Key: "user_id"}},
		{ExistsRule: &ExistsRule{Key: "region"}},
		{AndRule: &AndRule{Rules: []ConcreteRule{
			{PrefixRule: &PrefixRule{Key: "user_id"}},
			{GeoFenceRule: &GeoFenceRule{LatKey: "lat", LngKey: "lng"}},
		}}},
		{OverrideRule: &OverrideRule{VariantID: "always"}},
		{CronRule: &CronRule{CronSpec: "0 9 * * *"}}, // empty Key → no context key
	}

	keys := CollectContextKeys(rules)

	assert.Equal(t, []string{"lat", "lng", "region", "user_id"}, keys)
}

func TestCollectContextKeysEmpty(t *testing.T) {
	assert.Nil(t, CollectContextKeys(nil))
	assert.Nil(t, CollectContextKeys([]ConcreteRule{
		{OverrideRule: &OverrideRule{}},
	}))
}

func TestCollectContextKeyFieldsRuleRefs(t *testing.T) {
	rules := []ConcreteRule{
		{ExactMatchRule: &ExactMatchRule{Key: "user_id"}},
		{AndRule: &AndRule{Rules: []ConcreteRule{
			{ExistsRule: &ExistsRule{Key: "user_id"}},
			{PrefixRule: &PrefixRule{Key: "region"}},
		}}},
	}

	fields := CollectContextKeyFields(rules)

	assert.Len(t, fields, 2)

	byKey := make(map[string]ContextKeyField, len(fields))
	for _, f := range fields {
		byKey[f.Key] = f
	}

	assert.Equal(t, []ContextKeyRef{
		{TopLevelIndex: 0, Label: "#1 exactMatchRule"},
		{TopLevelIndex: 1, Label: "#2 andRule · existsRule"},
	}, byKey["user_id"].Rules)

	assert.Equal(t, []ContextKeyRef{
		{TopLevelIndex: 1, Label: "#2 andRule · prefixRule"},
	}, byKey["region"].Rules)
}
