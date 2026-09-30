package main

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CORS", Label("unit"), func() {
	preflight := func(origin string) *httptest.ResponseRecorder {
		handler := corsMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		req := httptest.NewRequest(http.MethodOptions, "/api/sync/stream", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		req.Header.Set("Access-Control-Request-Headers", "content-type")

		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}

	DescribeTable("allows the app's webview origins",
		func(origin string) {
			Expect(preflight(origin).Header().Get("Access-Control-Allow-Origin")).To(Equal(origin))
		},
		Entry("iOS", "capacitor://localhost"),
		Entry("Android", "https://localhost"),
	)

	It("does not allow any other origin", func() {
		Expect(preflight("https://example.com").Header().Get("Access-Control-Allow-Origin")).To(BeEmpty())
	})
})
