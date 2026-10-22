package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/go-fuego/fuego"
	"github.com/jackc/pgx/v5/pgconn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("handleError", Label("unit"), func() {
	serve := func(err error) *httptest.ResponseRecorder {
		server := fuego.NewServer(fuego.WithEngineOptions(fuego.WithErrorHandler(handleError)))
		fuego.Get(server, "/fail", func(fuego.ContextNoBody) (any, error) { return nil, err })
		res := httptest.NewRecorder()
		server.Mux.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/fail", nil))
		return res
	}

	It("hides the fields of a raw error", func() {
		res := serve(&pgconn.PgError{Message: "secret message", Detail: "secret detail", TableName: "secret_table"})

		Expect(res.Code).To(Equal(http.StatusInternalServerError))
		Expect(res.Body.String()).NotTo(ContainSubstring("secret"))
	})

	It("keeps the detail of an HTTP error", func() {
		res := serve(fuego.BadRequestError{Detail: "no types requested", Err: fmt.Errorf("wrapped")})

		Expect(res.Code).To(Equal(http.StatusBadRequest))
		Expect(res.Body.String()).To(ContainSubstring("no types requested"))
		Expect(res.Body.String()).NotTo(ContainSubstring("wrapped"))
	})
})
