package editor

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/zackarysantana/mongo-openfeature-go/src/client"
	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type WebHandler struct {
	client    *client.Client
	templates map[string]*template.Template
}

func NewWebHandler(c *client.Client) *WebHandler {
	templates := make(map[string]*template.Template)
	layout := template.Must(template.ParseFiles("internal/editor/layout.tmpl"))
	templates["index"] = template.Must(template.Must(layout.Clone()).ParseFiles("internal/editor/index.tmpl"))
	templates["edit"] = template.Must(template.Must(layout.Clone()).ParseFiles("internal/editor/edit.tmpl"))
	return &WebHandler{
		client:    c,
		templates: templates,
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
	fmt.Println("Flag name from path:", flagName)

	if flagName != "" {
		fmt.Println("Getting flag name")
		var err error
		def, err = h.client.GetFlag(r.Context(), flagName)
		if err != nil {
			// Your client returns a specific error string, so we check for that.
			// A better approach might be a custom error type.
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
func (h *WebHandler) HandleSaveFlag(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	var rules []rule.ConcreteRule
	if err := json.Unmarshal([]byte(r.FormValue("rules")), &rules); err != nil {
		http.Error(w, "Invalid JSON in rules: "+err.Error(), http.StatusBadRequest)
		return
	}

	var defaultValue any
	if err := json.Unmarshal([]byte(r.FormValue("defaultValue")), &defaultValue); err != nil {
		http.Error(w, "Invalid JSON in default value: "+err.Error(), http.StatusBadRequest)
		return
	}

	def := flag.Definition{
		FlagName:       r.FormValue("flagName"),
		DefaultVariant: r.FormValue("defaultVariant"),
		DefaultValue:   defaultValue,
		Rules:          rules,
	}

	if err := h.client.SetFlag(r.Context(), def); err != nil {
		log.Printf("ERROR saving flag: %v", err)
		http.Error(w, "Failed to save flag", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HandleDeleteFlag processes the delete request.
func (h *WebHandler) HandleDeleteFlag(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	flagName := r.FormValue("flagName")
	if flagName == "" {
		http.Error(w, "Missing flag name", http.StatusBadRequest)
		return
	}

	if err := h.client.DeleteFlag(r.Context(), flagName); err != nil {
		log.Printf("ERROR deleting flag: %v", err)
		http.Error(w, "Failed to delete flag", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// renderTemplate is a helper to execute the correct template.
func (h *WebHandler) renderTemplate(w http.ResponseWriter, name string, data any) {
	tmpl, ok := h.templates[name]
	if !ok {
		http.Error(w, "Template not found: "+name, http.StatusInternalServerError)
		return
	}
	// We always execute the template by its base file name now, which contains the layout.
	err := tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}
