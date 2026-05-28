package editor

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/zackarysantana/mongo-openfeature-go/src/client"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// run configures and starts the web application.
func RunEditor(mongoClient *mongo.Client, ofClient *client.Client) error {
	handler := NewWebHandler(ofClient)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /editor.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		http.ServeFile(w, r, "internal/editor/editor.js")
	})
	mux.HandleFunc("GET /assistant.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		http.ServeFile(w, r, "internal/editor/assistant.js")
	})
	mux.HandleFunc("GET /editor.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		http.ServeFile(w, r, "internal/editor/editor.css")
	})

	mux.HandleFunc("GET /api/auth/status", handler.HandleAuthStatus)
	mux.HandleFunc("GET /auth/openrouter/start", handler.HandleOpenRouterStart)
	mux.HandleFunc("GET /auth/openrouter/callback", handler.HandleOpenRouterCallback)
	mux.HandleFunc("POST /api/auth/logout", handler.HandleOpenRouterLogout)
	mux.HandleFunc("POST /api/chat", handler.HandleChat)
	mux.HandleFunc("GET /api/chat/stream", handler.HandleChatStream)
	mux.HandleFunc("GET /api/models", handler.HandleModels)

	mux.HandleFunc("GET /edit/", handler.HandleEditFlag)
	mux.HandleFunc("GET /edit/{name}", handler.HandleEditFlag)
	mux.HandleFunc("POST /save", handler.HandleSaveFlag)
	mux.HandleFunc("POST /delete", handler.HandleDeleteFlag)
	mux.HandleFunc("POST /test/{name}", handler.HandleEvaluateFlag)
	mux.HandleFunc("GET /", handler.HandleListFlags)

	port := ":3000"
	if envPort := os.Getenv("EDITOR_PORT"); envPort != "" {
		port = ":" + envPort
	}

	log.Println("Starting flag manager UI on http://localhost" + port)
	warnIfOpenRouterPKCEDisabled(port)
	return http.ListenAndServe(port, mux)
}

// warnIfOpenRouterPKCEDisabled logs when the listen port cannot be used for OpenRouter OAuth callbacks.
func warnIfOpenRouterPKCEDisabled(port string) {
	if os.Getenv("OPENROUTER_CALLBACK_URL") != "" {
		return
	}
	switch strings.TrimPrefix(port, ":") {
	case "3000", "443":
		return
	}
	log.Printf(
		"WARNING: OpenRouter PKCE OAuth requires callback port 443 or 3000; editor is listening on %s — Connect OpenRouter in Settings will not work unless OPENROUTER_CALLBACK_URL points to a supported port",
		port,
	)
}
