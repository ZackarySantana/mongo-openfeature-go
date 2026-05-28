package editor

import (
	"io"
	"net/http"
)

const openRouterModelsURL = "https://openrouter.ai/api/v1/models"

// HandleModels proxies the OpenRouter model list using the caller's stored API key.
// The frontend uses this to populate the in-app model selector.
func (h *WebHandler) HandleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	apiKey := apiKeyFromRequest(r)
	if apiKey == "" {
		http.Error(w, "Connect OpenRouter to use the assistant.", http.StatusUnauthorized)
		return
	}

	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodGet, openRouterModelsURL, nil)
	if err != nil {
		http.Error(w, "Failed to build models request.", http.StatusInternalServerError)
		return
	}
	upstream.Header.Set("Authorization", "Bearer "+apiKey)
	upstream.Header.Set("Accept", "application/json")
	upstream.Header.Set("HTTP-Referer", openRouterReferer(r))
	upstream.Header.Set("X-Title", "mongo-openfeature-go Flag Manager")

	resp, err := http.DefaultClient.Do(upstream)
	if err != nil {
		http.Error(w, "Failed to reach OpenRouter.", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
