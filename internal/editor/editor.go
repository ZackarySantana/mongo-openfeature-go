package editor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/zackarysantana/mongo-openfeature-go/src/client"
	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type WebHandler struct {
	client     *client.Client
	templates  map[string]*template.Template
	toast      *template.Template
	testResult *template.Template
}

func NewWebHandler(c *client.Client) *WebHandler {
	templates := make(map[string]*template.Template)
	layout := template.Must(template.ParseFiles("internal/editor/layout.tmpl"))
	templates["index"] = template.Must(template.Must(layout.Clone()).ParseFiles("internal/editor/index.tmpl"))
	// The edit page renders the test-result partial as a placeholder for the
	// inline tester, so parse it into the same tree.
	templates["edit"] = template.Must(template.Must(layout.Clone()).ParseFiles(
		"internal/editor/edit.tmpl",
		"internal/editor/_test_result.tmpl",
	))
	toast := template.Must(template.ParseFiles("internal/editor/_toast.tmpl"))
	testResult := template.Must(template.ParseFiles("internal/editor/_test_result.tmpl"))
	return &WebHandler{
		client:     c,
		templates:  templates,
		toast:      toast,
		testResult: testResult,
	}
}

// isHTMX returns true if the request was initiated by htmx.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// toastData is the payload rendered by the _toast.tmpl partial.
type toastData struct {
	Error bool
	Title string
	Body  string
}

// writeToast renders the toast partial as the response body.
func (h *WebHandler) writeToast(w http.ResponseWriter, status int, t toastData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := h.toast.ExecuteTemplate(w, "toast", t); err != nil {
		log.Printf("ERROR rendering toast: %v", err)
	}
}

// HandleListFlags shows the home page with a list of all flags.
func (h *WebHandler) HandleListFlags(w http.ResponseWriter, r *http.Request) {
	flags, err := h.client.GetAllFlags(r.Context())
	if err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("ERROR fetching flags: %v", err)
			http.Error(w, "Failed to fetch flags", http.StatusInternalServerError)
			return
		}
	}
	flagNames := make([]string, 0, len(flags))
	for _, f := range flags {
		flagNames = append(flagNames, f.FlagName)
	}
	flagNamesJSON, _ := json.Marshal(flagNames)
	data := map[string]any{
		"FlagSections":  BuildFlagListSections(flags),
		"HasFlags":      len(flags) > 0,
		"FlagNamesJSON": string(flagNamesJSON),
	}
	h.renderTemplate(w, "index", data)
}

// HandleEditFlag shows the form to edit an existing flag or configure a new one
// that was named on the home page. Bare /edit/ redirects to the flag list.
func (h *WebHandler) HandleEditFlag(w http.ResponseWriter, r *http.Request) {
	flagName := r.PathValue("name")
	if flagName == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	var def *flag.Definition
	def, err := h.client.GetFlag(r.Context(), flagName)
	if err != nil {
		log.Printf("Flag '%s' not found or error: %v", flagName, err)
		def = &flag.Definition{
			FlagName: flagName,
			Rules:    []rule.ConcreteRule{},
		}
	}

	rulesJSON, _ := json.MarshalIndent(def.Rules, "", "  ")
	defaultValueJSON, _ := json.Marshal(def.DefaultValue)
	contextKeyFieldsJSON, _ := json.Marshal(rule.CollectContextKeyFields(def.Rules))

	if string(defaultValueJSON) == "null" {
		defaultValueJSON = []byte(`""`)
	}

	viewData := map[string]any{
		"Flag":                 def,
		"Categories":           h.listCategories(r.Context()),
		"RulesJSON":            string(rulesJSON),
		"DefaultValueJSON":     string(defaultValueJSON),
		"ContextKeyFields":     rule.CollectContextKeyFields(def.Rules),
		"ContextKeyFieldsJSON": string(contextKeyFieldsJSON),
		// Pre-render the tester output region with an empty placeholder so the
		// layout reserves space on first paint and doesn't shift after Run test.
		"TestResult": testResultData{},
	}
	h.renderTemplate(w, "edit", viewData)
}

