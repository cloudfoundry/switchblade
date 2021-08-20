package docker_test

import (
	gocontext "context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"github.com/ryanmoran/switchblade/internal/docker"
	"github.com/ryanmoran/switchblade/internal/docker/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testTeardown(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("Run", func() {
		var (
			teardown docker.Teardown

			client         *fakes.TeardownClient
			networkManager *fakes.TeardownNetworkManager
			workspace      string
		)

		it.Before(func() {
			client = &fakes.TeardownClient{}

			networkManager = &fakes.TeardownNetworkManager{}

			var err error
			workspace, err = os.MkdirTemp("", "workspace")
			Expect(err).NotTo(HaveOccurred())

			err = os.Mkdir(filepath.Join(workspace, "droplets"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(workspace, "droplets", "some-app.tar.gz"), []byte("some-droplet-contents"), 0600)
			Expect(err).NotTo(HaveOccurred())

			err = os.Mkdir(filepath.Join(workspace, "source"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(workspace, "source", "some-app.tar.gz"), []byte("some-source-contents"), 0600)
			Expect(err).NotTo(HaveOccurred())

			err = os.MkdirAll(filepath.Join(workspace, "buildpacks", "some-app"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(workspace, "buildpacks", "some-app.tar.gz"), []byte("some-buildpacks-contents"), 0600)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(workspace, "buildpacks", "some-app", "some-buildpack"), []byte("some-buildpack-file-contents"), 0600)
			Expect(err).NotTo(HaveOccurred())

			err = os.Mkdir(filepath.Join(workspace, "build-cache"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(workspace, "build-cache", "some-app.tar.gz"), []byte("some-build-cache-contents"), 0600)
			Expect(err).NotTo(HaveOccurred())

			teardown = docker.NewTeardown(client, networkManager, workspace)
		})

		it.After(func() {
			Expect(os.RemoveAll(workspace)).To(Succeed())
		})

		it("stops the app and cleans up its artifacts", func() {
			ctx := gocontext.Background()

			err := teardown.Run(ctx, "some-app")
			Expect(err).NotTo(HaveOccurred())

			Expect(client.ContainerRemoveCall.Receives.Ctx).To(Equal(ctx))
			Expect(client.ContainerRemoveCall.Receives.ContainerID).To(Equal("some-app"))
			Expect(client.ContainerRemoveCall.Receives.Options).To(Equal(types.ContainerRemoveOptions{
				Force: true,
			}))

			Expect(networkManager.DeleteCall.Receives.Name).To(Equal("switchblade-internal"))

			Expect(filepath.Join(workspace, "droplets", "some-app.tar.gz")).NotTo(BeAnExistingFile())
			Expect(filepath.Join(workspace, "source", "some-app.tar.gz")).NotTo(BeAnExistingFile())
			Expect(filepath.Join(workspace, "buildpacks", "some-app.tar.gz")).NotTo(BeAnExistingFile())
			Expect(filepath.Join(workspace, "buildpacks", "some-app", "some-buildpack")).NotTo(BeAnExistingFile())
			Expect(filepath.Join(workspace, "buildpacks", "some-app")).NotTo(BeADirectory())
			Expect(filepath.Join(workspace, "build-cache", "some-app.tar.gz")).NotTo(BeAnExistingFile())
		})

		context("when the container does not exist", func() {
			it.Before(func() {
				client.ContainerRemoveCall.Returns.Error = errdefs.NotFound(errors.New("no such container"))
			})

			it("does not error", func() {
				ctx := gocontext.Background()

				err := teardown.Run(ctx, "some-app")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		context("when the droplet tarball does not exist", func() {
			it.Before(func() {
				Expect(os.Remove(filepath.Join(workspace, "droplets", "some-app.tar.gz"))).To(Succeed())
			})

			it("does not error", func() {
				ctx := gocontext.Background()

				err := teardown.Run(ctx, "some-app")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		context("when the source tarball does not exist", func() {
			it.Before(func() {
				Expect(os.Remove(filepath.Join(workspace, "source", "some-app.tar.gz"))).To(Succeed())
			})

			it("does not error", func() {
				ctx := gocontext.Background()

				err := teardown.Run(ctx, "some-app")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		context("when the buildpack tarball does not exist", func() {
			it.Before(func() {
				Expect(os.Remove(filepath.Join(workspace, "buildpacks", "some-app.tar.gz"))).To(Succeed())
			})

			it("does not error", func() {
				ctx := gocontext.Background()

				err := teardown.Run(ctx, "some-app")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		context("when the buildpacks directory does not exist", func() {
			it.Before(func() {
				Expect(os.RemoveAll(filepath.Join(workspace, "buildpacks"))).To(Succeed())
			})

			it("does not error", func() {
				ctx := gocontext.Background()

				err := teardown.Run(ctx, "some-app")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		context("when the build cache tarball does not exist", func() {
			it.Before(func() {
				Expect(os.Remove(filepath.Join(workspace, "build-cache", "some-app.tar.gz"))).To(Succeed())
			})

			it("does not error", func() {
				ctx := gocontext.Background()

				err := teardown.Run(ctx, "some-app")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		context("failure cases", func() {
			context("when the container cannot be removed", func() {
				it.Before(func() {
					client.ContainerRemoveCall.Returns.Error = errors.New("could not remove container")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()

					err := teardown.Run(ctx, "some-app")
					Expect(err).To(MatchError("failed to remove container: could not remove container"))
				})
			})

			context("when the network cannot be delete", func() {
				it.Before(func() {
					networkManager.DeleteCall.Returns.Error = errors.New("could not delete network")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()

					err := teardown.Run(ctx, "some-app")
					Expect(err).To(MatchError("failed to delete network: could not delete network"))
				})
			})
		})
	})
}
