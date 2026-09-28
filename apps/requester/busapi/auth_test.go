package busapi

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The site embeds the token as base64 that is always 288 characters plus a "MQ==" group,
// which is what the extraction pattern keys on.
const testToken = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
	"1"

// newSessionServer serves a landing page carrying the token and a session cookie, and a JSON
// document at every other path. It reports how many times the landing page was read.
func newSessionServer(t *testing.T, page string) (*httptest.Server, *capturedRequest, *int) {
	t.Helper()

	var captured capturedRequest
	var sessionReads int

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		sessionReads++
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Path: "/"})
		io.WriteString(w, page)
	})
	mux.HandleFunc(pathActiveRoutes, func(w http.ResponseWriter, r *http.Request) {
		captured = capturedRequest{header: r.Header.Clone()}
		w.Header().Set("Content-Type", contentTypeJSON)
		io.WriteString(w, `[]`)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, &captured, &sessionReads
}

func landingPage(token string) string {
	return `<input name="__RequestVerificationToken" value="` +
		base64.StdEncoding.EncodeToString([]byte(token)) + `" />`
}

func TestSessionAuthSendsDecodedTokenAndSessionCookie(t *testing.T) {
	server, captured, _ := newSessionServer(t, landingPage(testToken))
	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})

	if _, err := client.GetActiveRoutes(context.Background()); err != nil {
		t.Fatalf("GetActiveRoutes: %v", err)
	}

	if got := captured.header.Get(headerVerificationToken); got != testToken {
		t.Errorf("%s = %q, want the decoded token", headerVerificationToken, got)
	}
	if got := captured.header.Get(headerRequestedWith); got != valueXMLHTTPRequest {
		t.Errorf("%s = %q, want %q", headerRequestedWith, got, valueXMLHTTPRequest)
	}
	if got := captured.header.Get("Cookie"); !strings.Contains(got, "session=abc") {
		t.Errorf("Cookie = %q, want the session cookie from the landing page", got)
	}
}

func TestSessionTokenIsReusedUntilTheIntervalElapses(t *testing.T) {
	server, _, sessionReads := newSessionServer(t, landingPage(testToken))
	client := NewClient(ClientConfig{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		SessionTTL: time.Hour,
	})

	for range 3 {
		if _, err := client.GetActiveRoutes(context.Background()); err != nil {
			t.Fatalf("GetActiveRoutes: %v", err)
		}
	}

	if *sessionReads != 1 {
		t.Errorf("landing page reads = %d, want 1", *sessionReads)
	}
}

func TestSessionTokenIsReadAgainOnceTheIntervalElapses(t *testing.T) {
	server, _, sessionReads := newSessionServer(t, landingPage(testToken))
	client := NewClient(ClientConfig{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		SessionTTL: -1,
	})

	for range 3 {
		if _, err := client.GetActiveRoutes(context.Background()); err != nil {
			t.Fatalf("GetActiveRoutes: %v", err)
		}
	}

	if *sessionReads != 3 {
		t.Errorf("landing page reads = %d, want 3", *sessionReads)
	}
}

func TestSessionAuthFailsWhenThePageCarriesNoToken(t *testing.T) {
	server, _, _ := newSessionServer(t, "<html>maintenance</html>")
	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})

	_, err := client.GetActiveRoutes(context.Background())
	if err == nil {
		t.Fatal("GetActiveRoutes succeeded, want a missing token error")
	}
	if !strings.Contains(err.Error(), "no verification token") {
		t.Errorf("error = %v, want it to name the missing token", err)
	}
}
