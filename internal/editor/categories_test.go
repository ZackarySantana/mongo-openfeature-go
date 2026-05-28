package editor

import (
	"testing"

	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
)

func TestCollectCategories(t *testing.T) {
	flags := map[string]flag.Definition{
		"a": {Category: "Beta"},
		"b": {Category: "Alpha"},
		"c": {Category: "Beta"},
		"d": {},
	}
	got := CollectCategories(flags)
	want := []string{"Alpha", "Beta"}
	if len(got) != len(want) {
		t.Fatalf("CollectCategories() = %v, want %v", got, want)
	}
	for i, cat := range want {
		if got[i] != cat {
			t.Fatalf("CollectCategories()[%d] = %q, want %q", i, got[i], cat)
		}
	}
}

func TestBuildFlagListSections(t *testing.T) {
	flags := map[string]flag.Definition{
		"z-flag": {Category: "Zebra"},
		"a-flag": {},
		"m-flag": {Category: "Alpha"},
		"b-flag": {Category: "Alpha"},
	}

	sections := BuildFlagListSections(flags)
	if len(sections) != 3 {
		t.Fatalf("len(sections) = %d, want 3", len(sections))
	}
	if sections[0].Name != "" {
		t.Fatalf("first section should be uncategorized, got name %q", sections[0].Name)
	}
	if len(sections[0].Flags) != 1 || sections[0].Flags[0].FlagName != "a-flag" {
		t.Fatalf("uncategorized section = %+v", sections[0].Flags)
	}
	if sections[1].Name != "Alpha" {
		t.Fatalf("second section name = %q, want Alpha", sections[1].Name)
	}
	if len(sections[1].Flags) != 2 {
		t.Fatalf("Alpha section len = %d, want 2", len(sections[1].Flags))
	}
	if sections[2].Name != "Zebra" {
		t.Fatalf("third section name = %q, want Zebra", sections[2].Name)
	}

	onlyCategorized := map[string]flag.Definition{
		"x": {Category: "Zebra"},
		"y": {Category: "Alpha"},
	}
	sections = BuildFlagListSections(onlyCategorized)
	if len(sections) != 2 || sections[0].Name != "Alpha" || sections[1].Name != "Zebra" {
		t.Fatalf("categorized-only sections = %+v", sections)
	}
}
