package docker_test

import (
	"errors"
	"testing"

	"github.com/cloudfoundry/switchblade/internal/docker"
	"github.com/cloudfoundry/switchblade/internal/docker/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDeinitialize(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("Run", func() {
		var (
			deinitialize docker.Deinitialize

			networkManager *fakes.DeinitializeNetworkManager
		)

		it.Before(func() {
			networkManager = &fakes.DeinitializeNetworkManager{}

			deinitialize = docker.NewDeinitialize(networkManager)
		})

		it("deletes the internal switchblade network", func() {
			err := deinitialize.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(networkManager.DeleteCall.Receives.Name).To(Equal("switchblade-internal"))
		})

		context("failure cases", func() {
			context("when the network cannot be deleted", func() {
				it.Before(func() {
					networkManager.DeleteCall.Returns.Error = errors.New("could not delete network")
				})

				it("returns an error", func() {
					err := deinitialize.Run()
					Expect(err).To(MatchError("failed to delete network: could not delete network"))
				})
			})
		})
	})
}
