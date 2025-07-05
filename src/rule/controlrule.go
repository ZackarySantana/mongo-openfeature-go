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
	ExactMatchRule *ExactMatchRule `bson:"exactMatchRule,omitempty" json:"exactMatchRule,omitempty"`
	RegexRule      *RegexRule      `bson:"regexRule,omitempty" json:"regexRule,omitempty"`
	ExistsRule     *ExistsRule     `bson:"existsRule,omitempty" json:"existsRule,omitempty"`
	FractionalRule *FractionalRule `bson:"fractionalRule,omitempty" json:"fractionalRule,omitempty"`
	RangeRule      *RangeRule      `bson:"rangeRule,omitempty" json:"rangeRule,omitempty"`
	InListRule     *InListRule     `bson:"inListRule,omitempty" json:"inListRule,omitempty"`
	PrefixRule     *PrefixRule     `bson:"prefixRule,omitempty" json:"prefixRule,omitempty"`
	SuffixRule     *SuffixRule     `bson:"suffixRule,omitempty" json:"suffixRule,omitempty"`
	ContainsRule   *ContainsRule   `bson:"containsRule,omitempty" json:"containsRule,omitempty"`
	IPRangeRule    *IPRangeRule    `bson:"ipRangeRule,omitempty" json:"ipRangeRule,omitempty"`
	GeoFenceRule   *GeoFenceRule   `bson:"geoFenceRule,omitempty" json:"geoFenceRule,omitempty"`
	DateTimeRule   *DateTimeRule   `bson:"dateTimeRule,omitempty" json:"dateTimeRule,omitempty"`
	SemVerRule     *SemVerRule     `bson:"semVerRule,omitempty" json:"semVerRule,omitempty"`
	CronRule       *CronRule       `bson:"cronRule,omitempty" json:"cronRule,omitempty"`

	// Control rules
	AndRule *AndRule `bson:"andRule,omitempty" json:"andRule,omitempty"`
	OrRule  *OrRule  `bson:"orRule,omitempty" json:"orRule,omitempty"`
	NotRule *NotRule `bson:"notRule,omitempty" json:"notRule,omitempty"`
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
	if c.RangeRule != nil {
		return c.RangeRule.Matches(ctx)
	}
	if c.InListRule != nil {
		return c.InListRule.Matches(ctx)
	}
	if c.PrefixRule != nil {
		return c.PrefixRule.Matches(ctx)
	}
	if c.SuffixRule != nil {
		return c.SuffixRule.Matches(ctx)
	}
	if c.ContainsRule != nil {
		return c.ContainsRule.Matches(ctx)
	}
	if c.IPRangeRule != nil {
		return c.IPRangeRule.Matches(ctx)
	}
	if c.GeoFenceRule != nil {
		return c.GeoFenceRule.Matches(ctx)
	}
	if c.DateTimeRule != nil {
		return c.DateTimeRule.Matches(ctx)
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
