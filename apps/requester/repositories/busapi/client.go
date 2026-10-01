package busapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://aggiespirit.ts.tamu.edu"
	defaultTimeout = 30 * time.Second

	// defaultSessionTTL is how long a verification token is reused before it is read again.
	defaultSessionTTL = 1 * time.Hour
	// sessionPage is the page the verification token and session cookie are read from.
	sessionPage = "/"

	contentTypeJSON = "application/json"
	contentTypeForm = "application/x-www-form-urlencoded; charset=UTF-8"
	acceptAjax      = "application/json, text/javascript, */*; q=0.01"

	errorBodyLimit = 512

	antiforgeryErrorMarker = "anti-forgery"
)

// Client calls the Texas A&M AggieSpirit bus API. It is safe for concurrent use, and by
// default authenticates itself, so one client serves the whole process.
type Client struct {
	baseURL    string
	http       *http.Client
	auth       AuthProvider
	sessionTTL time.Duration
}

type ClientConfig struct {
	// defaults to "https://aggiespirit.ts.tamu.edu"
	BaseURL string

	HTTPClient *http.Client

	// defaults to handling session authentication automatically
	Auth AuthProvider

	// defaults to 1 hour
	SessionTTL time.Duration
}

func NewClient(cfg ClientConfig) *Client {
	c := &Client{
		baseURL:    defaultBaseURL,
		http:       cfg.HTTPClient,
		auth:       cfg.Auth,
		sessionTTL: defaultSessionTTL,
	}
	if cfg.BaseURL != "" {
		c.baseURL = strings.TrimSuffix(cfg.BaseURL, "/")
	}
	if cfg.SessionTTL != 0 {
		c.sessionTTL = cfg.SessionTTL
	}
	if c.http == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		// The server omits its intermediate certificate, so Go cannot build the chain.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
		c.http = &http.Client{Timeout: defaultTimeout, Transport: transport}
	}
	if c.http.Jar == nil {
		jar, _ := cookiejar.New(nil)
		c.http.Jar = jar
	}
	if c.auth == nil {
		c.auth = &sessionAuth{http: c.http, baseURL: c.baseURL, ttl: c.sessionTTL}
	}
	return c
}

// StatusError reports a response the API refused to serve.
type StatusError struct {
	StatusCode int
	Method     string
	URL        string
	Body       string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("aggiespirit: %s %s: %s: %s", e.Method, e.URL, http.StatusText(e.StatusCode), e.Body)
}

// sessionRejected reports whether the server refused the session's antiforgery token,
// which it does with a 500 error page rather than a 4xx.
func (e *StatusError) sessionRejected() bool {
	return e.StatusCode == http.StatusInternalServerError && strings.Contains(e.Body, antiforgeryErrorMarker)
}

type request struct {
	method      string
	path        string
	query       url.Values
	contentType string
	accept      string
	body        []byte
}

func (c *Client) do(ctx context.Context, r request, out any) error {
	err := c.send(ctx, r, out)

	var statusErr *StatusError
	if errors.As(err, &statusErr) && statusErr.sessionRejected() {
		c.auth.Invalidate()
		return c.send(ctx, r, out)
	}
	return err
}

func (c *Client) send(ctx context.Context, r request, out any) error {
	endpoint := c.baseURL + r.path
	if len(r.query) > 0 {
		endpoint += "?" + r.query.Encode()
	}

	var body io.Reader
	if r.body != nil {
		body = bytes.NewReader(r.body)
	}

	req, err := http.NewRequestWithContext(ctx, r.method, endpoint, body)
	if err != nil {
		return fmt.Errorf("aggiespirit: build request: %w", err)
	}

	auth, err := c.auth.Auth(ctx)
	if err != nil {
		return fmt.Errorf("aggiespirit: authenticate: %w", err)
	}
	for k, v := range auth {
		req.Header.Set(k, v)
	}
	if r.contentType != "" {
		req.Header.Set("Content-Type", r.contentType)
	}
	if r.accept != "" {
		req.Header.Set("Accept", r.accept)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("aggiespirit: %s %s: %w", r.method, r.path, err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, errorBodyLimit))
		return &StatusError{
			StatusCode: res.StatusCode,
			Method:     r.method,
			URL:        endpoint,
			Body:       string(snippet),
		}
	}

	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("aggiespirit: decode %s: %w", r.path, err)
	}
	return nil
}

func (c *Client) postJSON(ctx context.Context, path string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("aggiespirit: encode %s: %w", path, err)
	}
	return c.do(ctx, request{
		method:      http.MethodPost,
		path:        path,
		contentType: contentTypeJSON,
		body:        body,
	}, out)
}

func (c *Client) postForm(ctx context.Context, path, body string, out any) error {
	return c.do(ctx, request{
		method:      http.MethodPost,
		path:        path,
		contentType: contentTypeForm,
		body:        []byte(body),
	}, out)
}

func (c *Client) postEmpty(ctx context.Context, path string, out any) error {
	return c.do(ctx, request{method: http.MethodPost, path: path}, out)
}
