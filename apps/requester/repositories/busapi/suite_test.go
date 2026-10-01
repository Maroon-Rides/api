package busapi

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBusAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bus API Suite")
}
