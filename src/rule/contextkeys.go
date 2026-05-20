package rule

import (
	"fmt"
	"sort"
)

// ContextKeyRef identifies a saved rule that reads a context key. TopLevelIndex
// is the 0-based index in the flag's top-level rules slice (used for scroll
// links in the editor).
type ContextKeyRef struct {
	TopLevelIndex int
	Label         string
}

// ContextKeyField groups a context key with the rules that reference it.
type ContextKeyField struct {
	Key   string
	Rules []ContextKeyRef
}

// CollectContextKeyFields returns context keys referenced by the given rules,
// each with the list of rules that read that key.
func CollectContextKeyFields(rules []ConcreteRule) []ContextKeyField {
	byKey := make(map[string][]ContextKeyRef)

	for i, cr := range rules {
		walkRule(cr, i, "", byKey)
	}

	if len(byKey) == 0 {
		return nil
	}

	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fields := make([]ContextKeyField, len(keys))
	for i, k := range keys {
		fields[i] = ContextKeyField{
			Key:   k,
			Rules: dedupeRefs(byKey[k]),
		}
	}
	return fields
}

// CollectContextKeys returns the sorted, deduplicated context keys referenced
// by the given rules (including nested composite rules).
func CollectContextKeys(rules []ConcreteRule) []string {
	fields := CollectContextKeyFields(rules)
	if len(fields) == 0 {
		return nil
	}
	keys := make([]string, len(fields))
	for i, f := range fields {
		keys[i] = f.Key
	}
	return keys
}

func walkRule(cr ConcreteRule, topLevel int, nestedIn string, byKey map[string][]ContextKeyRef) {
	ruleType := cr.RuleType()

	for _, k := range directContextKeys(cr) {
		ref := ContextKeyRef{
			TopLevelIndex: topLevel,
			Label:         formatContextKeyRefLabel(topLevel, ruleType, nestedIn),
		}
		byKey[k] = appendRefIfNew(byKey[k], ref)
	}

	switch {
	case cr.AndRule != nil:
		for _, child := range cr.AndRule.Rules {
			walkRule(child, topLevel, ruleType, byKey)
		}
	case cr.OrRule != nil:
		for _, child := range cr.OrRule.Rules {
			walkRule(child, topLevel, ruleType, byKey)
		}
	case cr.NotRule != nil:
		walkRule(cr.NotRule.Rule, topLevel, ruleType, byKey)
	}
}

func formatContextKeyRefLabel(topLevel int, ruleType, nestedIn string) string {
	if nestedIn != "" {
		return fmt.Sprintf("#%d %s · %s", topLevel+1, nestedIn, ruleType)
	}
	return fmt.Sprintf("#%d %s", topLevel+1, ruleType)
}

func appendRefIfNew(refs []ContextKeyRef, ref ContextKeyRef) []ContextKeyRef {
	for _, existing := range refs {
		if existing.TopLevelIndex == ref.TopLevelIndex && existing.Label == ref.Label {
			return refs
		}
	}
	return append(refs, ref)
}

func dedupeRefs(refs []ContextKeyRef) []ContextKeyRef {
	if len(refs) <= 1 {
		return refs
	}
	out := make([]ContextKeyRef, 0, len(refs))
	for _, ref := range refs {
		out = appendRefIfNew(out, ref)
	}
	return out
}

// directContextKeys returns context keys read directly by this rule (not via
// composite children).
func directContextKeys(cr ConcreteRule) []string {
	switch {
	case cr.ExactMatchRule != nil:
		return []string{cr.ExactMatchRule.Key}
	case cr.RegexRule != nil:
		return []string{cr.RegexRule.Key}
	case cr.ExistsRule != nil:
		return []string{cr.ExistsRule.Key}
	case cr.FractionalRule != nil:
		return []string{cr.FractionalRule.Key}
	case cr.RangeRule != nil:
		return []string{cr.RangeRule.Key}
	case cr.InListRule != nil:
		return []string{cr.InListRule.Key}
	case cr.PrefixRule != nil:
		return []string{cr.PrefixRule.Key}
	case cr.SuffixRule != nil:
		return []string{cr.SuffixRule.Key}
	case cr.ContainsRule != nil:
		return []string{cr.ContainsRule.Key}
	case cr.IPRangeRule != nil:
		return []string{cr.IPRangeRule.Key}
	case cr.GeoFenceRule != nil:
		return []string{cr.GeoFenceRule.LatKey, cr.GeoFenceRule.LngKey}
	case cr.DateTimeRule != nil:
		return []string{cr.DateTimeRule.Key}
	case cr.SemVerRule != nil:
		return []string{cr.SemVerRule.Key}
	case cr.CronRule != nil:
		if cr.CronRule.Key != "" {
			return []string{cr.CronRule.Key}
		}
		return nil
	default:
		return nil
	}
}
