package editor

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseDraftDefinition(t *testing.T) {
	form := url.Values{}
	form.Set("rules", `[{"exactMatchRule":{"Key":"user_id","KeyValue":"alice","VariantID":"on","ValueData":"true","Priority":0}}]`)
	form.Set("defaultVariant", "off")
	form.Set("defaultValue", `"fallback"`)

	req := httptest.NewRequest(http.MethodPost, "/test/my-flag", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ParseForm()

	def, err := parseDraftDefinition(req, "my-flag")
	if err != nil {
		t.Fatalf("parseDraftDefinition: %v", err)
	}
	if def.FlagName != "my-flag" {
		t.Fatalf("FlagName = %q, want my-flag", def.FlagName)
	}
	if def.DefaultVariant != "off" {
		t.Fatalf("DefaultVariant = %q, want off", def.DefaultVariant)
	}
	if len(def.Rules) != 1 {
		t.Fatalf("len(Rules) = %d, want 1", len(def.Rules))
	}
	if def.Rules[0].ExactMatchRule == nil || def.Rules[0].ExactMatchRule.Key != "user_id" {
		t.Fatalf("expected exactMatchRule on user_id")
	}
}

func TestHandleEvaluateFlagDraft(t *testing.T) {
	h := NewWebHandler(nil)

	form := url.Values{}
	form.Set("source", "draft")
	form.Set("context", `{"user_id":"alice"}`)
	form.Set("rules", `[{"exactMatchRule":{"Key":"user_id","KeyValue":"alice","VariantID":"on","ValueData":"true","Priority":10}}]`)
	form.Set("defaultVariant", "off")
	form.Set("defaultValue", `"fallback"`)

	req := httptest.NewRequest(http.MethodPost, "/test/my-flag", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("name", "my-flag")

	rec := httptest.NewRecorder()
	h.HandleEvaluateFlag(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, body)
	}
	for _, want := range []string{"test-out--matched", "on", "TARGETING_MATCH", "#1 exactMatchRule"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected response to contain %q; got:\n%s", want, body)
		}
	}
}

func TestHandleEvaluateFlagDraftInvalidRules(t *testing.T) {
	h := NewWebHandler(nil)

	form := url.Values{}
	form.Set("source", "draft")
	form.Set("context", `{}`)
	form.Set("rules", `{not-json`)

	req := httptest.NewRequest(http.MethodPost, "/test/my-flag", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("name", "my-flag")

	rec := httptest.NewRecorder()
	h.HandleEvaluateFlag(rec, req)

	if !strings.Contains(rec.Body.String(), "invalid rules JSON") {
		t.Fatalf("expected invalid rules error, got:\n%s", rec.Body.String())
	}
}
