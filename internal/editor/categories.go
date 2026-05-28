package editor

import (
	"sort"
	"strings"

	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
)

// flagListSection groups flags for display on the home page.
type flagListSection struct {
	Name  string
	Flags []flag.Definition
}

// normalizeFlagName ensures FlagName is set when flags come from a map key.
func normalizeFlagName(name string, def flag.Definition) flag.Definition {
	if def.FlagName == "" {
		def.FlagName = name
	}
	return def
}

// CollectCategories returns sorted, unique non-empty category names across flags.
func CollectCategories(flags map[string]flag.Definition) []string {
	seen := make(map[string]struct{})
	for name, def := range flags {
		def = normalizeFlagName(name, def)
		cat := strings.TrimSpace(def.Category)
		if cat == "" {
			continue
		}
		seen[cat] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for cat := range seen {
		out = append(out, cat)
	}
	sort.Strings(out)
	return out
}

// BuildFlagListSections groups flags for the home page: uncategorized first,
// then each category in alphabetical order. Flags within a section are sorted
// by name.
func BuildFlagListSections(flags map[string]flag.Definition) []flagListSection {
	if len(flags) == 0 {
		return nil
	}

	byCategory := make(map[string][]flag.Definition)
	var uncategorized []flag.Definition

	for name, def := range flags {
		def = normalizeFlagName(name, def)
		cat := strings.TrimSpace(def.Category)
		if cat == "" {
			uncategorized = append(uncategorized, def)
		} else {
			byCategory[cat] = append(byCategory[cat], def)
		}
	}

	sortFlags := func(list []flag.Definition) {
		sort.Slice(list, func(i, j int) bool {
			return list[i].FlagName < list[j].FlagName
		})
	}

	var sections []flagListSection
	if len(uncategorized) > 0 {
		sortFlags(uncategorized)
		sections = append(sections, flagListSection{Flags: uncategorized})
	}

	categories := make([]string, 0, len(byCategory))
	for cat := range byCategory {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	for _, cat := range categories {
		group := byCategory[cat]
		sortFlags(group)
		sections = append(sections, flagListSection{Name: cat, Flags: group})
	}

	return sections
}
