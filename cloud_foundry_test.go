package switchblade_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/ryanmoran/switchblade"
	"github.com/ryanmoran/switchblade/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	. "github.com/ryanmoran/switchblade/matchers"
)

func testCloudFoundry(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("Deploy", func() {
		var (
			cf         switchblade.Platform
			executable *fakes.Executable
			executions []pexec.Execution
			tmpDir     string
			homeDir    string
		)

		it.Before(func() {
			executable = &fakes.Executable{}
			executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
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
				case strings.HasPrefix(command, "push"):
					fmt.Fprintln(execution.Stdout, "Pushing app...")
				case strings.HasPrefix(command, "set-env"):
					fmt.Fprintln(execution.Stdout, "Setting environment variable...")
				case strings.HasPrefix(command, "start"):
					fmt.Fprintln(execution.Stdout, "Starting app...")
				case strings.HasPrefix(command, "app"):
					fmt.Fprintln(execution.Stdout, "some-app-guid")
				case strings.HasPrefix(command, "curl /v2/apps/some-app-guid/routes"):
					fmt.Fprintln(execution.Stdout, `{
						"resources": [
							{
								"entity": {
									"domain_url": "/v2/shared_domains/some-domain-guid",
									"host": "some-app",
									"path": "/some/path"
								}
							}
						]
					}`)
				case strings.HasPrefix(command, "curl /v2/shared_domains"):
					fmt.Fprintln(execution.Stdout, `{
						"entity": {
							"name": "example.com"
						}
					}`)
				}
				executions = append(executions, execution)
				return nil
			}
			var err error
			tmpDir, err = os.MkdirTemp("", "tmp-dir")
			Expect(err).NotTo(HaveOccurred())
			homeDir, err = os.MkdirTemp("", "home-dir")
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(homeDir, "config.json"), []byte(`{"Target": "https://localhost"}`), 0600)
			Expect(err).NotTo(HaveOccurred())
			cf = switchblade.NewCloudFoundry(executable, tmpDir, homeDir)
		})

		it.After(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
			Expect(os.RemoveAll(homeDir)).To(Succeed())
		})

		it("pushes the app", func() {
			deployment, logs, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment).To(Equal(switchblade.Deployment{
				Name: "some-app",
				URL:  "http://some-app.example.com/some/path",
			}))

			Expect(executions).To(HaveLen(11))
			Expect(executions[0]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"create-org", "some-app"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s/some-app", tmpDir)),
			}))
			Expect(executions[1]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"create-space", "some-app", "-o", "some-app"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s/some-app", tmpDir)),
			}))
			Expect(executions[2]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"target", "-o", "some-app", "-s", "some-app"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s/some-app", tmpDir)),
			}))
			Expect(executions[3]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"create-security-group", "some-app", filepath.Join(tmpDir, "some-app", "security-group.json")}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s/some-app", tmpDir)),
			}))
			Expect(executions[4]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"bind-security-group", "some-app", "some-app", "some-app", "--lifecycle", "staging"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s/some-app", tmpDir)),
			}))
			Expect(executions[5]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"update-security-group", "public_networks", filepath.Join(tmpDir, "some-app", "empty-security-group.json")}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s/some-app", tmpDir)),
			}))
			Expect(executions[6]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"push", "some-app", "-p", "/some/path/to/my/app", "--no-start"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s/some-app", tmpDir)),
			}))
			Expect(executions[7]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"start", "some-app"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s/some-app", tmpDir)),
			}))
			Expect(executions[8]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"app", "some-app", "--guid"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s/some-app", tmpDir)),
			}))
			Expect(executions[9]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"curl", "/v2/apps/some-app-guid/routes"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s/some-app", tmpDir)),
			}))
			Expect(executions[10]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"curl", "/v2/shared_domains/some-domain-guid"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s/some-app", tmpDir)),
			}))

			Expect(logs).To(ContainLines(
				"Creating org...",
				"Creating space...",
				"Targeting org/space...",
				"Creating security group...",
				"Pushing app...",
				"Starting app...",
			))

			content, err := os.ReadFile(filepath.Join(tmpDir, "some-app", ".cf", "config.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(MatchJSON(`{"Target": "https://localhost"}`))

			content, err = os.ReadFile(filepath.Join(tmpDir, "some-app", "security-group.json"))
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

			content, err = os.ReadFile(filepath.Join(tmpDir, "some-app", "empty-security-group.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(MatchJSON("[]"))
		})

		context("when the app has buildpacks", func() {
			it("pushes the app with those buildpacks", func() {
				_, _, err := cf.Deploy.
					WithBuildpacks("some-buildpack", "other-buildpack").
					Execute("some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())
				Expect(executions).To(HaveLen(11))
				Expect(executions[6]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"push", "some-app", "-p", "/some/path/to/my/app", "--no-start", "-b", "some-buildpack", "-b", "other-buildpack"}),
				}))
			})
		})

		context("when the app has environment variables", func() {
			it("pushes the app with those environment variables", func() {
				_, logs, err := cf.Deploy.
					WithEnv(map[string]string{
						"SOME_VARIABLE":  "some-value",
						"OTHER_VARIABLE": "other-value",
					}).
					Execute("some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())
				Expect(executions).To(HaveLen(13))
				Expect(executions[6]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"push", "some-app", "-p", "/some/path/to/my/app", "--no-start"}),
				}))
				Expect(executions[7]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"set-env", "some-app", "OTHER_VARIABLE", "other-value"}),
				}))
				Expect(executions[8]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"set-env", "some-app", "SOME_VARIABLE", "some-value"}),
				}))
				Expect(executions[9]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"start", "some-app"}),
				}))
				Expect(logs).To(ContainLines(
					"Pushing app...",
					"Setting environment variable...",
					"Setting environment variable...",
					"Starting app...",
				))
			})
		})

		context("when the app is offline", func() {
			it("uses a private network security group", func() {
				_, _, err := cf.Deploy.
					WithoutInternetAccess().
					Execute("some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())
				content, err := os.ReadFile(filepath.Join(tmpDir, "some-app", "security-group.json"))
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
					Expect(os.Chmod(tmpDir, 0000)).To(Succeed())
					cf = switchblade.NewCloudFoundry(executable, filepath.Join(tmpDir, "some-path"), homeDir)
				})

				it("returns an error", func() {
					_, _, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when the global home directory cannot be copied", func() {
				it.Before(func() {
					Expect(os.Chmod(homeDir, 0000)).To(Succeed())
					cf = switchblade.NewCloudFoundry(executable, tmpDir, homeDir)
				})

				it.After(func() {
					Expect(os.Chmod(homeDir, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					_, _, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
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
					_, logs, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
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
					_, logs, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
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
					_, logs, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to target: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Target failed")))
					Expect(logs).To(ContainSubstring("Target failed"))
				})
			})

			context("when the security-group.json file cannot be created", func() {
				it.Before(func() {
					Expect(os.MkdirAll(filepath.Join(tmpDir, "some-app"), os.ModePerm)).To(Succeed())
					Expect(os.WriteFile(filepath.Join(tmpDir, "some-app", "security-group.json"), nil, 0000)).To(Succeed())
				})

				it("returns an error", func() {
					_, _, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when the config.json file is malformed", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(homeDir, "config.json"), []byte("%%%"), 0600)).To(Succeed())
				})

				it("returns an error", func() {
					_, _, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("invalid character '%'")))
				})
			})

			context("when the config.json target is malformed", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(homeDir, "config.json"), []byte(`{"Target": "%%%"}`), 0600)).To(Succeed())
				})

				it("returns an error", func() {
					_, _, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring(`invalid URL escape "%%%"`)))
				})
			})

			context("when the target host is malformed", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(homeDir, "config.json"), []byte(`{"Target": "this is not a host"}`), 0600)).To(Succeed())
				})

				it("returns an error", func() {
					_, _, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("no such host")))
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
					_, logs, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
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
					_, logs, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
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
					_, logs, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
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
					_, logs, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
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
					_, logs, err := cf.Deploy.
						WithEnv(map[string]string{"SOME_VARIABLE": "some-value"}).
						Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to set-env: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("App failed to set environment")))
					Expect(logs).To(ContainSubstring("App failed to set environment"))
				})
			})

			context("when the app cannot be started", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.HasPrefix(strings.Join(execution.Args, " "), "start") {
							fmt.Fprintln(execution.Stdout, "App failed to start")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					_, logs, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to start: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("App failed to start")))
					Expect(logs).To(ContainSubstring("App failed to start"))
				})
			})

			context("when the guid cannot be fetched", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						fmt.Fprintln(execution.Stdout, "Some log output")
						if strings.HasPrefix(strings.Join(execution.Args, " "), "app") {
							fmt.Fprintln(execution.Stdout, "Could not fetch guid")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					_, logs, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to fetch guid: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Could not fetch guid")))
					Expect(logs).To(ContainSubstring("Some log output"))
				})
			})

			context("when the routes cannot be fetched", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						fmt.Fprintln(execution.Stdout, "Some log output")
						if strings.HasPrefix(strings.Join(execution.Args, " "), "curl /v2/app") {
							fmt.Fprintln(execution.Stdout, "Could not fetch routes")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					_, logs, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to fetch routes: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Could not fetch routes")))
					Expect(logs).To(ContainSubstring("Some log output"))
				})
			})

			context("when the routes response is not JSON", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.HasPrefix(strings.Join(execution.Args, " "), "curl /v2/apps") {
							fmt.Fprintln(execution.Stdout, "%%%%")
						} else {
							fmt.Fprintln(execution.Stdout, "Some log output")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					_, logs, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to parse routes: invalid character '%'")))
					Expect(logs).To(ContainSubstring("Some log output"))
				})
			})

			context("when the domain cannot be fetched", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "app some-app --guid"):
							fmt.Fprintln(execution.Stdout, "some-app-guid")
						case strings.HasPrefix(command, "curl /v2/apps/some-app-guid/routes"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{
										"entity": {
											"domain_url": "/v2/shared_domains/some-domain-guid",
											"host": "some-app",
											"path": "/some/path"
										}
									}
								]
							}`)
						case strings.HasPrefix(command, "curl /v2/shared_domains"):
							fmt.Fprintln(execution.Stdout, "Could not fetch domain")
							return errors.New("exit status 1")
						default:
							fmt.Fprintln(execution.Stdout, "Some log output")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					_, logs, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to fetch domain: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Could not fetch domain")))
					Expect(logs).To(ContainSubstring("Some log output"))
				})
			})

			context("when the domain cannot be parsed", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "app some-app --guid"):
							fmt.Fprintln(execution.Stdout, "some-app-guid")
						case strings.HasPrefix(command, "curl /v2/apps/some-app-guid/routes"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{
										"entity": {
											"domain_url": "/v2/shared_domains/some-domain-guid",
											"host": "some-app",
											"path": "/some/path"
										}
									}
								]
							}`)
						case strings.HasPrefix(command, "curl /v2/shared_domains"):
							fmt.Fprintln(execution.Stdout, "%%%")
						default:
							fmt.Fprintln(execution.Stdout, "Some log output")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					_, logs, err := cf.Deploy.Execute("some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to parse domain: invalid character '%'")))
					Expect(logs).To(ContainSubstring("Some log output"))
				})
			})
		})
	})

	context("Delete", func() {
		var (
			cf         switchblade.Platform
			executable *fakes.Executable
			executions []pexec.Execution
			tmpDir     string
		)

		it.Before(func() {
			executable = &fakes.Executable{}
			executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
				executions = append(executions, execution)
				return nil
			}

			var err error
			tmpDir, err = os.MkdirTemp("", "tmp-dir")
			Expect(err).NotTo(HaveOccurred())

			err = os.MkdirAll(filepath.Join(tmpDir, "some-app"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			cf = switchblade.NewCloudFoundry(executable, tmpDir, "")
		})

		it("deletes the org, security-group, and config", func() {
			err := cf.Delete.Execute("some-app")
			Expect(err).NotTo(HaveOccurred())

			Expect(executions).To(HaveLen(2))
			Expect(executions[0]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"delete-org", "some-app", "-f"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s/some-app", tmpDir)),
			}))
			Expect(executions[1]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"delete-security-group", "some-app", "-f"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s/some-app", tmpDir)),
			}))

			Expect(filepath.Join(tmpDir, "some-app")).NotTo(BeADirectory())
		})

		context("failure cases", func() {
			context("when the delete-org fails", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.HasPrefix(strings.Join(execution.Args, " "), "delete-org") {
							fmt.Fprintf(execution.Stdout, "Could not delete org")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error", func() {
					err := cf.Delete.Execute("some-app")
					Expect(err).To(MatchError(ContainSubstring("failed to delete-org: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Could not delete org")))
				})
			})

			context("when the delete-security-group fails", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.HasPrefix(strings.Join(execution.Args, " "), "delete-security-group") {
							fmt.Fprintf(execution.Stdout, "Could not delete security group")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error", func() {
					err := cf.Delete.Execute("some-app")
					Expect(err).To(MatchError(ContainSubstring("failed to delete-security-group: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Could not delete security group")))
				})
			})
		})
	})
}
