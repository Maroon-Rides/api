package services

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/MaroonRides/api/test"
)

func TestServices(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Medium Services Suite")
}

// Postgres boots once for this suite; each spec gets its own database via test.Configure.
var _ = SynchronizedBeforeSuite(test.StartBackends, test.UseBackends)

var _ = SynchronizedAfterSuite(func() {}, test.StopBackends)
