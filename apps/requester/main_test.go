package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/fx"
)

func TestRequester(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Requester Suite")
}

var _ = Describe("app", Label("unit"), func() {
	It("resolves its dependency graph", func() {
		Expect(fx.ValidateApp(fx.NopLogger, app)).To(Succeed())
	})
})
