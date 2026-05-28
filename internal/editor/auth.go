package editor

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	cookiePKCEVerifier = "or_pkce_verifier"
	cookieAPIKey       = "or_api_key"

	openRouterAuthURL     = "https://openrouter.ai/auth"
	openRouterAuthKeysURL = "https://openrouter.ai/api/v1/auth/keys"
)

type authStatusResponse struct {
	Connected bool `json:"connected"`
}

type authKeysResponse struct {
	Key string `json:"key"`
}

// HandleAuthStatus returns whether an OpenRouter API key is stored for this browser session.
func (h *WebHandler) HandleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, authStatusResponse{Connected: apiKeyFromRequest(r) != ""})
}

// HandleOpenRouterStart begins the OpenRouter OAuth PKCE flow.
func (h *WebHandler) HandleOpenRouterStart(w http.ResponseWriter, r *http.Request) {
	verifier, err := generateCodeVerifier()
	if err != nil {
		http.Error(w, "Failed to start authentication", http.StatusInternalServerError)
		return
	}

	challenge := codeChallengeS256(verifier)
	callback := openRouterCallbackURL(r)

	http.SetCookie(w, &http.Cookie{
		Name:     cookiePKCEVerifier,
		Value:    verifier,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	authURL, err := url.Parse(openRouterAuthURL)
	if err != nil {
		http.Error(w, "Failed to start authentication", http.StatusInternalServerError)
		return
	}
	q := authURL.Query()
	q.Set("callback_url", callback)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	authURL.RawQuery = q.Encode()

	http.Redirect(w, r, authURL.String(), http.StatusFound)
}

// HandleOpenRouterCallback completes the PKCE exchange and stores the API key in an httpOnly cookie.
func (h *WebHandler) HandleOpenRouterCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/?auth=missing_code", http.StatusSeeOther)
		return
	}

	verifierCookie, err := r.Cookie(cookiePKCEVerifier)
	if err != nil || verifierCookie.Value == "" {
		http.Redirect(w, r, "/?auth=missing_verifier", http.StatusSeeOther)
		return
	}

	key, err := exchangeOpenRouterCode(r, code, verifierCookie.Value)
	if err != nil {
		http.Redirect(w, r, "/?auth=exchange_failed", http.StatusSeeOther)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookiePKCEVerifier,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     cookieAPIKey,
		Value:    key,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})

	http.Redirect(w, r, "/?auth=success", http.StatusSeeOther)
}

// HandleOpenRouterLogout clears the stored OpenRouter API key.
func (h *WebHandler) HandleOpenRouterLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieAPIKey,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, authStatusResponse{Connected: false})
}

func apiKeyFromRequest(r *http.Request) string {
	c, err := r.Cookie(cookieAPIKey)
	if err != nil {
		return ""
	}
	return c.Value
}

func openRouterCallbackURL(r *http.Request) string {
	if override := os.Getenv("OPENROUTER_CALLBACK_URL"); override != "" {
		return override
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/auth/openrouter/callback", scheme, r.Host)
}

func openRouterReferer(r *http.Request) string {
	if override := os.Getenv("OPENROUTER_HTTP_REFERER"); override != "" {
		return override
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/", scheme, r.Host)
}

func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64URLEncode(b), nil
}

func codeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64URLEncode(sum[:])
}

func base64URLEncode(b []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}

func exchangeOpenRouterCode(r *http.Request, code, verifier string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"code":                  code,
		"code_verifier":         verifier,
		"code_challenge_method": "S256",
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, openRouterAuthKeysURL, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter auth exchange: %s", strings.TrimSpace(string(respBody)))
	}

	var parsed authKeysResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if parsed.Key == "" {
		return "", fmt.Errorf("openrouter auth exchange: empty key")
	}
	return parsed.Key, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
