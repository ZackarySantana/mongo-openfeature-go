package editor

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleEditFlagRedirectsBarePath(t *testing.T) {
	h := NewWebHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/edit/", nil)
	rec := httptest.NewRecorder()
	h.HandleEditFlag(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Fatalf("Location = %q, want /", loc)
	}
}
