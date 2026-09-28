package dtos_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDtos(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DTOs Suite")
}
