package editor

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
)

func init() {
	// Templates are loaded with paths relative to the module root
	// (e.g. "internal/editor/layout.tmpl"). Tests run with cwd = this package,
	// so hop up two levels once at startup.
	mustChdirToRoot()
}

// TestNewWebHandlerParsesTemplates makes sure every template the handler uses
// parses cleanly. Catches template breakage at test time rather than at
// runtime when a request first arrives.
func TestNewWebHandlerParsesTemplates(t *testing.T) {
	h := NewWebHandler(nil) // client unused for template parsing.
	if h.templates["index"] == nil || h.templates["edit"] == nil {
		t.Fatalf("expected index and edit templates to be parsed")
	}
	if h.toast == nil || h.testResult == nil {
		t.Fatalf("expected toast and testResult partials to be parsed")
	}
}

// TestTestResultPartial renders the inline tester output partial against the
// four states it has to handle: empty (placeholder), matched rule, default
// value, and error. All four must produce the same 4-row structure so the
// layout never shifts when a real result lands.
func TestTestResultPartial(t *testing.T) {
	h := NewWebHandler(nil)

	cases := []struct {
		name string
		data testResultData
		want []string
		// notWant is checked to be ABSENT from the rendered output.
		notWant []string
	}{
		{
			name:    "empty placeholder",
			data:    testResultData{},
			want:    []string{"test-out--empty", "Not evaluated yet", "test-out__pre--placeholder", "test-out__rule-text"},
			notWant: []string{"test-out--matched", "test-out--default", "test-out--error", "data-rule-link"},
		},
		{
			name: "matched",
			data: testResultData{
				Matched:          true,
				Variant:          "v1",
				Reason:           "TARGETING_MATCH",
				ValueJSON:        `"on"`,
				MatchedRuleIndex: 1,
				MatchedRuleType:  "exactMatchRule",
				MatchedRuleLabel: "#2 exactMatchRule · v1",
			},
			want: []string{
				"test-out--matched",
				"v1",
				"TARGETING_MATCH",
				"data-rule-link",
				`data-rule-index="1"`,
				"#2 exactMatchRule · v1",
			},
		},
		{
			name:    "default",
			data:    testResultData{Matched: false, Variant: "d", Reason: "DEFAULT", ValueJSON: `"off"`},
			want:    []string{"test-out--default", "DEFAULT", "Default"},
			notWant: []string{"data-rule-link"},
		},
		{
			name: "error",
			data: testResultData{Error: "bad context"},
			want: []string{"test-out--error", "bad context", "Error"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := h.testResult.ExecuteTemplate(&buf, "test-result", tc.data); err != nil {
				t.Fatalf("rendering partial: %v", err)
			}
			got := buf.String()
			for _, fragment := range tc.want {
				if !strings.Contains(got, fragment) {
					t.Errorf("expected partial to contain %q; got:\n%s", fragment, got)
				}
			}
			for _, fragment := range tc.notWant {
				if strings.Contains(got, fragment) {
					t.Errorf("did not expect partial to contain %q; got:\n%s", fragment, got)
				}
			}
			// All states should produce the same row layout markers so the
			// htmx swap doesn't shift the page.
			for _, marker := range []string{"Variant", "Reason", "Rule", "Value"} {
				if !strings.Contains(got, marker) {
					t.Errorf("expected partial to always include %q row; got:\n%s", marker, got)
				}
			}
		})
	}
}

