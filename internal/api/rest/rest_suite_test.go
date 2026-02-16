package rest_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestREST(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "REST Suite")
}
