package cursor_test

import (
	"io/ioutil"
	"os"
	"time"

	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/alphagov/paas-observability-release/src/bosh-auditor/pkg/cursor"
)

var _ = Describe("FileCursor", func() {
	var (
		err   error
		tempd string

		logger lager.Logger

		fc cursor.Cursor
	)

	BeforeEach(func() {
		tempd, err = ioutil.TempDir("", "bosh-auditor-cursor")
		Expect(err).NotTo(HaveOccurred())

		logger = lager.NewLogger("bosh-auditor-cursor-test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))

		fc = cursor.NewFileCursor(
			"test-cursor",
			tempd,
			time.Unix(0, 0),
			logger,
		)
	})

	AfterEach(func() {
		if tempd != "" {
			os.RemoveAll(tempd)
		}
	})

	Context("when using a cursor", func() {
		It("should set the time, then get the time", func() {
			currTime := time.Now()
			err = fc.UpdateTime(currTime)
			Expect(err).NotTo(HaveOccurred())

			gotTime := fc.GetTime()
			Expect(currTime).To(BeTemporally("~", gotTime, 1*time.Second))
		})
	})

	Context("when getting the time", func() {
		Context("when the time has not been set", func() {
			It("should fallback to the default value", func() {
				gotTime := fc.GetTime()
				Expect(time.Unix(0, 0)).To(BeTemporally("~", gotTime, 1*time.Second))
			})
		})

		Context("when the file does not exist", func() {
			It("should fallback to the default value", func() {
				fc = cursor.NewFileCursor(
					"test-cursor",
					"/path/does/not/exist",
					time.Unix(0, 0),
					logger,
				)
				gotTime := fc.GetTime()
				Expect(time.Unix(0, 0)).To(BeTemporally("~", gotTime, 1*time.Second))
			})
		})
	})

	Context("when setting the time", func() {
		Context("when the file does not exist", func() {
			It("should return an error", func() {
				fc = cursor.NewFileCursor(
					"test-cursor",
					"/path/does/not/exist",
					time.Unix(0, 0),
					logger,
				)
				currTime := time.Now()
				err = fc.UpdateTime(currTime)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
