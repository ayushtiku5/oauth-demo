package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

var oauthConfig = &oauth2.Config{
	ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
	ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
	RedirectURL:  "http://localhost:8080/auth/google/callback",
	Scopes: []string{
		"https://www.googleapis.com/auth/calendar.readonly",
	},
	Endpoint: google.Endpoint,
}

// In-memory token store keyed by session ID (prototype only — not for production).
var sessions = map[string]*oauth2.Token{}

func main() {
	if oauthConfig.ClientID == "" || oauthConfig.ClientSecret == "" {
		log.Fatal("Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables")
	}

	http.HandleFunc("/", handleHome)
	http.HandleFunc("/auth/google", handleGoogleLogin)
	http.HandleFunc("/auth/google/callback", handleGoogleCallback)
	http.HandleFunc("/calendar", handleCalendar)

	log.Println("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Step 0 — landing page with a login link.
func handleHome(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `<h1>OAuth + Google Calendar Demo</h1>
<p><a href="/auth/google">Login with Google</a></p>
<p><a href="/calendar">View Calendar Events</a></p>`)
}

// Step 1 — redirect the browser to Google's authorization endpoint.
func handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	// state is a random nonce that prevents CSRF. We echo it back in the
	// callback and reject responses where it doesn't match.
	state := randomState()
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
	})

	// AccessTypeOffline asks Google to include a refresh token so the app can
	// renew access without asking the user to log in again.
	url := oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// Step 2 — Google redirects here with ?code=...&state=...
func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Validate the state to guard against CSRF.
	cookie, err := r.Cookie("oauth_state")
	if err != nil || cookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}

	// Exchange the one-time authorization code for an access token (and
	// optionally a refresh token).
	code := r.URL.Query().Get("code")
	token, err := oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Persist the token in our in-memory session store.
	sessionID := randomState()
	sessions[sessionID] = token

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
	})

	log.Printf("Token obtained — type: %s, expiry: %s", token.TokenType, token.Expiry)
	http.Redirect(w, r, "/calendar", http.StatusTemporaryRedirect)
}

// Step 3 — use the access token to call the Google Calendar API.
func handleCalendar(w http.ResponseWriter, r *http.Request) {
	token, err := tokenFromSession(r)
	if err != nil {
		http.Redirect(w, r, "/auth/google", http.StatusTemporaryRedirect)
		return
	}

	// Build an HTTP client that automatically attaches the Bearer token to
	// every request and refreshes it when it expires.
	httpClient := oauthConfig.Client(context.Background(), token)
	svc, err := calendar.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		http.Error(w, "could not create calendar service: "+err.Error(), http.StatusInternalServerError)
		return
	}

	events, err := svc.Events.List("primary").
		TimeMin(time.Now().Format(time.RFC3339)).
		MaxResults(10).
		SingleEvents(true).
		OrderBy("startTime").
		Do()
	if err != nil {
		http.Error(w, "calendar fetch failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if len(events.Items) == 0 {
		fmt.Fprintln(w, `{"message": "No upcoming events found"}`)
		return
	}

	type event struct {
		Summary string `json:"summary"`
		Start   string `json:"start"`
	}
	var result []event
	for _, e := range events.Items {
		start := e.Start.DateTime
		if start == "" {
			start = e.Start.Date // all-day events have no DateTime
		}
		result = append(result, event{Summary: e.Summary, Start: start})
	}

	json.NewEncoder(w).Encode(result)
}

func tokenFromSession(r *http.Request) (*oauth2.Token, error) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return nil, fmt.Errorf("no session cookie")
	}
	token, ok := sessions[cookie.Value]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return token, nil
}

func randomState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