// HandleSaveFlag processes the form submission from the edit page.
// htmx requests get a toast partial back; classic form posts redirect to "/".
func (h *WebHandler) HandleSaveFlag(w http.ResponseWriter, r *http.Request) {
	htmx := isHTMX(r)

	if err := r.ParseForm(); err != nil {
		if htmx {
			h.writeToast(w, http.StatusOK, toastData{Error: true, Title: "Bad request", Body: "Could not parse form."})
			return
		}
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	var rules []rule.ConcreteRule
	if err := json.Unmarshal([]byte(r.FormValue("rules")), &rules); err != nil {
		if htmx {
			h.writeToast(w, http.StatusOK, toastData{Error: true, Title: "Invalid rules JSON", Body: err.Error()})
			return
		}
		http.Error(w, "Invalid JSON in rules: "+err.Error(), http.StatusBadRequest)
		return
	}

	var defaultValue any
	if err := json.Unmarshal([]byte(r.FormValue("defaultValue")), &defaultValue); err != nil {
		if htmx {
			h.writeToast(w, http.StatusOK, toastData{Error: true, Title: "Invalid default value", Body: err.Error()})
			return
		}
		http.Error(w, "Invalid JSON in default value: "+err.Error(), http.StatusBadRequest)
		return
	}

	flagName := r.FormValue("flagName")
	def := flag.Definition{
		FlagName:       flagName,
		DefaultVariant: r.FormValue("defaultVariant"),
		DefaultValue:   defaultValue,
		Category:       strings.TrimSpace(r.FormValue("category")),
		Rules:          rules,
	}

	if err := h.client.SetFlag(r.Context(), def); err != nil {
		log.Printf("ERROR saving flag: %v", err)
		if htmx {
			h.writeToast(w, http.StatusOK, toastData{Error: true, Title: "Save failed", Body: "Could not save the flag. Check server logs."})
			return
		}
		http.Error(w, "Failed to save flag", http.StatusInternalServerError)
		return
	}

	if htmx {
		if r.FormValue("afterSave") == "list" {
			w.Header().Set("HX-Redirect", "/")
		}
		h.writeToast(w, http.StatusOK, toastData{
			Title: "Flag saved",
			Body:  fmt.Sprintf("Changes to %q have been saved.", flagName),
		})
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HandleDeleteFlag processes the delete request.
// htmx requests get an empty body + HX-Trigger toast event; classic form posts redirect to "/".
func (h *WebHandler) HandleDeleteFlag(w http.ResponseWriter, r *http.Request) {
	htmx := isHTMX(r)

	if err := r.ParseForm(); err != nil {
		if htmx {
			h.writeToast(w, http.StatusOK, toastData{Error: true, Title: "Bad request", Body: "Could not parse form."})
			return
		}
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	flagName := r.FormValue("flagName")
	if flagName == "" {
		if htmx {
			h.writeToast(w, http.StatusOK, toastData{Error: true, Title: "Missing flag name", Body: "No flag name provided."})
			return
		}
		http.Error(w, "Missing flag name", http.StatusBadRequest)
		return
	}

	if err := h.client.DeleteFlag(r.Context(), flagName); err != nil {
		log.Printf("ERROR deleting flag: %v", err)
		if htmx {
			h.writeToast(w, http.StatusOK, toastData{Error: true, Title: "Delete failed", Body: "Could not delete the flag."})
			return
		}
		http.Error(w, "Failed to delete flag", http.StatusInternalServerError)
		return
	}

	if htmx {
		// Fire a toast via HX-Trigger so any swap behavior (e.g. row removal
		// or redirect) on the client stays decoupled from the toast.
		trigger := fmt.Sprintf(`{"showToast":{"kind":"success","title":"Flag deleted","body":"%s was removed."}}`, jsonEscape(flagName))
		w.Header().Set("HX-Trigger", trigger)

		// If the request came from the edit page, navigate back to the list.
		// We detect this by checking the HX-Current-URL header.
		if cur := r.Header.Get("HX-Current-URL"); cur != "" && containsEditPath(cur) {
			w.Header().Set("HX-Redirect", "/")
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// containsEditPath reports whether the given URL path includes the edit prefix.
func containsEditPath(url string) bool {
	return strings.Contains(url, "/edit")
}

// jsonEscape escapes a string for safe inclusion inside a JSON value used in
// the HX-Trigger response header. It handles the small set of characters that
// matter for header-embedded JSON (quotes, backslashes, control chars).
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	if len(b) >= 2 {
		// Trim surrounding quotes added by json.Marshal.
		return string(b[1 : len(b)-1])
	}
	return s
}

// testResultData is rendered by _test_result.tmpl as the inline tester's output.
type testResultData struct {
	// Matched is true when a rule matched and the result is not the default.
	Matched bool
	Variant string
	Reason  string
	// ValueJSON is the resolved value pretty-printed as JSON.
	ValueJSON string
	// Error, when set, replaces the normal output with a failure message.
	Error string
	// MatchedRuleIndex is the 0-based index of the winning top-level rule in the
	// saved definition, or -1 when the default value was used.
	MatchedRuleIndex int
	// MatchedRuleType is the rule variant key (e.g. "exactMatchRule").
	MatchedRuleType string
	// MatchedRuleLabel is a human-readable label for the matched rule row.
	MatchedRuleLabel string
}

// HandleEvaluateFlag evaluates a flag against the user-supplied JSON context from
// the inline tester and returns the result partial. By default it uses the saved
// definition from storage; when source=draft it evaluates the unsaved form state.
func (h *WebHandler) HandleEvaluateFlag(w http.ResponseWriter, r *http.Request) {
	flagName := r.PathValue("name")
	if flagName == "" {
		h.writeTestResult(w, testResultData{Error: "Missing flag name."})
		return
	}

	if err := r.ParseForm(); err != nil {
		h.writeTestResult(w, testResultData{Error: "Could not parse form: " + err.Error()})
		return
	}

	contextStr := strings.TrimSpace(r.FormValue("context"))
	if contextStr == "" {
		contextStr = "{}"
	}

	def, err := h.loadFlagForEvaluation(r, flagName)
	if err != nil {
		log.Printf("ERROR preparing flag '%s' for evaluation: %v", flagName, err)
		h.writeTestResult(w, testResultData{Error: err.Error()})
		return
	}

	ctx := map[string]any{}
	if err := json.Unmarshal([]byte(contextStr), &ctx); err != nil {
		h.writeTestResult(w, testResultData{Error: "Invalid context JSON: " + err.Error()})
		return
	}

	// Date/cron/etc. rules expect time.Time, but JSON only carries strings.
	// Best-effort: any string that parses as RFC3339 is upgraded to time.Time
	// so time-based rules behave the same way they would in real Go callers.
	convertTimestamps(ctx)

	match := def.EvaluateWithMatch(ctx)

	valueJSON, marshalErr := json.MarshalIndent(match.Value, "", "  ")
	if marshalErr != nil {
		valueJSON = []byte(fmt.Sprintf("%v", match.Value))
	}

	result := testResultData{
		Matched:   match.Detail.Reason == openfeature.TargetingMatchReason,
		Variant:   match.Detail.Variant,
		Reason:    string(match.Detail.Reason),
		ValueJSON: string(valueJSON),
	}

	if result.Matched && match.MatchedRuleIndex >= 0 && match.MatchedRuleIndex < len(def.Rules) {
		matched := def.Rules[match.MatchedRuleIndex]
		result.MatchedRuleIndex = match.MatchedRuleIndex
		result.MatchedRuleType = matched.RuleType()
		result.MatchedRuleLabel = formatMatchedRuleLabel(match.MatchedRuleIndex, matched)
	}

	h.writeTestResult(w, result)
}

// loadFlagForEvaluation returns the flag definition to evaluate. source=saved
// (the default) loads from storage; source=draft builds from the edit form.
func (h *WebHandler) loadFlagForEvaluation(r *http.Request, flagName string) (*flag.Definition, error) {
	source := strings.TrimSpace(r.FormValue("source"))
	if source == "" {
		source = "draft"
	}

	switch source {
	case "saved":
		def, err := h.client.GetFlag(r.Context(), flagName)
		if err != nil {
			return nil, fmt.Errorf("flag not found: %s", flagName)
		}
		return def, nil
	case "draft":
		return parseDraftDefinition(r, flagName)
	default:
		return nil, fmt.Errorf("unknown evaluation source %q", source)
	}
}

func parseDraftDefinition(r *http.Request, flagName string) (*flag.Definition, error) {
	rulesStr := strings.TrimSpace(r.FormValue("rules"))
	if rulesStr == "" {
		rulesStr = "[]"
	}

	var rules []rule.ConcreteRule
	if err := json.Unmarshal([]byte(rulesStr), &rules); err != nil {
		return nil, fmt.Errorf("invalid rules JSON: %w", err)
	}

	defaultValueStr := strings.TrimSpace(r.FormValue("defaultValue"))
	if defaultValueStr == "" {
		defaultValueStr = "null"
	}

	var defaultValue any
	if err := json.Unmarshal([]byte(defaultValueStr), &defaultValue); err != nil {
		return nil, fmt.Errorf("invalid default value JSON: %w", err)
	}

	return &flag.Definition{
		FlagName:       flagName,
		DefaultVariant: r.FormValue("defaultVariant"),
		DefaultValue:   defaultValue,
		Rules:          rules,
	}, nil
}

// formatMatchedRuleLabel builds the display string shown in the tester result,
// mirroring the rules overview format: "#2 exactMatchRule" with optional variant.
func formatMatchedRuleLabel(index int, r rule.ConcreteRule) string {
	label := fmt.Sprintf("#%d %s", index+1, r.RuleType())
	if v := r.Variant(); v != "" {
		label += " · " + v
	}
	return label
}

// writeTestResult renders the result partial as the response body.
func (h *WebHandler) writeTestResult(w http.ResponseWriter, result testResultData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.testResult.ExecuteTemplate(w, "test-result", result); err != nil {
		log.Printf("ERROR rendering test result: %v", err)
	}
}

// convertTimestamps walks a decoded JSON map and replaces any string that parses
// as RFC3339 with the corresponding time.Time. This lets date-based rules
// (DateTimeRule, CronRule) be tested through the JSON context input.
func convertTimestamps(m map[string]any) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				m[k] = t
			}
		case map[string]any:
			convertTimestamps(val)
		case []any:
			convertTimestampsSlice(val)
		}
	}
}

func convertTimestampsSlice(s []any) {
	for i, v := range s {
		switch val := v.(type) {
		case string:
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				s[i] = t
			}
		case map[string]any:
			convertTimestamps(val)
		case []any:
			convertTimestampsSlice(val)
		}
	}
}

// listCategories returns sorted category names from all saved flags.
func (h *WebHandler) listCategories(ctx context.Context) []string {
	if h.client == nil {
		return nil
	}
	flags, err := h.client.GetAllFlags(ctx)
	if err != nil {
		return nil
	}
	return CollectCategories(flags)
}

// renderTemplate is a helper to execute the correct template.
func (h *WebHandler) renderTemplate(w http.ResponseWriter, name string, data any) {
	tmpl, ok := h.templates[name]
	if !ok {
		http.Error(w, "Template not found: "+name, http.StatusInternalServerError)
		return
	}
	err := tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}
