package db

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDB(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DB Suite")
}

var _ = Describe("maxConnsFromEnv", Label("unit"), func() {
	It("uses the default when unset", func() {
		GinkgoT().Setenv(envDatabaseMaxConns, "")

		Expect(maxConnsFromEnv()).To(Equal(defaultMaxConns))
	})

	It("reads the configured limit", func() {
		GinkgoT().Setenv(envDatabaseMaxConns, "8")

		Expect(maxConnsFromEnv()).To(Equal(8))
	})

	DescribeTable("rejects a limit that is not a positive integer",
		func(raw string) {
			GinkgoT().Setenv(envDatabaseMaxConns, raw)

			_, err := maxConnsFromEnv()
			Expect(err).To(HaveOccurred())
		},
		Entry("zero", "0"),
		Entry("negative", "-3"),
		Entry("not a number", "lots"),
	)
})
