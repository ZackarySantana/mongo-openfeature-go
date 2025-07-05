package rules

import (
	"fmt"
	"hash/fnv"
	"log/slog"
	"reflect"
	"regexp"
	"strings"

	"github.com/open-feature/go-sdk/openfeature"
)

type Rule interface {
	Matches(ctx map[string]any) bool
	Value() any
	Variant() string
}

// ExactMatchRule fires if ctx[Key] deep‐equals ValueData.
type ExactMatchRule struct {
	Key      string
	KeyValue string

	VariantID string
	ValueData any
}

func (r *ExactMatchRule) Matches(ctx map[string]any) bool {
	v, ok := ctx[r.Key]
	return ok && (v == r.KeyValue || reflect.DeepEqual(v, r.KeyValue))
}

func (r *ExactMatchRule) Value() any      { return r.ValueData }
func (r *ExactMatchRule) Variant() string { return r.VariantID }

// RegexRule fires if ctx[Key] (string) matches Pattern.
type RegexRule struct {
	Key           string
	RegexpPattern string
	Regexp        *regexp.Regexp `json:"-" bson:"-"`
	VariantID     string
	ValueData     any
}

func (r *RegexRule) Matches(ctx map[string]any) bool {
	v, ok := ctx[r.Key]
	if !ok {
		return false
	}
	if r.Regexp == nil {
		var err error
		r.Regexp, err = regexp.Compile(r.RegexpPattern)
		if err != nil {
			slog.Error("invalid regex pattern", "key", r.Key, "pattern", r.RegexpPattern, "error", err)
			return false
		}
	}
	s, ok := v.(string)
	return ok && r.Regexp.MatchString(s)
}

func (r *RegexRule) Value() any      { return r.ValueData }
func (r *RegexRule) Variant() string { return r.VariantID }

// ExistsRule fires if ctx contains Key at all.
type ExistsRule struct {
	Key       string
	VariantID string
	ValueData any
}

func (r *ExistsRule) Matches(ctx map[string]any) bool {
	_, ok := ctx[r.Key]
	return ok
}

func (r *ExistsRule) Value() any      { return r.ValueData }
func (r *ExistsRule) Variant() string { return r.VariantID }

// FractionalRule fires a percentage of the time (deterministic via FNV+salt).
type FractionalRule struct {
	Key        string
	Percentage float64 // in [0.0,100.0)
	VariantID  string
	ValueData  any
}

func (r *FractionalRule) Matches(ctx map[string]any) bool {
	raw, ok := ctx[r.Key]
	if !ok {
		return false
	}
	h := fnv.New32a()
	fmt.Fprint(h, r.Key, raw)
	bucket := h.Sum32() % 100
	return float64(bucket) < r.Percentage
}

func (r *FractionalRule) Value() any      { return r.ValueData }
func (r *FractionalRule) Variant() string { return r.VariantID }

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
	AndRule        *AndRule
	OrRule         *OrRule
	NotRule        *NotRule
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

// -----------------------------------------------------------------------------
// FlagDefinition (non‐generic) with its own Evaluate
// -----------------------------------------------------------------------------

type FlagDefinition struct {
	FlagName string

	DefaultValue   any
	DefaultVariant string

	Rules []ConcreteRule
}

// Evaluate walks the Rules in order, returns the first match’s (value,detail),
// or the default if none match.
func (def *FlagDefinition) Evaluate(ctx map[string]any) (any, openfeature.ProviderResolutionDetail) {
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
