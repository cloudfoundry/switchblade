package switchblade_test

import (
	"testing"

	"github.com/ryanmoran/switchblade"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testPlatform(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("when given a cf platform type", func() {
		it("returns a cf platform", func() {
			platform, err := switchblade.NewPlatform("cf", "some-token")
			Expect(err).NotTo(HaveOccurred())

			_, ok := platform.Deploy.(switchblade.CloudFoundryDeployProcess)
			Expect(ok).To(BeTrue())

			_, ok = platform.Delete.(switchblade.CloudFoundryDeleteProcess)
			Expect(ok).To(BeTrue())
		})
	})

	context("when given a docker platform type", func() {
		it("returns a cf platform", func() {
			platform, err := switchblade.NewPlatform("docker", "some-token")
			Expect(err).NotTo(HaveOccurred())

			_, ok := platform.Deploy.(switchblade.DockerDeployProcess)
			Expect(ok).To(BeTrue())

			_, ok = platform.Delete.(switchblade.DockerDeleteProcess)
			Expect(ok).To(BeTrue())
		})
	})

	context("when given an unknown platform type", func() {
		it("returns an error", func() {
			_, err := switchblade.NewPlatform("unknown", "some-token")
			Expect(err).To(MatchError(`unknown platform type: "unknown"`))
		})
	})
}
