package rule

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAndRule(t *testing.T) {
	for tName, tCase := range map[string]struct {
		ctx     map[string]any
		matches bool
	}{
		"AllMatch": {
			ctx: map[string]any{
				"test_key1": "value1",
				"test_key2": "value2",
			},
			matches: true,
		},
		"OneNotMatch": {
			ctx: map[string]any{
				"test_key1": "value1",
				"test_key2": "other_value",
			},
			matches: false,
		},
		"NoneMatch": {
			ctx:     map[string]any{},
			matches: false,
		},
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &AndRule{
				Rules: []ConcreteRule{
					{ContainsRule: &ContainsRule{
						Key:       "test_key1",
						Substring: "value1",
						VariantID: "variant1",
						ValueData: "data1",
					}},
					{ContainsRule: &ContainsRule{
						Key:       "test_key2",
						Substring: "value2",
						VariantID: "variant2",
						ValueData: "data2",
					}},
				},
				ValueData: "and_value_data",
			}
			assert.Equal(t, tCase.matches, rule.Matches(tCase.ctx))
		})
	}
}

func TestOrRule(t *testing.T) {
	for tName, tCase := range map[string]struct {
		ctx     map[string]any
		matches bool
	}{
		"OneMatch": {
			ctx: map[string]any{
				"test_key1": "value1",
			},
			matches: true,
		},
		"AllMatch": {
			ctx: map[string]any{
				"test_key1": "value1",
				"test_key2": "value2",
			},
			matches: true,
		},
		"NoneMatch": {
			ctx:     map[string]any{},
			matches: false,
		},
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &OrRule{
				Rules: []ConcreteRule{
					{ContainsRule: &ContainsRule{
						Key:       "test_key1",
						Substring: "value1",
						VariantID: "variant1",
						ValueData: "data1",
					}},
					{ContainsRule: &ContainsRule{
						Key:       "test_key2",
						Substring: "value2",
						VariantID: "variant2",
						ValueData: "data2",
					}},
				},
				ValueData: "or_value_data",
			}
			assert.Equal(t, tCase.matches, rule.Matches(tCase.ctx))
		})
	}
}

func TestNotRule(t *testing.T) {
	for tName, tCase := range map[string]struct {
		ctx     map[string]any
		matches bool
	}{
		"Exact": {
			ctx: map[string]any{
				"test_key": "value",
			},
			matches: false,
		},
		"Contains": {
			ctx: map[string]any{
				"test_key": "other_value",
			},
			matches: false,
		},
		"DoesNotContain": {
			ctx: map[string]any{
				"test_key": "not_gonna_say_it",
			},
			matches: true,
		},
		"Empty": {
			ctx:     map[string]any{},
			matches: true,
		},
	} {
		t.Run(tName, func(t *testing.T) {
			rule := &NotRule{
				Rule: ConcreteRule{
					ContainsRule: &ContainsRule{
						Key:       "test_key",
						Substring: "value",
						VariantID: "variant1",
						ValueData: "data1",
					},
				},
				ValueData: "not_value_data",
			}
			assert.Equal(t, tCase.matches, rule.Matches(tCase.ctx))
		})
	}
}
