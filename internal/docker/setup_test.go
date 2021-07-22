package docker_test

import (
	"bytes"
	gocontext "context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/ryanmoran/switchblade/internal/docker"
	"github.com/ryanmoran/switchblade/internal/docker/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/ryanmoran/switchblade/matchers"
)

func testSetup(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("Run", func() {
		var (
			setup docker.Setup

			client            *fakes.SetupClient
			lifecycleBuilder  *fakes.LifecycleBuilder
			buildpacksBuilder *fakes.BuildpacksBuilder
			archiver          *fakes.Archiver
			networkManager    *fakes.SetupNetworkManager
			workspace         string

			copyToContainerInvocations []copyToContainerInvocation
		)

		it.Before(func() {
			var err error
			workspace, err = os.MkdirTemp("", "workspace")
			Expect(err).NotTo(HaveOccurred())

			lifecycleBuilder = &fakes.LifecycleBuilder{}
			lifecycleBuilder.BuildCall.Returns.Path = filepath.Join(workspace, "lifecycle", "lifecycle.tar.gz")
			Expect(os.MkdirAll(filepath.Join(workspace, "lifecycle"), os.ModePerm)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workspace, "lifecycle", "lifecycle.tar.gz"), []byte("lifecycle-content"), 0600)).To(Succeed())

			buildpacksBuilder = &fakes.BuildpacksBuilder{}
			buildpacksBuilder.BuildCall.Returns.Path = filepath.Join(workspace, "buildpacks", "some-app.tar.gz")
			Expect(os.MkdirAll(filepath.Join(workspace, "buildpacks"), os.ModePerm)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workspace, "buildpacks", "some-app.tar.gz"), []byte("buildpacks-content"), 0600)).To(Succeed())

			buildpacksBuilder.OrderCall.Returns.Order = "some-buildpack,other-buildpack"

			archiver = &fakes.Archiver{}
			archiver.WithPrefixCall.Returns.Archiver = archiver
			Expect(os.MkdirAll(filepath.Join(workspace, "source"), os.ModePerm)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workspace, "source", "some-app.tar.gz"), []byte("app-content"), 0600)).To(Succeed())

			networkManager = &fakes.SetupNetworkManager{}

			client = &fakes.SetupClient{}
			client.ImagePullCall.Returns.ReadCloser = io.NopCloser(bytes.NewBuffer([]byte("Pulling image...\n")))
			client.ContainerCreateCall.Returns.ContainerCreateCreatedBody = container.ContainerCreateCreatedBody{ID: "some-container-id"}
			client.CopyToContainerCall.Stub = func(ctx gocontext.Context, containerID, dstPath string, content io.Reader, options types.CopyToContainerOptions) error {
				b, err := io.ReadAll(content)
				if err != nil {
					return err
				}

				copyToContainerInvocations = append(copyToContainerInvocations, copyToContainerInvocation{
					ContainerID: containerID,
					DstPath:     dstPath,
					Content:     string(b),
				})

				return nil
			}

			setup = docker.NewSetup(client, lifecycleBuilder, buildpacksBuilder, archiver, networkManager, workspace)
		})

		it.After(func() {
			Expect(os.RemoveAll(workspace)).To(Succeed())
		})

		it("sets up the staging container", func() {
			ctx := gocontext.Background()
			logs := bytes.NewBuffer(nil)

			containerID, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
			Expect(err).NotTo(HaveOccurred())
			Expect(containerID).To(Equal("some-container-id"))

			Expect(lifecycleBuilder.BuildCall.Receives.SourceURI).To(Equal("https://github.com/cloudfoundry/buildpackapplifecycle/archive/refs/heads/master.zip"))
			Expect(lifecycleBuilder.BuildCall.Receives.Workspace).To(Equal(filepath.Join(workspace, "lifecycle")))

			Expect(archiver.WithPrefixCall.Receives.Prefix).To(Equal("/tmp/app"))
			Expect(archiver.CompressCall.Receives.Input).To(Equal("/some/path/to/my/app"))
			Expect(archiver.CompressCall.Receives.Output).To(Equal(filepath.Join(workspace, "source", "some-app.tar.gz")))

			Expect(buildpacksBuilder.BuildCall.Receives.Workspace).To(Equal(filepath.Join(workspace, "buildpacks")))
			Expect(buildpacksBuilder.BuildCall.Receives.Name).To(Equal("some-app"))

			Expect(client.ImagePullCall.Receives.Ref).To(Equal("cloudfoundry/cflinuxfs3:latest"))

			Expect(networkManager.CreateCall.Receives.Name).To(Equal("switchblade-internal"))
			Expect(networkManager.CreateCall.Receives.Driver).To(Equal("bridge"))
			Expect(networkManager.CreateCall.Receives.Internal).To(BeTrue())

			Expect(client.ContainerCreateCall.Receives.Config).To(Equal(&container.Config{
				Image: "cloudfoundry/cflinuxfs3:latest",
				Cmd: []string{
					"/tmp/lifecycle/builder",
					"--buildpackOrder=some-buildpack,other-buildpack",
					"--skipDetect=false",
					"--buildDir=/tmp/app",
					"--outputDroplet=/tmp/droplet",
					"--outputMetadata=/tmp/result.json",
					"--buildpacksDir=/tmp/buildpacks",
				},
				User: "vcap",
				Env: []string{
					"CF_STACK=cflinuxfs3",
				},
				WorkingDir: "/home/vcap",
			}))
			Expect(client.ContainerCreateCall.Receives.HostConfig).To(Equal(&container.HostConfig{
				NetworkMode: container.NetworkMode("switchblade-internal"),
			}))
			Expect(client.ContainerCreateCall.Receives.ContainerName).To(Equal("some-app"))

			Expect(networkManager.ConnectCall.Receives.ContainerID).To(Equal("some-container-id"))
			Expect(networkManager.ConnectCall.Receives.Name).To(Equal("bridge"))

			Expect(copyToContainerInvocations).To(HaveLen(3))
			Expect(copyToContainerInvocations[0]).To(Equal(copyToContainerInvocation{
				ContainerID: "some-container-id",
				DstPath:     "/",
				Content:     "lifecycle-content",
			}))
			Expect(copyToContainerInvocations[1]).To(Equal(copyToContainerInvocation{
				ContainerID: "some-container-id",
				DstPath:     "/",
				Content:     "buildpacks-content",
			}))
			Expect(copyToContainerInvocations[2]).To(Equal(copyToContainerInvocation{
				ContainerID: "some-container-id",
				DstPath:     "/",
				Content:     "app-content",
			}))

			Expect(logs).To(ContainLines("Pulling image..."))
		})

		context("WithBuildpacks", func() {
			it.Before(func() {
				buildpacksBuilder.WithBuildpacksCall.Returns.BuildpacksBuilder = buildpacksBuilder
				buildpacksBuilder.OrderCall.Returns.SkipDetect = true
			})

			it("only builds the specified buildpacks", func() {
				ctx := gocontext.Background()
				logs := bytes.NewBuffer(nil)

				_, err := setup.
					WithBuildpacks("some-buildpack", "other-buildpack").
					Run(ctx, logs, "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				Expect(buildpacksBuilder.WithBuildpacksCall.Receives.Buildpacks).To(Equal([]string{"some-buildpack", "other-buildpack"}))

				Expect(client.ContainerCreateCall.Receives.Config.Cmd).To(Equal(strslice.StrSlice([]string{
					"/tmp/lifecycle/builder",
					"--buildpackOrder=some-buildpack,other-buildpack",
					"--skipDetect=true",
					"--buildDir=/tmp/app",
					"--outputDroplet=/tmp/droplet",
					"--outputMetadata=/tmp/result.json",
					"--buildpacksDir=/tmp/buildpacks",
				})))
				Expect(client.ContainerCreateCall.Receives.ContainerName).To(Equal("some-app"))
			})
		})

		context("WithEnv", func() {
			it("sets the environment for the container", func() {
				ctx := gocontext.Background()
				logs := bytes.NewBuffer(nil)

				_, err := setup.
					WithEnv(map[string]string{
						"SOME_KEY":  "some-value",
						"OTHER_KEY": "other-value",
					}).
					Run(ctx, logs, "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				Expect(client.ContainerCreateCall.Receives.Config.Env).To(ConsistOf([]string{
					"CF_STACK=cflinuxfs3",
					"OTHER_KEY=other-value",
					"SOME_KEY=some-value",
				}))
			})
		})

		context("WithoutInternetAccess", func() {
			it("does not connect the container to the internet", func() {
				ctx := gocontext.Background()
				logs := bytes.NewBuffer(nil)

				_, err := setup.
					WithoutInternetAccess().
					Run(ctx, logs, "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				Expect(networkManager.ConnectCall.CallCount).To(Equal(0))
			})
		})
	})
}
