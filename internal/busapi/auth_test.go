package busapi

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The site embeds the token as base64 that is always 288 characters plus a "MQ==" group,
// which is what the extraction pattern keys on.
const testToken = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
	"1"

// newSessionServer serves a landing page carrying the token and a session cookie, and a JSON
// document at every other path. It reports how many times the landing page was read.
func newSessionServer(page string) (*httptest.Server, *capturedRequest, *int) {
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
	DeferCleanup(server.Close)
	return server, &captured, &sessionReads
}

func landingPage(token string) string {
	return `<input name="__RequestVerificationToken" value="` +
		base64.StdEncoding.EncodeToString([]byte(token)) + `" />`
}

var _ = Describe("session auth", Label("unit"), func() {
	ctx := context.Background()

	It("sends the decoded token and session cookie", func() {
		server, captured, _ := newSessionServer(landingPage(testToken))
		client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})

		_, err := client.GetActiveRoutes(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(captured.header.Get(headerVerificationToken)).To(Equal(testToken))
		Expect(captured.header.Get(headerRequestedWith)).To(Equal(valueXMLHTTPRequest))
		Expect(captured.header.Get("Cookie")).To(ContainSubstring("session=abc"))
	})

	DescribeTable("reads the landing page again only once the session TTL elapses",
		func(ttl time.Duration, wantReads int) {
			server, _, sessionReads := newSessionServer(landingPage(testToken))
			client := NewClient(ClientConfig{
				BaseURL:    server.URL,
				HTTPClient: server.Client(),
				SessionTTL: ttl,
			})

			for range 3 {
				_, err := client.GetActiveRoutes(ctx)
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(*sessionReads).To(Equal(wantReads))
		},
		Entry("reuses the token within the TTL", time.Hour, 1),
		Entry("reads it again after the TTL", time.Duration(-1), 3),
	)

	It("fails when the page carries no token", func() {
		server, _, _ := newSessionServer("<html>maintenance</html>")
		client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})

		_, err := client.GetActiveRoutes(ctx)
		Expect(err).To(MatchError(ContainSubstring("no verification token")))
	})

	It("renews a rejected session and retries the request", func() {
		var sessionReads int
		var apiCalls int

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			sessionReads++
			io.WriteString(w, landingPage(testToken))
		})
		mux.HandleFunc(pathActiveRoutes, func(w http.ResponseWriter, r *http.Request) {
			apiCalls++
			if apiCalls == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, `<title>The anti-forgery token could not be decrypted.</title>`)
				return
			}
			w.Header().Set("Content-Type", contentTypeJSON)
			io.WriteString(w, `[]`)
		})
		server := httptest.NewServer(mux)
		DeferCleanup(server.Close)

		client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
		_, err := client.GetActiveRoutes(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(sessionReads).To(Equal(2))
	})
})
