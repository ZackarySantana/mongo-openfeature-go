package editor

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/zackarysantana/mongo-openfeature-go/src/client"
	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type WebHandler struct {
	client    *client.Client
	templates map[string]*template.Template
	toast     *template.Template
}

func NewWebHandler(c *client.Client) *WebHandler {
	templates := make(map[string]*template.Template)
	layout := template.Must(template.ParseFiles("internal/editor/layout.tmpl"))
	templates["index"] = template.Must(template.Must(layout.Clone()).ParseFiles("internal/editor/index.tmpl"))
	templates["edit"] = template.Must(template.Must(layout.Clone()).ParseFiles("internal/editor/edit.tmpl"))
	toast := template.Must(template.ParseFiles("internal/editor/_toast.tmpl"))
	return &WebHandler{
		client:    c,
		templates: templates,
		toast:     toast,
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
	data := map[string]any{"Flags": flags}
	h.renderTemplate(w, "index", data)
}

// HandleEditFlag shows the form to edit a flag, or a blank form for a new one.
func (h *WebHandler) HandleEditFlag(w http.ResponseWriter, r *http.Request) {
	flagName := r.PathValue("name")
	var def *flag.Definition

	if flagName != "" {
		var err error
		def, err = h.client.GetFlag(r.Context(), flagName)
		if err != nil {
			log.Printf("Flag '%s' not found or error: %v", flagName, err)
		}
	}

	if def == nil {
		def = &flag.Definition{Rules: []rule.ConcreteRule{}}
	}

	rulesJSON, _ := json.MarshalIndent(def.Rules, "", "  ")
	defaultValueJSON, _ := json.Marshal(def.DefaultValue)

	if string(defaultValueJSON) == "null" {
		defaultValueJSON = []byte(`""`)
	}

	viewData := map[string]any{
		"Flag":             def,
		"RulesJSON":        string(rulesJSON),
		"DefaultValueJSON": string(defaultValueJSON),
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
