package docker_test

import (
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

			registry *fakes.BPRegistry
		)

		it.Before(func() {
			registry = &fakes.BPRegistry{}

			initialize = docker.NewInitialize(registry)
		})

		it("overrides the buildpacks specified in the registry", func() {
			initialize.Run([]docker.Buildpack{
				{
					Name: "some-buildpack-name",
					URI:  "some-buildpack-uri",
				},
				{
					Name: "other-buildpack-name",
					URI:  "other-buildpack-uri",
				},
			})

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
		})
	})
}
