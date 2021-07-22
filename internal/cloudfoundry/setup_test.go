package cloudfoundry_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/ryanmoran/switchblade/internal/cloudfoundry"
	"github.com/ryanmoran/switchblade/internal/cloudfoundry/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	. "github.com/ryanmoran/switchblade/matchers"
)

func testSetup(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("Run", func() {
		var (
			setup cloudfoundry.Setup

			executable      *fakes.Executable
			workspace, home string

			executions []pexec.Execution
		)

		it.Before(func() {
			executable = &fakes.Executable{}
			executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
				executions = append(executions, execution)

				command := strings.Join(execution.Args, " ")
				switch {
				case strings.HasPrefix(command, "create-org"):
					fmt.Fprintln(execution.Stdout, "Creating org...")
				case strings.HasPrefix(command, "create-space"):
					fmt.Fprintln(execution.Stdout, "Creating space...")
				case strings.HasPrefix(command, "target"):
					fmt.Fprintln(execution.Stdout, "Targeting org/space...")
				case strings.HasPrefix(command, "create-security-group"):
					fmt.Fprintln(execution.Stdout, "Creating security group...")
				case strings.HasPrefix(command, "bind-security-group"):
					fmt.Fprintln(execution.Stdout, "Binding security group...")
				case strings.HasPrefix(command, "update-security-group"):
					fmt.Fprintln(execution.Stdout, "Updating security group...")
				case strings.HasPrefix(command, "push"):
					fmt.Fprintln(execution.Stdout, "Pushing app...")
				case strings.HasPrefix(command, "set-env"):
					fmt.Fprintln(execution.Stdout, "Setting environment variable...")
				}

				return nil
			}

			var err error
			workspace, err = os.MkdirTemp("", "workspace")
			Expect(err).NotTo(HaveOccurred())

			home, err = os.MkdirTemp("", "home")
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(filepath.Join(home, "config.json"), []byte(`{"Target": "https://localhost"}`), 0600)
			Expect(err).NotTo(HaveOccurred())

			setup = cloudfoundry.NewSetup(executable, home)
		})

		it.After(func() {
			Expect(os.RemoveAll(workspace)).To(Succeed())
			Expect(os.RemoveAll(home)).To(Succeed())
		})

		it("sets up the app", func() {
			logs := bytes.NewBuffer(nil)

			err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
			Expect(err).NotTo(HaveOccurred())

			Expect(executions).To(HaveLen(7))
			Expect(executions[0]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"create-org", "some-app"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[1]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"create-space", "some-app", "-o", "some-app"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[2]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"target", "-o", "some-app", "-s", "some-app"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[3]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"create-security-group", "some-app", filepath.Join(workspace, "some-home", "security-group.json")}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[4]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"bind-security-group", "some-app", "some-app", "some-app", "--lifecycle", "staging"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[5]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"update-security-group", "public_networks", filepath.Join(workspace, "some-home", "empty-security-group.json")}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[6]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"push", "some-app", "-p", "/some/path/to/my/app", "--no-start"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))

			Expect(logs).To(ContainLines(
				"Creating org...",
				"Creating space...",
				"Targeting org/space...",
				"Creating security group...",
				"Binding security group...",
				"Updating security group...",
				"Pushing app...",
			))

			content, err := os.ReadFile(filepath.Join(workspace, "some-home", ".cf", "config.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(MatchJSON(`{"Target": "https://localhost"}`))

			content, err = os.ReadFile(filepath.Join(workspace, "some-home", "security-group.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(MatchJSON(`[
				{
					"destination": "0.0.0.0-9.255.255.255",
					"protocol": "all"
				},
				{
					"destination": "11.0.0.0-169.253.255.255",
					"protocol": "all"
				},
				{
					"destination": "169.255.0.0-172.15.255.255",
					"protocol": "all"
				},
				{
					"destination": "172.32.0.0-192.167.255.255",
					"protocol": "all"
				},
				{
					"destination": "192.169.0.0-255.255.255.255",
					"protocol": "all"
				},
				{
					"destination": "127.0.0.1",
					"protocol": "all"
				}
			]`))

			content, err = os.ReadFile(filepath.Join(workspace, "some-home", "empty-security-group.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(MatchJSON("[]"))
		})

		context("when the app has buildpacks", func() {
			it("pushes the app with those buildpacks", func() {
				err := setup.
					WithBuildpacks("some-buildpack", "other-buildpack").
					Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(7))
				Expect(executions[6]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"push", "some-app", "-p", "/some/path/to/my/app", "--no-start", "-b", "some-buildpack", "-b", "other-buildpack"}),
				}))
			})
		})

		context("when the app has environment variables", func() {
			it("pushes the app with those environment variables", func() {
				logs := bytes.NewBuffer(nil)

				err := setup.
					WithEnv(map[string]string{
						"SOME_VARIABLE":  "some-value",
						"OTHER_VARIABLE": "other-value",
					}).
					Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(9))
				Expect(executions[6]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"push", "some-app", "-p", "/some/path/to/my/app", "--no-start"}),
				}))
				Expect(executions[7]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"set-env", "some-app", "OTHER_VARIABLE", "other-value"}),
				}))
				Expect(executions[8]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"set-env", "some-app", "SOME_VARIABLE", "some-value"}),
				}))

				Expect(logs).To(ContainLines(
					"Pushing app...",
					"Setting environment variable...",
					"Setting environment variable...",
				))
			})
		})

		context("when the app is offline", func() {
			it("uses a private network security group", func() {
				err := setup.
					WithoutInternetAccess().
					Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				content, err := os.ReadFile(filepath.Join(workspace, "some-home", "security-group.json"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(MatchJSON(`[
					{
						"protocol": "tcp",
						"destination": "10.0.0.0-10.255.255.255",
						"ports": "443"
					},
					{
						"protocol": "tcp",
						"destination": "172.16.0.0-172.31.255.255",
						"ports": "443"
					},
					{
						"protocol": "tcp",
						"destination": "192.168.0.0-192.168.255.255",
						"ports": "443"
					},
					{
						"destination": "127.0.0.1",
						"protocol": "all"
					}
				]`))
			})
		})

		context("failure cases", func() {
			context("when the home directory cannot be created", func() {
				it.Before(func() {
					Expect(os.Chmod(workspace, 0000)).To(Succeed())
				})

				it("returns an error", func() {
					err := setup.Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to make temporary $CF_HOME:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when the global home directory cannot be copied", func() {
				it.Before(func() {
					Expect(os.Chmod(home, 0000)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(home, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					err := setup.Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to copy $CF_HOME:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when the org cannot be created", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.HasPrefix(strings.Join(execution.Args, " "), "create-org") {
							fmt.Fprintln(execution.Stdout, "Org failed to create")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to create-org: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Org failed to create")))

					Expect(logs).To(ContainSubstring("Org failed to create"))
				})
			})

			context("when the space cannot be created", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.HasPrefix(strings.Join(execution.Args, " "), "create-space") {
							fmt.Fprintln(execution.Stdout, "Space failed to create")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to create-space: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Space failed to create")))

					Expect(logs).To(ContainSubstring("Space failed to create"))
				})
			})

			context("when the org/space cannot be targeted", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.HasPrefix(strings.Join(execution.Args, " "), "target") {
							fmt.Fprintln(execution.Stdout, "Target failed")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to target: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Target failed")))

					Expect(logs).To(ContainSubstring("Target failed"))
				})
			})

			context("when the config.json file is malformed", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(home, "config.json"), []byte("%%%"), 0600)).To(Succeed())
				})

				it("returns an error", func() {
					err := setup.Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("invalid character '%'")))
				})
			})

			context("when the config.json target is malformed", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(home, "config.json"), []byte(`{"Target": "%%%"}`), 0600)).To(Succeed())
				})

				it("returns an error", func() {
					err := setup.Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring(`invalid URL escape "%%%"`)))
				})
			})

			context("when the target host is malformed", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(home, "config.json"), []byte(`{"Target": "this is not a host"}`), 0600)).To(Succeed())
				})

				it("returns an error", func() {
					err := setup.Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("no such host")))
				})
			})

			context("when the security-group.json file cannot be created", func() {
				it.Before(func() {
					Expect(os.MkdirAll(filepath.Join(workspace, "some-home"), os.ModePerm)).To(Succeed())
					Expect(os.WriteFile(filepath.Join(workspace, "some-home", "security-group.json"), nil, 0000)).To(Succeed())
				})

				it("returns an error", func() {
					err := setup.Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when the security-group cannot be created", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.HasPrefix(strings.Join(execution.Args, " "), "create-security-group") {
							fmt.Fprintln(execution.Stdout, "Security group failed to create")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to create-security-group: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Security group failed to create")))

					Expect(logs).To(ContainSubstring("Security group failed to create"))
				})
			})

			context("when the security-group cannot be bound", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.HasPrefix(strings.Join(execution.Args, " "), "bind-security-group") {
							fmt.Fprintln(execution.Stdout, "Security group failed to bind")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to bind-security-group: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Security group failed to bind")))

					Expect(logs).To(ContainSubstring("Security group failed to bind"))
				})
			})

			context("when the security-group cannot be updated", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.HasPrefix(strings.Join(execution.Args, " "), "update-security-group") {
							fmt.Fprintln(execution.Stdout, "Security group failed to update")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to update-security-group: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Security group failed to update")))

					Expect(logs).To(ContainSubstring("Security group failed to update"))
				})
			})

			context("when the app cannot be pushed", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.HasPrefix(strings.Join(execution.Args, " "), "push") {
							fmt.Fprintln(execution.Stdout, "App failed to create")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to push: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("App failed to create")))

					Expect(logs).To(ContainSubstring("App failed to create"))
				})
			})

			context("when the environment cannot be set", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.HasPrefix(strings.Join(execution.Args, " "), "set-env") {
							fmt.Fprintln(execution.Stdout, "App failed to set environment")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					err := setup.
						WithEnv(map[string]string{"SOME_VARIABLE": "some-value"}).
						Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to set-env: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("App failed to set environment")))

					Expect(logs).To(ContainSubstring("App failed to set environment"))
				})
			})
		})
	})
}
