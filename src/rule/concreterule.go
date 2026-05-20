package rule

type Rule interface {
	// Matches checks if the rule matches the provided context.
	Matches(ctx map[string]any) bool
	// Value returns the value associated with the rule.
	Value() any
	// Variant returns the variant identifier for the rule.
	Variant() string
	// GetPriority returns the priority of the rule.
	GetPriority() int
}

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
	AndRule      *AndRule      `bson:"andRule,omitempty" json:"andRule,omitempty"`
	OrRule       *OrRule       `bson:"orRule,omitempty" json:"orRule,omitempty"`
	NotRule      *NotRule      `bson:"notRule,omitempty" json:"notRule,omitempty"`
	OverrideRule *OverrideRule `bson:"overrideRule,omitempty" json:"overrideRule,omitempty"`
}

func (c *ConcreteRule) Unwrap() Rule {
	if c.ExactMatchRule != nil {
		return c.ExactMatchRule
	}
	if c.RegexRule != nil {
		return c.RegexRule
	}
	if c.ExistsRule != nil {
		return c.ExistsRule
	}
	if c.FractionalRule != nil {
		return c.FractionalRule
	}
	if c.RangeRule != nil {
		return c.RangeRule
	}
	if c.InListRule != nil {
		return c.InListRule
	}
	if c.PrefixRule != nil {
		return c.PrefixRule
	}
	if c.SuffixRule != nil {
		return c.SuffixRule
	}
	if c.ContainsRule != nil {
		return c.ContainsRule
	}
	if c.IPRangeRule != nil {
		return c.IPRangeRule
	}
	if c.GeoFenceRule != nil {
		return c.GeoFenceRule
	}
	if c.DateTimeRule != nil {
		return c.DateTimeRule
	}
	if c.SemVerRule != nil {
		return c.SemVerRule
	}
	if c.CronRule != nil {
		return c.CronRule
	}
	if c.AndRule != nil {
		return c.AndRule
	}
	if c.OrRule != nil {
		return c.OrRule
	}
	if c.NotRule != nil {
		return c.NotRule
	}
	if c.OverrideRule != nil {
		return c.OverrideRule
	}
	return nil
}

func (c *ConcreteRule) Matches(ctx map[string]any) bool {
	rule := c.Unwrap()
	if rule == nil {
		return false
	}
	return rule.Matches(ctx)
}

func (c *ConcreteRule) Value() any {
	rule := c.Unwrap()
	if rule == nil {
		return nil
	}
	return rule.Value()
}

func (c *ConcreteRule) Variant() string {
	rule := c.Unwrap()
	if rule == nil {
		return ""
	}
	return rule.Variant()
}

func (c *ConcreteRule) GetPriority() int {
	rule := c.Unwrap()
	if rule == nil {
		return 0
	}
	return rule.GetPriority()
}

func (c *ConcreteRule) IsOverride() bool {
	return c.OverrideRule != nil
}

// RuleType returns the JSON/BSON key for the active rule variant (e.g.
// "exactMatchRule"), or an empty string when the concrete rule is empty.
func (c *ConcreteRule) RuleType() string {
	switch {
	case c.ExactMatchRule != nil:
		return "exactMatchRule"
	case c.RegexRule != nil:
		return "regexRule"
	case c.ExistsRule != nil:
		return "existsRule"
	case c.FractionalRule != nil:
		return "fractionalRule"
	case c.RangeRule != nil:
		return "rangeRule"
	case c.InListRule != nil:
		return "inListRule"
	case c.PrefixRule != nil:
		return "prefixRule"
	case c.SuffixRule != nil:
		return "suffixRule"
	case c.ContainsRule != nil:
		return "containsRule"
	case c.IPRangeRule != nil:
		return "ipRangeRule"
	case c.GeoFenceRule != nil:
		return "geoFenceRule"
	case c.DateTimeRule != nil:
		return "dateTimeRule"
	case c.SemVerRule != nil:
		return "semVerRule"
	case c.CronRule != nil:
		return "cronRule"
	case c.AndRule != nil:
		return "andRule"
	case c.OrRule != nil:
		return "orRule"
	case c.NotRule != nil:
		return "notRule"
	case c.OverrideRule != nil:
		return "overrideRule"
	default:
		return ""
	}
}
