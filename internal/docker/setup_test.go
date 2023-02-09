package docker_test

import (
	"bytes"
	gocontext "context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/cloudfoundry/switchblade/internal/docker"
	"github.com/cloudfoundry/switchblade/internal/docker/fakes"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/errdefs"
	"github.com/sclevine/spec"

	. "github.com/cloudfoundry/switchblade/matchers"
	. "github.com/onsi/gomega"
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
			client.ContainerCreateCall.Returns.CreateResponse = container.CreateResponse{ID: "some-container-id"}
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
			client.ContainerInspectCall.Returns.Error = errdefs.NotFound(errors.New("no such container"))

			setup = docker.NewSetup(client, lifecycleBuilder, buildpacksBuilder, archiver, networkManager, workspace, "default-stack")
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

			Expect(client.ImagePullCall.Receives.Ref).To(Equal("cloudfoundry/default-stack:latest"))

			Expect(networkManager.CreateCall.Receives.Name).To(Equal("switchblade-internal"))
			Expect(networkManager.CreateCall.Receives.Driver).To(Equal("bridge"))
			Expect(networkManager.CreateCall.Receives.Internal).To(BeTrue())

			Expect(client.ContainerInspectCall.Receives.ContainerID).To(Equal("some-app"))
			Expect(client.ContainerRemoveCall.CallCount).To(Equal(0))

			Expect(client.ContainerCreateCall.Receives.Config).To(Equal(&container.Config{
				Image: "cloudfoundry/default-stack:latest",
				Cmd: []string{
					"/tmp/lifecycle/builder",
					"--buildArtifactsCacheDir=/tmp/cache",
					"--buildDir=/tmp/app",
					"--buildpackOrder=some-buildpack,other-buildpack",
					"--buildpacksDir=/tmp/buildpacks",
					"--outputBuildArtifactsCache=/tmp/output-cache",
					"--outputDroplet=/tmp/droplet",
					"--outputMetadata=/tmp/result.json",
					"--skipDetect=false",
				},
				User: "vcap",
				Env: []string{
					"CF_STACK=default-stack",
					"VCAP_SERVICES={}",
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
					"--buildArtifactsCacheDir=/tmp/cache",
					"--buildDir=/tmp/app",
					"--buildpackOrder=some-buildpack,other-buildpack",
					"--buildpacksDir=/tmp/buildpacks",
					"--outputBuildArtifactsCache=/tmp/output-cache",
					"--outputDroplet=/tmp/droplet",
					"--outputMetadata=/tmp/result.json",
					"--skipDetect=true",
				})))
				Expect(client.ContainerCreateCall.Receives.ContainerName).To(Equal("some-app"))
			})
		})

		context("WithStack", func() {
			it("builds using that stack", func() {
				ctx := gocontext.Background()
				logs := bytes.NewBuffer(nil)

				_, err := setup.
					WithStack("some-stack").
					Run(ctx, logs, "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				Expect(client.ImagePullCall.Receives.Ref).To(Equal("cloudfoundry/some-stack:latest"))
				Expect(client.ContainerCreateCall.Receives.Config.Image).To(Equal("cloudfoundry/some-stack:latest"))
				Expect(client.ContainerCreateCall.Receives.Config.Env).To(ContainElement(
					"CF_STACK=some-stack",
				))
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
					"CF_STACK=default-stack",
					"OTHER_KEY=other-value",
					"SOME_KEY=some-value",
					"VCAP_SERVICES={}",
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

		context("WithServices", func() {
			it("sets up VCAP_SERVICES with those services", func() {
				ctx := gocontext.Background()
				logs := bytes.NewBuffer(nil)

				_, err := setup.
					WithServices(map[string]map[string]interface{}{
						"some-service": map[string]interface{}{
							"some-key": "some-value",
						},
						"other-service": map[string]interface{}{
							"other-key": "other-value",
						},
					}).
					Run(ctx, logs, "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				Expect(client.ContainerCreateCall.Receives.Config.Env).To(ConsistOf([]string{
					"CF_STACK=default-stack",
					`VCAP_SERVICES={"user-provided":[{"credentials":{"other-key":"other-value"},"name":"some-app-other-service"},{"credentials":{"some-key":"some-value"},"name":"some-app-some-service"}]}`,
				}))
			})
		})

		context("when a conflicting container already exists", func() {
			it.Before(func() {
				client.ContainerInspectCall.Returns.ContainerJSON = types.ContainerJSON{ContainerJSONBase: &types.ContainerJSONBase{ID: "some-container-id"}}
				client.ContainerInspectCall.Returns.Error = nil
			})

			it("removes the conflicting container", func() {
				ctx := gocontext.Background()
				logs := bytes.NewBuffer(nil)

				containerID, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())
				Expect(containerID).To(Equal("some-container-id"))

				Expect(client.ContainerInspectCall.Receives.ContainerID).To(Equal("some-app"))
				Expect(client.ContainerRemoveCall.Receives.ContainerID).To(Equal("some-container-id"))
			})
		})

		context("when there is a build cache", func() {
			it.Before(func() {
				Expect(os.Mkdir(filepath.Join(workspace, "build-cache"), os.ModePerm)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(workspace, "build-cache", "some-app.tar.gz"), []byte("cache-content"), 0600)).To(Succeed())
			})

			it("copies the build cache into the container", func() {
				ctx := gocontext.Background()
				logs := bytes.NewBuffer(nil)

				containerID, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())
				Expect(containerID).To(Equal("some-container-id"))

				Expect(copyToContainerInvocations).To(HaveLen(4))
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
				Expect(copyToContainerInvocations[3]).To(Equal(copyToContainerInvocation{
					ContainerID: "some-container-id",
					DstPath:     "/",
					Content:     "cache-content",
				}))

				Expect(logs).To(ContainLines("Pulling image..."))
			})
		})

		context("failure cases", func() {
			context("when the lifecycle cannot be built", func() {
				it.Before(func() {
					lifecycleBuilder.BuildCall.Returns.Err = errors.New("could not build lifecycle")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError("failed to build lifecycle: could not build lifecycle"))
				})
			})

			context("when the buildpacks cannot be built", func() {
				it.Before(func() {
					buildpacksBuilder.BuildCall.Returns.Err = errors.New("could not build buildpacks")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError("failed to build buildpacks: could not build buildpacks"))
				})
			})

			context("when the source cannot be archived", func() {
				it.Before(func() {
					archiver.CompressCall.Returns.Error = errors.New("could not compress source")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError("failed to archive source code: could not compress source"))
				})
			})

			context("when the image cannot be pulled", func() {
				it.Before(func() {
					client.ImagePullCall.Returns.Error = errors.New("could not pull image")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError("failed to pull base image: could not pull image"))
				})
			})

			context("when the pull logs cannot be copied", func() {
				it.Before(func() {
					client.ImagePullCall.Returns.ReadCloser = io.NopCloser(iotest.ErrReader(errors.New("could not read logs")))
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError("failed to copy image pull logs: could not read logs"))
				})
			})

			context("when the network cannot be created", func() {
				it.Before(func() {
					networkManager.CreateCall.Returns.Error = errors.New("could not create network")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError("failed to create network: could not create network"))
				})
			})

			context("when service bindings cannot be marshalled to json", func() {
				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := setup.
						WithServices(map[string]map[string]interface{}{
							"some-service": map[string]interface{}{
								"some-key": func() {},
							},
						}).
						Run(ctx, logs, "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to marshal services json")))
					Expect(err).To(MatchError(ContainSubstring("unsupported type: func()")))
				})
			})

			context("when the buildpack order cannot be listed", func() {
				it.Before(func() {
					buildpacksBuilder.OrderCall.Returns.Err = errors.New("could not order buildpacks")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError("failed to determine buildpack ordering: could not order buildpacks"))
				})
			})

			context("when the container cannot be inspected", func() {
				it.Before(func() {
					client.ContainerInspectCall.Returns.Error = errors.New("could not inspect container")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError("failed to inspect staging container: could not inspect container"))
				})
			})

			context("when a conflicting container cannot be removed", func() {
				it.Before(func() {
					client.ContainerInspectCall.Returns.ContainerJSON = types.ContainerJSON{ContainerJSONBase: &types.ContainerJSONBase{ID: "some-container-id"}}
					client.ContainerInspectCall.Returns.Error = nil
					client.ContainerRemoveCall.Returns.Error = errors.New("could not remove container")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError("failed to remove conflicting container: could not remove container"))
				})
			})

			context("when the container cannot be created", func() {
				it.Before(func() {
					client.ContainerCreateCall.Returns.Error = errors.New("could not create container")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError("failed to create staging container: could not create container"))
				})
			})

			context("when the network cannot be connected", func() {
				it.Before(func() {
					networkManager.ConnectCall.Returns.Error = errors.New("could not connect network")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError("failed to connect container to network: could not connect network"))
				})
			})

			context("when the lifecycle cannot be opened", func() {
				it.Before(func() {
					Expect(os.Chmod(filepath.Join(workspace, "lifecycle", "lifecycle.tar.gz"), 0000)).To(Succeed())
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to open tarball:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when the lifecycle cannot be copied to the container", func() {
				it.Before(func() {
					client.CopyToContainerCall.Stub = func(ctx gocontext.Context, containerID, dstPath string, content io.Reader, options types.CopyToContainerOptions) error {
						b, err := io.ReadAll(content)
						if err != nil {
							return err
						}

						if strings.Contains(string(b), "lifecycle") {
							return errors.New("could not copy lifecycle to container")
						}

						return nil
					}
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(ctx, logs, "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError("failed to copy tarball to container: could not copy lifecycle to container"))
				})
			})
		})
	})
}
