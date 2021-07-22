package switchblade_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryanmoran/switchblade"
	"github.com/ryanmoran/switchblade/fakes"
	"github.com/ryanmoran/switchblade/internal/cloudfoundry"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/ryanmoran/switchblade/matchers"
)

func testCloudFoundry(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		initialize *fakes.CloudFoundryInitializePhase
		setup      *fakes.CloudFoundrySetupPhase
		stage      *fakes.CloudFoundryStagePhase
		teardown   *fakes.CloudFoundryTeardownPhase
		workspace  string

		platform switchblade.Platform
	)

	it.Before(func() {
		initialize = &fakes.CloudFoundryInitializePhase{}
		setup = &fakes.CloudFoundrySetupPhase{}
		stage = &fakes.CloudFoundryStagePhase{}
		teardown = &fakes.CloudFoundryTeardownPhase{}

		var err error
		workspace, err = os.MkdirTemp("", "workspace")
		Expect(err).NotTo(HaveOccurred())

		platform = switchblade.NewCloudFoundry(initialize, setup, stage, teardown, workspace)
	})

	it.After(func() {
		Expect(os.RemoveAll(workspace)).To(Succeed())
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
			Expect(initialize.RunCall.Receives.BuildpackSlice).To(Equal([]cloudfoundry.Buildpack{
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

		context("when the initialize phase errors", func() {
			it.Before(func() {
				initialize.RunCall.Returns.Error = errors.New("failed to initialize")
			})

			it("returns an error", func() {
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
				Expect(err).To(MatchError("failed to initialize"))
			})
		})
	})

	context("Deploy", func() {
		var home string

		it.Before(func() {
			var err error
			home, err = os.MkdirTemp("", "home")
			Expect(err).NotTo(HaveOccurred())

			setup.RunCall.Stub = func(logs io.Writer, home, name, source string) error {
				fmt.Fprintln(logs, "Setting up...")
				return nil
			}

			stage.RunCall.Stub = func(logs io.Writer, home, name string) (string, error) {
				fmt.Fprintln(logs, "Staging...")
				return "some-url", nil
			}
		})

		it.After(func() {
			Expect(os.RemoveAll(home)).To(Succeed())
		})

		it("executes the setup and stage phases", func() {
			deployment, logs, err := platform.Deploy.Execute("some-app", "/some/path/to/my/app")
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment).To(Equal(switchblade.Deployment{
				Name:        "some-app",
				ExternalURL: "some-url",
				InternalURL: "some-url",
			}))
			Expect(logs).To(ContainLines(
				"Setting up...",
				"Staging...",
			))

			Expect(setup.RunCall.Receives.Logs).To(Equal(logs))
			Expect(setup.RunCall.Receives.Home).To(Equal(filepath.Join(workspace, "some-app")))
			Expect(setup.RunCall.Receives.Name).To(Equal("some-app"))
			Expect(setup.RunCall.Receives.Name).To(Equal("some-app"))

			Expect(stage.RunCall.Receives.Logs).To(Equal(logs))
			Expect(stage.RunCall.Receives.Home).To(Equal(filepath.Join(workspace, "some-app")))
			Expect(stage.RunCall.Receives.Name).To(Equal("some-app"))
		})

		context("WithBuildpacks", func() {
			it("uses those buildpacks", func() {
				platform.Deploy.WithBuildpacks("some-buildpack", "other-buildpack")
				Expect(setup.WithBuildpacksCall.Receives.Buildpacks).To(Equal([]string{"some-buildpack", "other-buildpack"}))
			})
		})

		context("WithEnv", func() {
			it("uses those environment variables", func() {
				platform.Deploy.WithEnv(map[string]string{"SOME_KEY": "some-value"})
				Expect(setup.WithEnvCall.Receives.Env).To(Equal(map[string]string{"SOME_KEY": "some-value"}))
			})
		})

		context("WithoutInternetAccess", func() {
			it("ensures the app does not have internet access", func() {
				platform.Deploy.WithoutInternetAccess()
				Expect(setup.WithoutInternetAccessCall.CallCount).To(Equal(1))
			})
		})

		context("failure cases", func() {
			context("when the setup phase errors", func() {
				it.Before(func() {
					setup.RunCall.Stub = func(logs io.Writer, home, name, source string) error {
						fmt.Fprintln(logs, "Setting up... errored")
						return errors.New("failed to setup")
					}
				})

				it("returns an error", func() {
					_, logs, err := platform.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError("failed to setup"))
					Expect(logs).To(ContainLines("Setting up... errored"))
				})
			})

			context("when the stage phase errors", func() {
				it.Before(func() {
					stage.RunCall.Stub = func(logs io.Writer, home, name string) (string, error) {
						fmt.Fprintln(logs, "Staging... errored")
						return "some-url", errors.New("failed to stage")
					}
				})

				it("returns an error", func() {
					_, logs, err := platform.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError("failed to stage"))
					Expect(logs).To(ContainLines(
						"Setting up...",
						"Staging... errored",
					))
				})
			})
		})
	})

	context("Delete", func() {
		it("deletes the org, security-group, and config", func() {
			err := platform.Delete.Execute("some-app")
			Expect(err).NotTo(HaveOccurred())

			Expect(teardown.RunCall.Receives.Home).To(Equal(filepath.Join(workspace, "some-app")))
			Expect(teardown.RunCall.Receives.Name).To(Equal("some-app"))
		})

		context("failure cases", func() {
			context("when the teardown phase errors", func() {
				it.Before(func() {
					teardown.RunCall.Returns.Error = errors.New("failed to teardown")
				})

				it("returns an error", func() {
					err := platform.Delete.Execute("some-app")
					Expect(err).To(MatchError("failed to teardown"))
				})
			})
		})
	})
}
