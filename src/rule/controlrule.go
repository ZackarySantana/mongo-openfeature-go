package rule

import (
	"fmt"
	"strings"
)

// AndRule matches if ALL children match; Variant = "&(v1+v2+…)".
type AndRule struct {
	Rules     []ConcreteRule
	ValueData any
}

func (r *AndRule) Matches(ctx map[string]any) bool {
	for _, c := range r.Rules {
		if !c.Matches(ctx) {
			return false
		}
	}
	return true
}

func (r *AndRule) Value() any { return r.ValueData }

func (r *AndRule) Variant() string {
	parts := make([]string, len(r.Rules))
	for i, c := range r.Rules {
		parts[i] = c.Variant()
	}
	return fmt.Sprintf("&(%s)", strings.Join(parts, "+"))
}

// OrRule matches if ANY child matches; Variant = "|(v1+v2+…)".
type OrRule struct {
	Rules     []ConcreteRule
	ValueData any
}

func (r *OrRule) Matches(ctx map[string]any) bool {
	for _, c := range r.Rules {
		if c.Matches(ctx) {
			return true
		}
	}
	return false
}

func (r *OrRule) Value() any { return r.ValueData }

func (r *OrRule) Variant() string {
	parts := make([]string, len(r.Rules))
	for i, c := range r.Rules {
		parts[i] = c.Variant()
	}
	return fmt.Sprintf("|(%s)", strings.Join(parts, "+"))
}

// NotRule inverts a single child; Variant = "!(v)".
type NotRule struct {
	Rule      ConcreteRule
	ValueData any
}

func (r *NotRule) Matches(ctx map[string]any) bool {
	return !r.Rule.Matches(ctx)
}

func (r *NotRule) Value() any      { return r.ValueData }
func (r *NotRule) Variant() string { return "!(" + r.Rule.Variant() + ")" }

type ConcreteRule struct {
	ExactMatchRule *ExactMatchRule
	RegexRule      *RegexRule
	ExistsRule     *ExistsRule
	FractionalRule *FractionalRule
	RangeRule      *RangeRule
	InListRule     *InListRule
	PrefixRule     *PrefixRule
	SuffixRule     *SuffixRule
	ContainsRule   *ContainsRule
	IPRangeRule    *IPRangeRule
	GeoFenceRule   *GeoFenceRule
	DateTimeRule   *DateTimeRule

	// Control rules
	AndRule *AndRule
	OrRule  *OrRule
	NotRule *NotRule
}

func (c *ConcreteRule) Matches(ctx map[string]any) bool {
	if c.ExactMatchRule != nil {
		return c.ExactMatchRule.Matches(ctx)
	}
	if c.RegexRule != nil {
		return c.RegexRule.Matches(ctx)
	}
	if c.ExistsRule != nil {
		return c.ExistsRule.Matches(ctx)
	}
	if c.FractionalRule != nil {
		return c.FractionalRule.Matches(ctx)
	}
	if c.AndRule != nil {
		return c.AndRule.Matches(ctx)
	}
	if c.OrRule != nil {
		return c.OrRule.Matches(ctx)
	}
	if c.NotRule != nil {
		return c.NotRule.Matches(ctx)
	}
	return false
}

func (c *ConcreteRule) Value() any {
	if c.ExactMatchRule != nil {
		return c.ExactMatchRule.Value()
	}
	if c.RegexRule != nil {
		return c.RegexRule.Value()
	}
	if c.ExistsRule != nil {
		return c.ExistsRule.Value()
	}
	if c.FractionalRule != nil {
		return c.FractionalRule.Value()
	}
	if c.AndRule != nil {
		return c.AndRule.Value()
	}
	if c.OrRule != nil {
		return c.OrRule.Value()
	}
	if c.NotRule != nil {
		return c.NotRule.Value()
	}
	return nil
}

func (c *ConcreteRule) Variant() string {
	if c.ExactMatchRule != nil {
		return c.ExactMatchRule.Variant()
	}
	if c.RegexRule != nil {
		return c.RegexRule.Variant()
	}
	if c.ExistsRule != nil {
		return c.ExistsRule.Variant()
	}
	if c.FractionalRule != nil {
		return c.FractionalRule.Variant()
	}
	if c.AndRule != nil {
		return c.AndRule.Variant()
	}
	if c.OrRule != nil {
		return c.OrRule.Variant()
	}
	if c.NotRule != nil {
		return c.NotRule.Variant()
	}
	return ""
}
