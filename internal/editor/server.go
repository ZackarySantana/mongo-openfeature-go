package editor

import (
	"log"
	"net/http"
	"os"

	"github.com/zackarysantana/mongo-openfeature-go/src/client"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// run configures and starts the web application.
func RunEditor(mongoClient *mongo.Client, ofClient *client.Client) error {
	handler := NewWebHandler(ofClient)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /editor.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "internal/editor/editor.js")
	})

	mux.HandleFunc("GET /edit/", handler.HandleEditFlag)
	mux.HandleFunc("GET /edit/{name}", handler.HandleEditFlag)
	mux.HandleFunc("POST /save", handler.HandleSaveFlag)
	mux.HandleFunc("POST /delete", handler.HandleDeleteFlag)
	mux.HandleFunc("GET /", handler.HandleListFlags)

	port := ":8080"
	if envPort := os.Getenv("EDITOR_PORT"); envPort != "" {
		port = ":" + envPort
	}

	log.Println("Starting flag manager UI on http://localhost" + port)
	return http.ListenAndServe(port, mux)
}
