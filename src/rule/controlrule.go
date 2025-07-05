package rule

import (
	"fmt"
	"strings"
)

// AndRule matches if ALL children match; Variant = "&(v1+v2+…)".
type AndRule struct {
	Rules []ConcreteRule

	Priority  int
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

func (r *AndRule) GetPriority() int { return r.Priority }

// OrRule matches if ANY child matches; Variant = "|(v1+v2+…)".
type OrRule struct {
	Rules []ConcreteRule

	Priority  int
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

func (r *OrRule) GetPriority() int { return r.Priority }

// NotRule inverts a single child; Variant = "!(v)".
type NotRule struct {
	Rule ConcreteRule

	Priority  int
	ValueData any
}

func (r *NotRule) Matches(ctx map[string]any) bool {
	return !r.Rule.Matches(ctx)
}

func (r *NotRule) Value() any       { return r.ValueData }
func (r *NotRule) Variant() string  { return "!(" + r.Rule.Variant() + ")" }
func (r *NotRule) GetPriority() int { return r.Priority }

type OverrideRule struct {
	ValueData any

	Priority  int
	VariantID string
}

func (r *OverrideRule) Matches(ctx map[string]any) bool {
	return true // Always matches, as it's an override
}

func (r *OverrideRule) Value() any      { return r.ValueData }
func (r *OverrideRule) Variant() string { return r.VariantID }

func (r *OverrideRule) GetPriority() int { return r.Priority }