// TestEditPageRendersTesterForSavedFlag verifies the inline tester only shows
// up for flags that have been saved (i.e. have a name), prefills context keys
// from the saved rules, and renders the empty result placeholder.
func TestEditPageRendersTesterForSavedFlag(t *testing.T) {
	h := NewWebHandler(nil)

	render := func(name string, fields []rule.ContextKeyField) string {
		var buf bytes.Buffer
		data := map[string]any{
			"Flag":                 struct{ FlagName, DefaultVariant string }{name, ""},
			"RulesJSON":            "[]",
			"DefaultValueJSON":     `""`,
			"ContextKeyFields":     fields,
			"ContextKeyFieldsJSON": `[]`,
			"TestResult":           testResultData{},
		}
		if err := h.templates["edit"].ExecuteTemplate(&buf, "layout", data); err != nil {
			t.Fatalf("rendering edit page: %v", err)
		}
		return buf.String()
	}

	saved := render("my-flag", []rule.ContextKeyField{
		{
			Key: "user_id",
			Rules: []rule.ContextKeyRef{
				{TopLevelIndex: 0, Label: "#1 exactMatchRule"},
				{TopLevelIndex: 1, Label: "#2 andRule · existsRule"},
			},
		},
		{
			Key: "region",
			Rules: []rule.ContextKeyRef{
				{TopLevelIndex: 2, Label: "#3 prefixRule"},
			},
		},
	})
	if !strings.Contains(saved, "tester-card") {
		t.Errorf("expected saved flag edit page to include the tester card")
	}
	if !strings.Contains(saved, `data-context-key="user_id"`) {
		t.Errorf("expected prefilled context key user_id")
	}
	if !strings.Contains(saved, "#1 exactMatchRule") {
		t.Errorf("expected rule ref hint under context key")
	}
	if !strings.Contains(saved, `data-rule-index="0"`) {
		t.Errorf("expected scroll link on rule ref")
	}
	if !strings.Contains(saved, "Leave empty to omit") {
		t.Errorf("expected omit placeholder on value inputs")
	}
	if !strings.Contains(saved, "Only fields you fill in are sent") {
		t.Errorf("expected context behavior description")
	}
	if !strings.Contains(saved, `data-tester-source`) {
		t.Errorf("expected saved/draft source toggle")
	}
	if !strings.Contains(saved, "Uses the version stored on the server") {
		t.Errorf("expected saved source subtitle")
	}
	if !strings.Contains(saved, "Draft") {
		t.Errorf("expected draft source option in toggle")
	}
	if strings.Contains(saved, "data-tester-add") {
		t.Errorf("did not expect add-field control")
	}
	if !strings.Contains(saved, "Not evaluated yet") {
		t.Errorf("expected the result region to render the empty placeholder")
	}
	if !strings.Contains(saved, `id="test-output"`) {
		t.Errorf("expected the htmx target #test-output to be present")
	}
	if strings.Contains(render("", nil), "tester-card") {
		t.Errorf("did not expect the tester card on the new-flag page")
	}

	noKeys := render("bare-flag", nil)
	if !strings.Contains(noKeys, "No context keys in the saved rules") {
		t.Errorf("expected empty-state copy when flag has no context keys")
	}
}

// TestConvertTimestamps verifies the helper that upgrades RFC3339 strings to
// time.Time so date/cron rules behave like real Go callers.
func TestConvertTimestamps(t *testing.T) {
	ctx := map[string]any{
		"now":    "2026-05-19T20:00:00Z",
		"name":   "alice",
		"nested": map[string]any{"then": "2025-01-01T00:00:00Z"},
		"list":   []any{"2024-06-01T00:00:00Z", "not-a-date"},
	}
	convertTimestamps(ctx)

	if _, ok := ctx["now"].(time.Time); !ok {
		t.Fatalf("expected ctx[now] to be time.Time, got %T", ctx["now"])
	}
	if ctx["name"] != "alice" {
		t.Fatalf("expected ctx[name] to stay 'alice', got %v", ctx["name"])
	}
	nested := ctx["nested"].(map[string]any)
	if _, ok := nested["then"].(time.Time); !ok {
		t.Fatalf("expected ctx[nested.then] to be time.Time, got %T", nested["then"])
	}
	list := ctx["list"].([]any)
	if _, ok := list[0].(time.Time); !ok {
		t.Fatalf("expected ctx[list[0]] to be time.Time, got %T", list[0])
	}
	if list[1] != "not-a-date" {
		t.Fatalf("expected non-date string to pass through, got %v", list[1])
	}
}
