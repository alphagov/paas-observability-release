package integrator_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestIntegrator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integrator Suite")
}
