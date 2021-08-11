package docker_test

import (
	gocontext "context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"github.com/ryanmoran/switchblade/internal/docker"
	"github.com/ryanmoran/switchblade/internal/docker/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testNetworkManager(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		client  *fakes.NetworkManagementClient
		manager docker.NetworkManager
	)

	it.Before(func() {
		client = &fakes.NetworkManagementClient{}

		manager = docker.NewNetworkManager(client)
	})

	context("Create", func() {
		it("creates the network", func() {
			ctx := gocontext.Background()

			err := manager.Create(ctx, "some-network", "some-driver", true)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.NetworkCreateCall.Receives.Ctx).To(Equal(ctx))
			Expect(client.NetworkCreateCall.Receives.Name).To(Equal("some-network"))
			Expect(client.NetworkCreateCall.Receives.Options).To(Equal(types.NetworkCreate{
				Driver:   "some-driver",
				Internal: true,
			}))
		})

		context("if the internal network already exists", func() {
			it.Before(func() {
				client.NetworkListCall.Returns.NetworkResourceSlice = []types.NetworkResource{
					{
						Name: "bridge",
						ID:   "bridge-network-id",
					},
					{
						Name: "some-network",
						ID:   "some-network-id",
					},
					{
						Name: "other-network",
						ID:   "other-network-id",
					},
				}
			})

			it("does not recreate the network", func() {
				ctx := gocontext.Background()

				err := manager.Create(ctx, "some-network", "some-driver", true)
				Expect(err).NotTo(HaveOccurred())

				Expect(client.NetworkListCall.CallCount).To(Equal(1))
				Expect(client.NetworkCreateCall.CallCount).To(Equal(0))
			})
		})

		context("failure cases", func() {
			context("when the client fails to list networks", func() {
				it.Before(func() {
					client.NetworkListCall.Returns.Error = errors.New("networks could not be listed")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()

					err := manager.Create(ctx, "some-network", "some-driver", true)
					Expect(err).To(MatchError("failed to list networks: networks could not be listed"))
				})
			})

			context("when the client fails to create a network", func() {
				it.Before(func() {
					client.NetworkCreateCall.Returns.Error = errors.New("network could not be created")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()

					err := manager.Create(ctx, "some-network", "some-driver", true)
					Expect(err).To(MatchError("failed to create network: network could not be created"))
				})
			})
		})
	})

	context("Connect", func() {
		it.Before(func() {
			client.NetworkListCall.Returns.NetworkResourceSlice = []types.NetworkResource{
				{
					Name: "bridge",
					ID:   "bridge-network-id",
				},
				{
					Name: "some-network",
					ID:   "some-network-id",
				},
				{
					Name: "other-network",
					ID:   "other-network-id",
				},
			}
		})

		it("connects the container to the named network", func() {
			ctx := gocontext.Background()

			err := manager.Connect(ctx, "some-container-id", "other-network")
			Expect(err).NotTo(HaveOccurred())

			Expect(client.NetworkListCall.CallCount).To(Equal(1))
			Expect(client.NetworkConnectCall.Receives.NetworkID).To(Equal("other-network-id"))
			Expect(client.NetworkConnectCall.Receives.ContainerID).To(Equal("some-container-id"))
		})

		context("when the named network does not exist", func() {
			it("returns an error", func() {
				ctx := gocontext.Background()

				err := manager.Connect(ctx, "some-container-id", "missing-network")
				Expect(err).To(MatchError("failed to connect container to network: no such network \"missing-network\""))

				Expect(client.NetworkListCall.CallCount).To(Equal(1))
				Expect(client.NetworkConnectCall.CallCount).To(Equal(0))
			})
		})

		context("failure cases", func() {
			context("when the client fails to list networks", func() {
				it.Before(func() {
					client.NetworkListCall.Returns.Error = errors.New("networks could not be listed")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()

					err := manager.Connect(ctx, "some-container-id", "some-network")
					Expect(err).To(MatchError("failed to list networks: networks could not be listed"))
				})
			})

			context("when the client fails to connect the network", func() {
				it.Before(func() {
					client.NetworkConnectCall.Returns.Error = errors.New("network could not be connected")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()

					err := manager.Connect(ctx, "some-container-id", "some-network")
					Expect(err).To(MatchError("failed to connect container to network: network could not be connected"))
				})
			})
		})
	})

	context("Delete", func() {
		it.Before(func() {
			client.NetworkListCall.Returns.NetworkResourceSlice = []types.NetworkResource{
				{
					Name: "bridge",
					ID:   "bridge-network-id",
				},
				{
					Name: "some-network",
					ID:   "some-network-id",
				},
				{
					Name: "other-network",
					ID:   "other-network-id",
				},
			}
		})

		it("deletes the network", func() {
			ctx := gocontext.Background()

			err := manager.Delete(ctx, "some-network")
			Expect(err).NotTo(HaveOccurred())

			Expect(client.NetworkListCall.CallCount).To(Equal(1))
			Expect(client.NetworkRemoveCall.Receives.NetworkID).To(Equal("some-network-id"))
		})

		context("when the named network does not exist", func() {
			it("does not error", func() {
				ctx := gocontext.Background()

				err := manager.Delete(ctx, "missing-network")
				Expect(err).NotTo(HaveOccurred())

				Expect(client.NetworkListCall.CallCount).To(Equal(1))
				Expect(client.NetworkConnectCall.CallCount).To(Equal(0))
			})
		})

		context("when the network still has containers attached", func() {
			it.Before(func() {
				client.NetworkRemoveCall.Returns.Error = errdefs.Forbidden(errors.New("containers still attached"))
			})

			it("does not error", func() {
				ctx := gocontext.Background()

				err := manager.Delete(ctx, "some-network")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		context("failure cases", func() {
			context("when the client fails to list networks", func() {
				it.Before(func() {
					client.NetworkListCall.Returns.Error = errors.New("networks could not be listed")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()

					err := manager.Delete(ctx, "some-network")
					Expect(err).To(MatchError("failed to list networks: networks could not be listed"))
				})
			})

			context("when the network cannot be removed", func() {
				it.Before(func() {
					client.NetworkRemoveCall.Returns.Error = errors.New("network could not be removed")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()

					err := manager.Delete(ctx, "some-network")
					Expect(err).To(MatchError("failed to delete network: network could not be removed"))
				})
			})
		})
	})
}
