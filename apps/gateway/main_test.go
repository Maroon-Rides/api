package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/fx"
)

var _ = Describe("app", Label("unit"), func() {
	It("resolves its dependency graph", func() {
		Expect(fx.ValidateApp(fx.NopLogger, app)).To(Succeed())
	})
})
