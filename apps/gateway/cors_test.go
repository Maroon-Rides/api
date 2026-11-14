package main

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CORS", Label("unit"), func() {
	DescribeTable("allows any origin",
		func(origin string) {
			handler := corsMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
			req := httptest.NewRequest(http.MethodOptions, "/api/sync/stream", nil)
			req.Header.Set("Origin", origin)
			req.Header.Set("Access-Control-Request-Method", http.MethodPost)
			req.Header.Set("Access-Control-Request-Headers", "content-type,x-app-platform,x-app-version")

			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)

			Expect(res.Header().Get("Access-Control-Allow-Origin")).To(Equal("*"))
			Expect(res.Header().Get("Access-Control-Allow-Headers")).To(Equal("content-type,x-app-platform,x-app-version"))
		},
		Entry("iOS app", "capacitor://localhost"),
		Entry("Android app", "https://localhost"),
		Entry("web app", "https://web.maroonrides.app"),
		Entry("Vite dev server", "http://100.89.139.58:5173"),
	)
})
