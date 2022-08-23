package switchblade_test

import (
	gocontext "context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/cloudfoundry/switchblade"
	"github.com/cloudfoundry/switchblade/fakes"
	"github.com/cloudfoundry/switchblade/internal/docker"
	"github.com/sclevine/spec"

	. "github.com/cloudfoundry/switchblade/matchers"
	. "github.com/onsi/gomega"
)

func testDocker(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		platform switchblade.Platform

		initialize *fakes.DockerInitializePhase
		setup      *fakes.DockerSetupPhase
		stage      *fakes.DockerStagePhase
		start      *fakes.DockerStartPhase
		teardown   *fakes.DockerTeardownPhase
	)

	it.Before(func() {
		initialize = &fakes.DockerInitializePhase{}
		setup = &fakes.DockerSetupPhase{}
		stage = &fakes.DockerStagePhase{}
		start = &fakes.DockerStartPhase{}
		teardown = &fakes.DockerTeardownPhase{}

		platform = switchblade.NewDocker(initialize, setup, stage, start, teardown)
	})

	context("Initialize", func() {
		it("initializes the buildpacks", func() {
			err := platform.Initialize(
				switchblade.Buildpack{
					Name: "some-buildpack-name",
					URI:  "some-buildpack-uri",
				},
				switchblade.Buildpack{
					Name: "other-buildpack-name",
					URI:  "other-buildpack-uri",
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(initialize.RunCall.Receives.BuildpackSlice).To(Equal([]docker.Buildpack{
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

	context("Deploy", func() {
		it.Before(func() {
			setup.RunCall.Stub = func(ctx gocontext.Context, logs io.Writer, name, path string) (string, error) {
				fmt.Fprintln(logs, "Setting up...")
				return "some-container-id", nil
			}

			stage.RunCall.Stub = func(ctx gocontext.Context, logs io.Writer, containerID, name string) (string, error) {
				fmt.Fprintln(logs, "Staging...")
				return "some-command", nil
			}

			start.RunCall.Stub = func(ctx gocontext.Context, logs io.Writer, name, command string) (string, string, error) {
				fmt.Fprintln(logs, "Starting...")
				return "some-external-url", "some-internal-url", nil
			}
		})

		it("builds and runs the app", func() {
			deployment, logs, err := platform.Deploy.Execute("some-app", "/some/path/to/my/app")
			Expect(err).NotTo(HaveOccurred())

			Expect(logs).To(ContainLines(
				"Setting up...",
				"Staging...",
				"Starting...",
			))
			Expect(deployment).To(Equal(switchblade.Deployment{
				Name:        "some-app",
				ExternalURL: "some-external-url",
				InternalURL: "some-internal-url",
			}))

			Expect(setup.RunCall.Receives.Ctx).To(Equal(gocontext.Background()))
			Expect(setup.RunCall.Receives.Logs).To(Equal(logs))
			Expect(setup.RunCall.Receives.Name).To(Equal("some-app"))
			Expect(setup.RunCall.Receives.Path).To(Equal("/some/path/to/my/app"))

			Expect(stage.RunCall.Receives.Ctx).To(Equal(gocontext.Background()))
			Expect(stage.RunCall.Receives.Logs).To(Equal(logs))
			Expect(stage.RunCall.Receives.ContainerID).To(Equal("some-container-id"))
			Expect(stage.RunCall.Receives.Name).To(Equal("some-app"))

			Expect(start.RunCall.Receives.Ctx).To(Equal(gocontext.Background()))
			Expect(start.RunCall.Receives.Logs).To(Equal(logs))
			Expect(start.RunCall.Receives.Name).To(Equal("some-app"))
			Expect(start.RunCall.Receives.Command).To(Equal("some-command"))
		})

		context("WithBuildpacks", func() {
			it("uses those buildpacks", func() {
				platform.Deploy.WithBuildpacks("some-buildpack", "other-buildpack")
				Expect(setup.WithBuildpacksCall.Receives.Buildpacks).To(Equal([]string{"some-buildpack", "other-buildpack"}))
			})
		})

		context("WithStack", func() {
			it("uses that stack", func() {
				platform.Deploy.WithStack("some-stack")
				Expect(setup.WithStackCall.Receives.Stack).To(Equal("some-stack"))
			})
		})

		context("WithEnv", func() {
			it("uses those environment variables", func() {
				platform.Deploy.WithEnv(map[string]string{"SOME_KEY": "some-value"})
				Expect(setup.WithEnvCall.Receives.Env).To(Equal(map[string]string{"SOME_KEY": "some-value"}))
				Expect(start.WithEnvCall.Receives.Env).To(Equal(map[string]string{"SOME_KEY": "some-value"}))
			})
		})

		context("WithoutInternetAccess", func() {
			it("ensures the app does not have internet access", func() {
				platform.Deploy.WithoutInternetAccess()
				Expect(setup.WithoutInternetAccessCall.CallCount).To(Equal(1))
			})
		})

		context("WithServices", func() {
			it("provides those services during setup and start", func() {
				platform.Deploy.WithServices(map[string]switchblade.Service{
					"some-service": {
						"some-key": "some-value",
					},
				})
				Expect(setup.WithServicesCall.Receives.Services).To(Equal(map[string]map[string]interface{}{
					"some-service": {
						"some-key": "some-value",
					},
				}))
				Expect(start.WithServicesCall.Receives.Services).To(Equal(map[string]map[string]interface{}{
					"some-service": {
						"some-key": "some-value",
					},
				}))
			})
		})

		context("failure cases", func() {
			context("when the setup phase errors", func() {
				it.Before(func() {
					setup.RunCall.Stub = func(ctx gocontext.Context, logs io.Writer, name, path string) (string, error) {
						fmt.Fprintln(logs, "Setting up...")
						return "", errors.New("setup phase errored")
					}
				})

				it("returns an error and the build logs", func() {
					_, logs, err := platform.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to run setup phase: setup phase errored")))
					Expect(err).To(MatchError(ContainSubstring("Setting up...")))
					Expect(logs).To(ContainLines(
						"Setting up...",
					))
					Expect(logs).NotTo(ContainLines(
						"Staging...",
						"Starting...",
					))
				})
			})

			context("when the stage phase errors", func() {
				it.Before(func() {
					stage.RunCall.Stub = func(ctx gocontext.Context, logs io.Writer, containerID, name string) (string, error) {
						fmt.Fprintln(logs, "Staging...")
						return "", errors.New("stage phase errored")
					}
				})

				it("returns an error and the build logs", func() {
					_, logs, err := platform.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to run stage phase: stage phase errored")))
					Expect(err).To(MatchError(ContainSubstring("Staging...")))
					Expect(logs).To(ContainLines(
						"Setting up...",
						"Staging...",
					))
					Expect(logs).NotTo(ContainLines(
						"Starting...",
					))
				})
			})

			context("when the start phase errors", func() {
				it.Before(func() {
					start.RunCall.Stub = func(ctx gocontext.Context, logs io.Writer, name, command string) (string, string, error) {
						fmt.Fprintln(logs, "Starting...")
						return "", "", errors.New("start phase errored")
					}
				})

				it("returns an error and the build logs", func() {
					_, logs, err := platform.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to run start phase: start phase errored")))
					Expect(err).To(MatchError(ContainSubstring("Starting...")))
					Expect(logs).To(ContainLines(
						"Setting up...",
						"Staging...",
						"Starting...",
					))
				})
			})
		})
	})

	context("Delete", func() {
		it("deletes the app", func() {
			err := platform.Delete.Execute("some-app")
			Expect(err).NotTo(HaveOccurred())

			Expect(teardown.RunCall.Receives.Ctx).To(Equal(gocontext.Background()))
			Expect(teardown.RunCall.Receives.Name).To(Equal("some-app"))
		})

		context("failure cases", func() {
			context("when the teardown phase errors", func() {
				it.Before(func() {
					teardown.RunCall.Stub = nil
					teardown.RunCall.Returns.Error = errors.New("teardown phase errored")
				})

				it("returns an error", func() {
					err := platform.Delete.Execute("some-app")
					Expect(err).To(MatchError("failed to run teardown phase: teardown phase errored"))
				})
			})
		})
	})
}
