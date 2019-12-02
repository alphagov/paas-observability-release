package discoverer_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDiscoverer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Discoverer Suite")
}
