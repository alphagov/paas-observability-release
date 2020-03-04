package shipper_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestShipper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shipper Suite")
}
