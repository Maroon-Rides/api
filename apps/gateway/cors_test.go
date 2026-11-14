package main

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CORS", Label("unit"), func() {
	preflight := func(origin, method, headers string) *httptest.ResponseRecorder {
		handler := corsMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		req := httptest.NewRequest(http.MethodOptions, "/api/sync/stream", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", method)
		req.Header.Set("Access-Control-Request-Headers", headers)

		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}

	DescribeTable("allows any origin",
		func(origin string) {
			res := preflight(origin, http.MethodPost, "content-type")

			Expect(res.Header().Get("Access-Control-Allow-Origin")).To(Equal("*"))
		},
		Entry("iOS app", "capacitor://localhost"),
		Entry("Android app", "https://localhost"),
		Entry("web app", "https://web.maroonrides.app"),
		Entry("Vite dev server", "http://100.89.139.58:5173"),
	)

	It("allows the headers the app names itself with", func() {
		res := preflight("https://web.maroonrides.app", http.MethodGet, "x-app-platform,x-app-version")

		Expect(res.Header().Get("Access-Control-Allow-Origin")).To(Equal("*"))
		Expect(res.Header().Get("Access-Control-Allow-Headers")).To(Equal("x-app-platform,x-app-version"))
	})
})
