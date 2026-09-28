package busapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"
)

const (
	headerVerificationToken = "Requestverificationtoken"
	headerRequestedWith     = "X-Requested-With"
	valueXMLHTTPRequest     = "XMLHttpRequest"

	sessionPageLimit = 2 << 20
)

// AuthProvider supplies the headers that authenticate each request.
// Implementations may refresh an expired session before returning.
type AuthProvider interface {
	Auth(ctx context.Context) (Auth, error)
}

// StaticAuth is an AuthProvider for headers that never change.
type StaticAuth Auth

func (a StaticAuth) Auth(context.Context) (Auth, error) {
	return Auth(a), nil
}

// verificationTokenPattern matches the base64 antiforgery token the site embeds in each page.
var verificationTokenPattern = regexp.MustCompile(`"([a-zA-Z0-9]{288}MQ==)"`)

// sessionAuth reads the antiforgery token off the landing page and reads it again once ttl
// elapses. The session cookie that the token is checked against lives in the client's jar.
type sessionAuth struct {
	http    *http.Client
	baseURL string
	ttl     time.Duration

	mu      sync.Mutex
	headers Auth
	renewAt time.Time
}

func (a *sessionAuth) Auth(ctx context.Context) (Auth, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.headers != nil && time.Now().Before(a.renewAt) {
		return a.headers, nil
	}

	token, err := a.fetchToken(ctx)
	if err != nil {
		return nil, err
	}

	a.headers = Auth{
		headerVerificationToken: token,
		headerRequestedWith:     valueXMLHTTPRequest,
	}
	a.renewAt = time.Now().Add(a.ttl)
	return a.headers, nil
}

func (a *sessionAuth) fetchToken(ctx context.Context) (string, error) {
	endpoint := a.baseURL + sessionPage

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("aggiespirit: build session request: %w", err)
	}

	res, err := a.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("aggiespirit: open session: %w", err)
	}
	defer res.Body.Close()

	page, err := io.ReadAll(io.LimitReader(res.Body, sessionPageLimit))
	if err != nil {
		return "", fmt.Errorf("aggiespirit: read session page: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", &StatusError{
			StatusCode: res.StatusCode,
			Method:     http.MethodGet,
			URL:        endpoint,
			Body:       truncate(string(page), errorBodyLimit),
		}
	}

	match := verificationTokenPattern.FindSubmatch(page)
	if match == nil {
		return "", fmt.Errorf("aggiespirit: no verification token on %s", endpoint)
	}

	token, err := base64.StdEncoding.DecodeString(string(match[1]))
	if err != nil {
		return "", fmt.Errorf("aggiespirit: decode verification token: %w", err)
	}
	return string(token), nil
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit]
}
