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
	})
}
