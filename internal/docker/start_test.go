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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/ryanmoran/switchblade/internal/docker"
	"github.com/ryanmoran/switchblade/internal/docker/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testStart(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("Run", func() {
		var (
			start docker.Start

			client         *fakes.StartClient
			networkManager *fakes.StartNetworkManager
			workspace      string

			copyToContainerInvocations []copyToContainerInvocation
		)

		it.Before(func() {
			var err error
			workspace, err = os.MkdirTemp("", "workspace")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.MkdirAll(filepath.Join(workspace, "lifecycle"), os.ModePerm)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workspace, "lifecycle", "lifecycle.tar.gz"), []byte("lifecycle-content"), 0600)).To(Succeed())

			Expect(os.MkdirAll(filepath.Join(workspace, "droplets"), os.ModePerm)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workspace, "droplets", "some-app.tar.gz"), []byte("droplet-content"), 0600)).To(Succeed())

			client = &fakes.StartClient{}
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
			client.ContainerInspectCall.Returns.ContainerJSON = types.ContainerJSON{
				NetworkSettings: &types.NetworkSettings{
					NetworkSettingsBase: types.NetworkSettingsBase{
						Ports: nat.PortMap{
							"8080/tcp": []nat.PortBinding{
								{
									HostIP:   "::",
									HostPort: "12345",
								},
								{
									HostIP:   "0.0.0.0",
									HostPort: "12345",
								},
							},
						},
					},
					Networks: map[string]*network.EndpointSettings{
						"switchblade-internal": {
							IPAddress: "172.19.0.2",
						},
					},
				},
			}

			networkManager = &fakes.StartNetworkManager{}

			start = docker.NewStart(client, networkManager, workspace)
		})

		it.After(func() {
			Expect(os.RemoveAll(workspace)).To(Succeed())
		})

		it("starts the droplet", func() {
			ctx := gocontext.Background()
			logs := bytes.NewBuffer(nil)

			externalURL, internalURL, err := start.Run(ctx, logs, "some-app", "some-command")
			Expect(err).NotTo(HaveOccurred())
			Expect(externalURL).To(Equal("http://0.0.0.0:12345"))
			Expect(internalURL).To(Equal("http://172.19.0.2:8080"))

			Expect(client.ContainerCreateCall.Receives.ContainerName).To(Equal("some-app"))
			Expect(client.ContainerCreateCall.Receives.Config).To(Equal(&container.Config{
				Image: "cloudfoundry/cflinuxfs3:latest",
				Cmd: []string{
					"/tmp/lifecycle/launcher",
					"app",
					"some-command",
					"",
				},
				User: "vcap",
				Env: []string{
					"LANG=en_US.UTF-8",
					"MEMORY_LIMIT=1024m",
					"PORT=8080",
					`VCAP_APPLICATION={"application_name":"some-app","name":"some-app","process_type":"web"}`,
					"VCAP_PLATFORM_OPTIONS={}",
					"VCAP_SERVICES={}",
				},
				WorkingDir: "/home/vcap",
				ExposedPorts: nat.PortSet{
					"8080/tcp": struct{}{},
				},
			}))
			Expect(client.ContainerCreateCall.Receives.HostConfig).To(Equal(&container.HostConfig{
				PublishAllPorts: true,
				NetworkMode:     container.NetworkMode("switchblade-internal"),
			}))

			Expect(networkManager.ConnectCall.Receives.ContainerID).To(Equal("some-container-id"))
			Expect(networkManager.ConnectCall.Receives.Name).To(Equal("bridge"))

			Expect(copyToContainerInvocations).To(HaveLen(2))
			Expect(copyToContainerInvocations[0]).To(Equal(copyToContainerInvocation{
				ContainerID: "some-container-id",
				DstPath:     "/",
				Content:     "lifecycle-content",
			}))
			Expect(copyToContainerInvocations[1]).To(Equal(copyToContainerInvocation{
				ContainerID: "some-container-id",
				DstPath:     "/home/vcap/",
				Content:     "droplet-content",
			}))

			Expect(client.ContainerStartCall.Receives.ContainerID).To(Equal("some-container-id"))

			Expect(client.ContainerInspectCall.Receives.ContainerID).To(Equal("some-container-id"))
		})

		context("WithEnv", func() {
			it("sets the environment for the container", func() {
				ctx := gocontext.Background()
				logs := bytes.NewBuffer(nil)

				_, _, err := start.
					WithEnv(map[string]string{
						"SOME_KEY":  "some-value",
						"OTHER_KEY": "other-value",
					}).
					Run(ctx, logs, "some-app", "some-command")
				Expect(err).NotTo(HaveOccurred())

				Expect(client.ContainerCreateCall.Receives.Config.Env).To(ConsistOf([]string{
					"LANG=en_US.UTF-8",
					"MEMORY_LIMIT=1024m",
					"OTHER_KEY=other-value",
					"PORT=8080",
					"SOME_KEY=some-value",
					"VCAP_PLATFORM_OPTIONS={}",
					"VCAP_SERVICES={}",
					`VCAP_APPLICATION={"application_name":"some-app","name":"some-app","process_type":"web"}`,
				}))
			})
		})

		context("WithServices", func() {
			it("sets up VCAP_SERVICES with those services", func() {
				ctx := gocontext.Background()
				logs := bytes.NewBuffer(nil)

				_, _, err := start.
					WithServices(map[string]map[string]interface{}{
						"some-service": map[string]interface{}{
							"some-key": "some-value",
						},
						"other-service": map[string]interface{}{
							"other-key": "other-value",
						},
					}).
					Run(ctx, logs, "some-app", "some-command")
				Expect(err).NotTo(HaveOccurred())

				Expect(client.ContainerCreateCall.Receives.Config.Env).To(ConsistOf([]string{
					"LANG=en_US.UTF-8",
					"MEMORY_LIMIT=1024m",
					"PORT=8080",
					"VCAP_PLATFORM_OPTIONS={}",
					`VCAP_SERVICES={"user-provided":[{"credentials":{"other-key":"other-value"},"name":"some-app-other-service"},{"credentials":{"some-key":"some-value"},"name":"some-app-some-service"}]}`,
					`VCAP_APPLICATION={"application_name":"some-app","name":"some-app","process_type":"web"}`,
				}))
			})
		})

		context("failure cases", func() {
			context("when service bindings cannot be marshalled to json", func() {
				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, _, err := start.
						WithServices(map[string]map[string]interface{}{
							"some-service": map[string]interface{}{
								"some-key": func() {},
							},
						}).
						Run(ctx, logs, "some-app", "some-command")
					Expect(err).To(MatchError(ContainSubstring("failed to marshal services json")))
					Expect(err).To(MatchError(ContainSubstring("unsupported type: func()")))
				})
			})

			context("when the container cannot be created", func() {
				it.Before(func() {
					client.ContainerCreateCall.Returns.Error = errors.New("could not create container")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, _, err := start.Run(ctx, logs, "some-app", "some-command")
					Expect(err).To(MatchError("failed to create running container: could not create container"))
				})
			})

			context("when the network cannot be connected", func() {
				it.Before(func() {
					networkManager.ConnectCall.Returns.Error = errors.New("could not connect network")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, _, err := start.Run(ctx, logs, "some-app", "some-command")
					Expect(err).To(MatchError("failed to connect container to network: could not connect network"))
				})
			})

			context("when the lifecycle cannot be read", func() {
				it.Before(func() {
					Expect(os.Chmod(filepath.Join(workspace, "lifecycle", "lifecycle.tar.gz"), 0000)).To(Succeed())
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, _, err := start.Run(ctx, logs, "some-app", "some-command")
					Expect(err).To(MatchError(ContainSubstring("failed to open lifecycle:")))
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
							return errors.New("could not copy lifecycle")
						}

						return nil
					}
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, _, err := start.Run(ctx, logs, "some-app", "some-command")
					Expect(err).To(MatchError("failed to copy lifecycle into container: could not copy lifecycle"))
				})
			})

			context("when the droplet cannot be read", func() {
				it.Before(func() {
					Expect(os.Chmod(filepath.Join(workspace, "droplets", "some-app.tar.gz"), 0000)).To(Succeed())
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, _, err := start.Run(ctx, logs, "some-app", "some-command")
					Expect(err).To(MatchError(ContainSubstring("failed to open droplet:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when the droplet cannot be copied to the container", func() {
				it.Before(func() {
					client.CopyToContainerCall.Stub = func(ctx gocontext.Context, containerID, dstPath string, content io.Reader, options types.CopyToContainerOptions) error {
						b, err := io.ReadAll(content)
						if err != nil {
							return err
						}

						if strings.Contains(string(b), "droplet") {
							return errors.New("could not copy droplet")
						}

						return nil
					}
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, _, err := start.Run(ctx, logs, "some-app", "some-command")
					Expect(err).To(MatchError("failed to copy droplet into container: could not copy droplet"))
				})
			})

			context("when the container cannot be started", func() {
				it.Before(func() {
					client.ContainerStartCall.Returns.Error = errors.New("could not start container")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, _, err := start.Run(ctx, logs, "some-app", "some-command")
					Expect(err).To(MatchError("failed to start container: could not start container"))
				})
			})

			context("when the container cannot be inspected", func() {
				it.Before(func() {
					client.ContainerInspectCall.Returns.Error = errors.New("could not inspect container")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, _, err := start.Run(ctx, logs, "some-app", "some-command")
					Expect(err).To(MatchError("failed to inspect container: could not inspect container"))
				})
			})
		})
	})
}
