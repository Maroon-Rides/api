package gtfs

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGTFS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GTFS Suite")
}
