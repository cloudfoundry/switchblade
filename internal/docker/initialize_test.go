package docker_test

import (
	"errors"
	"testing"

	"github.com/cloudfoundry/switchblade/internal/docker"
	"github.com/cloudfoundry/switchblade/internal/docker/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testInitialize(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("Run", func() {
		var (
			initialize docker.Initialize

			registry       *fakes.BPRegistry
			networkManager *fakes.InitializeNetworkManager
		)

		it.Before(func() {
			registry = &fakes.BPRegistry{}
			networkManager = &fakes.InitializeNetworkManager{}

			initialize = docker.NewInitialize(registry, networkManager)
		})

		it("overrides the buildpacks specified in the registry and creates the internal switchblade network", func() {
			err := initialize.Run([]docker.Buildpack{
				{
					Name: "some-buildpack-name",
					URI:  "some-buildpack-uri",
				},
				{
					Name: "other-buildpack-name",
					URI:  "other-buildpack-uri",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(registry.OverrideCall.Receives.BuildpackSlice).To(Equal([]docker.Buildpack{
				{
					Name: "some-buildpack-name",
					URI:  "some-buildpack-uri",
				},
				{
					Name: "other-buildpack-name",
					URI:  "other-buildpack-uri",
				},
			}))

			Expect(networkManager.CreateCall.Receives.Name).To(Equal("switchblade-internal"))
			Expect(networkManager.CreateCall.Receives.Driver).To(Equal("bridge"))
			Expect(networkManager.CreateCall.Receives.Internal).To(BeTrue())
		})

		context("failure cases", func() {
			context("when the network cannot be created", func() {
				it.Before(func() {
					networkManager.CreateCall.Returns.Error = errors.New("could not create network")
				})

				it("returns an error", func() {
					err := initialize.Run([]docker.Buildpack{
						{
							Name: "some-buildpack-name",
							URI:  "some-buildpack-uri",
						},
						{
							Name: "other-buildpack-name",
							URI:  "other-buildpack-uri",
						},
					})
					Expect(err).To(MatchError("failed to create network: could not create network"))
				})
			})
		})
	})
}
