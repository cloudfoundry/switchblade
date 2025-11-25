package cloudfoundry_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudfoundry/switchblade/internal/cloudfoundry"
	"github.com/cloudfoundry/switchblade/internal/cloudfoundry/fakes"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/sclevine/spec"

	. "github.com/cloudfoundry/switchblade/matchers"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
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
				case strings.HasPrefix(command, "curl /v3/domains"):
					fmt.Fprintln(execution.Stdout, `{
						"resources": [
							{ "name": "other-domain", "internal": true },
							{ "name": "example.com", "internal": false }
						]
					}`)
				case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
					fmt.Fprintln(execution.Stdout, `[
						{ "name": "other-router-group", "type": "http" },
						{ "name": "some-router-group", "type": "tcp" }
					]`)
				case strings.HasPrefix(command, "curl /v3/spaces"):
					fmt.Fprintln(execution.Stdout, `{
						"resources": [
							{ "name": "other-app", "guid": "other-space-guid" },
							{ "name": "some-app", "guid": "some-space-guid" }
						]
					}`)
				case strings.HasPrefix(command, "curl /v3/routes"):
					fmt.Fprintln(execution.Stdout, `{ "resources": [
						{ "protocol": "http", "port": null },
						{ "protocol": "tcp", "port": 5555 }
					] }`)
				case strings.HasPrefix(command, "curl /v3/security_groups"):
					fmt.Fprintln(execution.Stdout, `{ "resources": [
						{ "name": "some-default-network" },
						{ "name": "switchblade-network" }
					] }`)

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
				case strings.HasPrefix(command, "create-user-provided-service"):
					fmt.Fprintln(execution.Stdout, "Creating service...")
				case strings.HasPrefix(command, "bind-service"):
					fmt.Fprintln(execution.Stdout, "Binding service...")
				case strings.HasPrefix(command, "create-shared-domain"):
					fmt.Fprintln(execution.Stdout, "Creating shared domain...")
				case strings.HasPrefix(command, "update-quota"):
					fmt.Fprintln(execution.Stdout, "Updating quota...")
				case strings.HasPrefix(command, "map-route"):
					fmt.Fprintln(execution.Stdout, "Mapping route...")
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

			err = os.MkdirAll(filepath.Join(workspace, "some-home", ".cf"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(filepath.Join(workspace, "some-home", ".cf", "config.json"), []byte(`{"Target": "https://example.com"}`), 0600)
			Expect(err).NotTo(HaveOccurred())

			setup = cloudfoundry.NewSetup(executable, home, "default-stack").WithCustomHostLookup(func(fqdn string) ([]string, error) {
				switch fqdn {
				case "localhost":
					return []string{"127.0.0.1", "::1"}, nil
				case "tcp.example.com":
					return []string{"192.168.0.1", "::1"}, nil
				default:
					return nil, errors.New("no such host")
				}
			})
		})

		it.After(func() {
			Expect(os.RemoveAll(workspace)).To(Succeed())
			Expect(os.RemoveAll(home)).To(Succeed())
		})

		it("sets up the app", func() {
			logs := bytes.NewBuffer(nil)

			url, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("http://tcp.example.com:5555"))

			Expect(executions).To(HaveLen(16))
			Expect(executions[0]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"curl", "/v3/domains"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[1]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"curl", "/routing/v1/router_groups"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[2]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"create-shared-domain", "tcp.example.com", "--router-group", "some-router-group"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[3]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"create-org", "some-app"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[4]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"create-space", "some-app", "-o", "some-app"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[5]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"target", "-o", "some-app", "-s", "some-app"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[6]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"create-security-group", "some-app", filepath.Join(workspace, "some-home", "security-group.json")}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[7]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"bind-security-group", "some-app", "some-app", "--space", "some-app", "--lifecycle", "staging"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[8]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"bind-security-group", "some-app", "some-app", "--space", "some-app", "--lifecycle", "running"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[9]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"curl", "/v3/security_groups"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[10]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"update-security-group", "some-default-network", filepath.Join(workspace, "some-home", "empty-security-group.json")}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[11]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"push", "some-app", "-p", "/some/path/to/my/app", "--no-start", "-s", "default-stack"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[12]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"update-quota", "default", "--reserved-route-ports", "100"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[13]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"map-route", "some-app", "tcp.example.com"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[14]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"curl", "/v3/spaces"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[15]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"curl", "/v3/routes?space_guids=some-space-guid"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))

			Expect(logs).To(ContainLines(
				"Creating shared domain...",
				"Creating org...",
				"Creating space...",
				"Targeting org/space...",
				"Creating security group...",
				"Binding security group...",
				"Binding security group...",
			))
			Expect(logs).To(ContainLines(
				"Updating security group...",
				"Pushing app...",
				"Updating quota...",
				"Mapping route...",
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
				},
				{
					"destination": "192.168.0.1",
					"protocol": "all"
				}
			]`))

			content, err = os.ReadFile(filepath.Join(workspace, "some-home", "empty-security-group.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(MatchJSON("[]"))
		})

		context("when the app has buildpacks", func() {
			it("pushes the app with those buildpacks", func() {
				_, err := setup.
					WithBuildpacks("some-buildpack", "other-buildpack").
					Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(16))
				Expect(executions[11]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{
						"push", "some-app",
						"-p", "/some/path/to/my/app",
						"--no-start",
						"-s", "default-stack",
						"-b", "some-buildpack",
						"-b", "other-buildpack",
					}),
					"Env": ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))
			})
		})

		context("when the app has a specific stack", func() {
			it("pushes the app with that stack", func() {
				_, err := setup.
					WithStack("some-stack").
					Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(16))
				Expect(executions[11]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"push", "some-app", "-p", "/some/path/to/my/app", "--no-start", "-s", "some-stack"}),
					"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))
			})
		})

		context("when the app has environment variables", func() {
			it("pushes the app with those environment variables", func() {
				logs := bytes.NewBuffer(nil)

				_, err := setup.
					WithEnv(map[string]string{
						"SOME_VARIABLE":  "some-value",
						"OTHER_VARIABLE": "other-value",
					}).
					Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(18))
				Expect(executions[16]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"set-env", "some-app", "OTHER_VARIABLE", "other-value"}),
					"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))
				Expect(executions[17]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"set-env", "some-app", "SOME_VARIABLE", "some-value"}),
					"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))

				Expect(logs).To(ContainLines(
					"Setting environment variable...",
					"Setting environment variable...",
				))
			})
		})

		context("when the app is offline", func() {
			it("uses a private network security group", func() {
				_, err := setup.
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
					},
					{
						"destination": "192.168.0.1",
						"protocol": "all"
					}
				]`))
			})
		})

		context("when the app has services", func() {
			it("creates and binds those services", func() {
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
					Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(20))
				Expect(executions[16]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"create-user-provided-service", "some-app-other-service", "-p", `{"other-key":"other-value"}`}),
					"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))
				Expect(executions[17]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"bind-service", "some-app", "some-app-other-service"}),
					"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))
				Expect(executions[18]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"create-user-provided-service", "some-app-some-service", "-p", `{"some-key":"some-value"}`}),
					"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))
				Expect(executions[19]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"bind-service", "some-app", "some-app-some-service"}),
					"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))

				Expect(logs).To(ContainLines(
					"Creating service...",
					"Binding service...",
					"Creating service...",
					"Binding service...",
				))
			})
		})

		context("when the tcp domain already exists", func() {
			it.Before(func() {
				executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
					executions = append(executions, execution)

					command := strings.Join(execution.Args, " ")
					switch {
					case strings.HasPrefix(command, "curl /v3/domains"):
						fmt.Fprintln(execution.Stdout, `{
							"resources": [
								{ "name": "other-domain", "internal": true },
								{ "name": "example.com", "internal": false },
								{ "name": "tcp.example.com", "internal": false }
							]
						}`)
					case strings.HasPrefix(command, "curl /v3/spaces"):
						fmt.Fprintln(execution.Stdout, `{
							"resources": [
								{ "name": "other-app", "guid": "other-space-guid" },
								{ "name": "some-app", "guid": "some-space-guid" }
							]
						}`)
					case strings.HasPrefix(command, "curl /v3/routes"):
						fmt.Fprintln(execution.Stdout, `{ "resources": [
							{ "protocol": "http", "port": null },
							{ "protocol": "tcp", "port": 5555 }
						] }`)
					case strings.HasPrefix(command, "curl /v3/security_groups"):
						fmt.Fprintln(execution.Stdout, `{ "resources": [
							{ "name": "some-default-network"},
							{ "name": "switchblade-network" }
						] }`)
					}

					return nil
				}
			})

			it("skips creating that domain again", func() {
				logs := bytes.NewBuffer(nil)

				_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(14))
				Expect(executions[0]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"curl", "/v3/domains"}),
					"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))
				Expect(executions[1]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"create-org", "some-app"}),
					"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))

				Expect(logs).NotTo(ContainLines("Creating shared domain..."))
			})
		})

		context("when the domain has an apps. prefix", func() {
			it.Before(func() {
				executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
					executions = append(executions, execution)

					command := strings.Join(execution.Args, " ")
					switch {
					case strings.HasPrefix(command, "curl /v3/domains"):
						fmt.Fprintln(execution.Stdout, `{
							"resources": [
								{ "name": "other-domain", "internal": true },
								{ "name": "apps.example.com", "internal": false }
							]
						}`)
					case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
						fmt.Fprintln(execution.Stdout, `[
							{ "name": "other-router-group", "type": "http" },
							{ "name": "some-router-group", "type": "tcp" }
						]`)
					case strings.HasPrefix(command, "curl /v3/spaces"):
						fmt.Fprintln(execution.Stdout, `{
							"resources": [
								{ "name": "other-app", "guid": "other-space-guid" },
								{ "name": "some-app", "guid": "some-space-guid" }
							]
						}`)
					case strings.HasPrefix(command, "curl /v3/routes"):
						fmt.Fprintln(execution.Stdout, `{ "resources": [
							{ "protocol": "http", "port": null },
							{ "protocol": "tcp", "port": 5555 }
						] }`)
					case strings.HasPrefix(command, "curl /v3/security_groups"):
						fmt.Fprintln(execution.Stdout, `{ "resources": [
							{ "name": "some-default-network" },
							{ "name": "switchblade-network" }
						] }`)
					}

					return nil
				}
			})

			it("strips the prefix from the domain", func() {
				logs := bytes.NewBuffer(nil)

				_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(16))
				Expect(executions[0]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"curl", "/v3/domains"}),
					"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))
				Expect(executions[1]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"curl", "/routing/v1/router_groups"}),
					"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))
				Expect(executions[2]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"create-shared-domain", "tcp.example.com", "--router-group", "some-router-group"}),
					"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))
			})
		})

		context("when the app has an start command", func() {
			it("sets the start command", func() {
				logs := bytes.NewBuffer(nil)

				_, err := setup.
					WithStartCommand("some-start-command some-file").
					Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(16))
				Expect(executions[11]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{
						"push", "some-app",
						"-p", "/some/path/to/my/app",
						"--no-start",
						"-s", "default-stack",
						"-c", "some-start-command some-file",
					}),
					"Env": ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))
			})
		})

		context("when the app has a manifest", func() {
			var tmpappdir string
			var err error

			it.Before(func() {
				tmpappdir, err = os.MkdirTemp("", "tmpappdir")
				Expect(err).NotTo(HaveOccurred())

				_, err = os.Create(filepath.Join(tmpappdir, "manifest.yml"))
				Expect(err).NotTo(HaveOccurred())
			})

			it.After(func() {
				Expect(os.RemoveAll(tmpappdir)).To(Succeed())
			})

			it("passes manifest to the push cmd", func() {
				logs := bytes.NewBuffer(nil)

				_, err := setup.
					Run(logs, filepath.Join(workspace, "some-home"), "some-app", tmpappdir)
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(16))
				Expect(executions[11]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{
						"push", "some-app",
						"-p", tmpappdir,
						"--no-start",
						"-s", "default-stack",
						"-f", filepath.Join(tmpappdir, "manifest.yml"),
					}),
					"Env": ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
				}))
			})
		})

		context("failure cases", func() {
			context("when the home directory cannot be created", func() {
				it.Before(func() {
					Expect(os.Chmod(workspace, 0000)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(workspace, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := setup.Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
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
					_, err := setup.Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to copy $CF_HOME:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when the domains cannot be listed", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{"error": "could not list domains"}`)
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to curl /v3/domains: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring(`{"error": "could not list domains"}`)))

					Expect(logs).To(ContainSubstring(`{"error": "could not list domains"}`))
				})
			})

			context("when the domains cannot be unmarshalled", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, "%%%")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to parse domains")))
					Expect(err).To(MatchError(ContainSubstring("invalid character '%'")))
				})
			})

			context("when the router groups cannot be listed", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `{"error": "could not list router groups"}`)
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to curl /routing/v1/router_groups: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring(`{"error": "could not list router groups"}`)))

					Expect(logs).To(ContainSubstring(`{"error": "could not list router groups"}`))
				})
			})

			context("when the router groups cannot be unmarshalled", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, "%%%")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to parse router groups")))
					Expect(err).To(MatchError(ContainSubstring("invalid character '%'")))
				})
			})

			context("when the shared domain cannot be created", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)

						case strings.HasPrefix(command, "create-shared-domain"):
							fmt.Fprintln(execution.Stdout, "Shared domain failed to create")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to create-shared-domain: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Shared domain failed to create")))

					Expect(logs).To(ContainSubstring("Shared domain failed to create"))
				})
			})

			context("when the org cannot be created", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)

						case strings.HasPrefix(command, "create-org"):
							fmt.Fprintln(execution.Stdout, "Org failed to create")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to create-org: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Org failed to create")))

					Expect(logs).To(ContainSubstring("Org failed to create"))
				})
			})

			context("when the space cannot be created", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)

						case strings.HasPrefix(command, "create-space"):
							fmt.Fprintln(execution.Stdout, "Space failed to create")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to create-space: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Space failed to create")))

					Expect(logs).To(ContainSubstring("Space failed to create"))
				})
			})

			context("when the org/space cannot be targeted", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)

						case strings.HasPrefix(command, "target"):
							fmt.Fprintln(execution.Stdout, "Target failed")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
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
					_, err := setup.Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("invalid character '%'")))
				})
			})

			context("when the config.json target is malformed", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(home, "config.json"), []byte(`{"Target": "%%%"}`), 0600)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := setup.Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring(`invalid URL escape "%%%"`)))
				})
			})

			context("when the target host is malformed", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(home, "config.json"), []byte(`{"Target": "this is not a host"}`), 0600)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := setup.Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("no such host")))
				})
			})

			context("when the security-group.json file cannot be created", func() {
				it.Before(func() {
					Expect(os.MkdirAll(filepath.Join(workspace, "some-home"), os.ModePerm)).To(Succeed())
					Expect(os.WriteFile(filepath.Join(workspace, "some-home", "security-group.json"), nil, 0000)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := setup.Run(bytes.NewBuffer(nil), filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when the security-group cannot be created", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)

						case strings.HasPrefix(command, "create-security-group"):
							fmt.Fprintln(execution.Stdout, "Security group failed to create")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to create-security-group: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Security group failed to create")))

					Expect(logs).To(ContainSubstring("Security group failed to create"))
				})
			})

			context("when the security-group cannot be bound", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)

						case strings.HasPrefix(command, "bind-security-group"):
							fmt.Fprintln(execution.Stdout, "Security group failed to bind")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to bind-security-group: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Security group failed to bind")))

					Expect(logs).To(ContainSubstring("Security group failed to bind"))
				})
			})

			context("when the security groups cannot be listed", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)

						case strings.HasPrefix(command, "curl /v3/security_groups"):
							fmt.Fprintln(execution.Stdout, `{"error": "could not list security groups"}`)
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to curl /v3/security_groups: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring(`{"error": "could not list security groups"}`)))

					Expect(logs).To(ContainSubstring(`{"error": "could not list security groups"}`))
				})
			})

			context("when the security groups list is malformed", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)

						case strings.HasPrefix(command, "curl /v3/security_groups"):
							fmt.Fprintln(execution.Stdout, "%%%")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to parse security groups")))
					Expect(err).To(MatchError(ContainSubstring("invalid character '%'")))
				})
			})

			context("when the security-group cannot be updated", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)
						case strings.HasPrefix(command, "curl /v3/security_groups"):
							fmt.Fprintln(execution.Stdout, `{ "resources": [
								{ "name": "some-default-network" },
								{ "name": "switchblade-network" }
							] }`)

						case strings.HasPrefix(command, "update-security-group"):
							fmt.Fprintln(execution.Stdout, "Security group failed to update")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to update-security-group: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Security group failed to update")))

					Expect(logs).To(ContainSubstring("Security group failed to update"))
				})
			})

			context("when the app cannot be pushed", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)
						case strings.HasPrefix(command, "curl /v3/security_groups"):
							fmt.Fprintln(execution.Stdout, `{ "resources": [
								{ "name": "some-default-network"},
								{ "name": "switchblade-network"}
							] }`)

						case strings.HasPrefix(command, "push"):
							fmt.Fprintln(execution.Stdout, "App failed to create")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to push: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("App failed to create")))

					Expect(logs).To(ContainSubstring("App failed to create"))
				})
			})

			context("when the quota cannot be updated", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)
						case strings.HasPrefix(command, "curl /v3/security_groups"):
							fmt.Fprintln(execution.Stdout, `{ "resources": [
								{ "name": "some-default-network"},
								{ "name": "switchblade-network"}
							] }`)

						case strings.HasPrefix(command, "update-quota"):
							fmt.Fprintln(execution.Stdout, "Quota failed to update")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to update-quota: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Quota failed to update")))

					Expect(logs).To(ContainSubstring("Quota failed to update"))
				})
			})

			context("when the route cannot be mapped", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)
						case strings.HasPrefix(command, "curl /v3/security_groups"):
							fmt.Fprintln(execution.Stdout, `{ "resources": [
								{ "name": "some-default-network"},
								{ "name": "switchblade-network"}
							] }`)

						case strings.HasPrefix(command, "map-route"):
							fmt.Fprintln(execution.Stdout, "Route failed to map")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to map-route: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Route failed to map")))

					Expect(logs).To(ContainSubstring("Route failed to map"))
				})
			})

			context("when the spaces cannot be listed", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)
						case strings.HasPrefix(command, "curl /v3/security_groups"):
							fmt.Fprintln(execution.Stdout, `{ "resources": [
								{ "name": "some-default-network"},
								{ "name": "switchblade-network"}
							] }`)

						case strings.HasPrefix(command, "curl /v3/spaces"):
							fmt.Fprintln(execution.Stdout, `{"error": "could not list spaces"}`)
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to curl /v3/spaces: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring(`{"error": "could not list spaces"}`)))

					Expect(logs).To(ContainSubstring(`{"error": "could not list spaces"}`))
				})
			})

			context("when the spaces cannot be unmarshalled", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)
						case strings.HasPrefix(command, "curl /v3/security_groups"):
							fmt.Fprintln(execution.Stdout, `{ "resources": [
								{ "name": "some-default-network" },
								{ "name": "switchblade-network" }
							] }`)

						case strings.HasPrefix(command, "curl /v3/spaces"):
							fmt.Fprintln(execution.Stdout, "%%%")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to parse spaces")))
					Expect(err).To(MatchError(ContainSubstring("invalid character '%'")))
				})
			})

			context("when the routes cannot be listed", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)
						case strings.HasPrefix(command, "curl /v3/spaces"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-app", "guid": "other-space-guid" },
									{ "name": "some-app", "guid": "some-space-guid" }
								]
							}`)
						case strings.HasPrefix(command, "curl /v3/security_groups"):
							fmt.Fprintln(execution.Stdout, `{ "resources": [
								{ "name": "some-default-network"},
								{ "name": "switchblade-network"}
							] }`)

						case strings.HasPrefix(command, "curl /v3/routes"):
							fmt.Fprintln(execution.Stdout, `{"error": "could not list routes"}`)
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to curl /v3/routes: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring(`{"error": "could not list routes"}`)))

					Expect(logs).To(ContainSubstring(`{"error": "could not list routes"}`))
				})
			})

			context("when the routes cannot be unmarshalled", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)
						case strings.HasPrefix(command, "curl /v3/spaces"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-app", "guid": "other-space-guid" },
									{ "name": "some-app", "guid": "some-space-guid" }
								]
							}`)
						case strings.HasPrefix(command, "curl /v3/security_groups"):
							fmt.Fprintln(execution.Stdout, `{ "resources": [
								{ "name": "some-default-network"},
								{ "name": "switchblade-network"}
							] }`)

						case strings.HasPrefix(command, "curl /v3/routes"):
							fmt.Fprintln(execution.Stdout, "%%%")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to parse routes")))
					Expect(err).To(MatchError(ContainSubstring("invalid character '%'")))
				})
			})

			context("when the environment cannot be set", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)
						case strings.HasPrefix(command, "curl /v3/spaces"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-app", "guid": "other-space-guid" },
									{ "name": "some-app", "guid": "some-space-guid" }
								]
							}`)
						case strings.HasPrefix(command, "curl /v3/routes"):
							fmt.Fprintln(execution.Stdout, `{ "resources": [
								{ "protocol": "http", "port": null },
								{ "protocol": "tcp", "port": 5555 }
							] }`)
						case strings.HasPrefix(command, "curl /v3/security_groups"):
							fmt.Fprintln(execution.Stdout, `{ "resources": [
								{ "name": "some-default-network" },
								{ "name": "switchblade-network" }
							] }`)

						case strings.HasPrefix(command, "set-env"):
							fmt.Fprintln(execution.Stdout, "App failed to set environment")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.
						WithEnv(map[string]string{"SOME_VARIABLE": "some-value"}).
						Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to set-env: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("App failed to set environment")))

					Expect(logs).To(ContainSubstring("App failed to set environment"))
				})
			})

			context("when the services cannot be marshalled to json", func() {
				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.
						WithServices(map[string]map[string]interface{}{
							"some-service": {
								"some-key": func() {},
							},
						}).
						Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to marshal services json")))
					Expect(err).To(MatchError(ContainSubstring("unsupported type: func()")))
				})
			})

			context("when a service cannot be created", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)
						case strings.HasPrefix(command, "curl /v3/spaces"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-app", "guid": "other-space-guid" },
									{ "name": "some-app", "guid": "some-space-guid" }
								]
							}`)
						case strings.HasPrefix(command, "curl /v3/routes"):
							fmt.Fprintln(execution.Stdout, `{ "resources": [
								{ "protocol": "http", "port": null },
								{ "protocol": "tcp", "port": 5555 }
							] }`)
						case strings.HasPrefix(command, "curl /v3/security_groups"):
							fmt.Fprintln(execution.Stdout, `{ "resources": [
								{ "name": "some-default-network" },
								{ "name": "switchblade-network" }
							] }`)

						case strings.HasPrefix(command, "create-user-provided-service"):
							fmt.Fprintln(execution.Stdout, "could not create user-provided service")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.
						WithServices(map[string]map[string]interface{}{
							"some-service": {
								"some-key": "some-value",
							},
						}).
						Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to create-user-provided-service: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("could not create user-provided service")))
				})
			})

			context("when a service cannot be bound", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						command := strings.Join(execution.Args, " ")
						switch {
						case strings.HasPrefix(command, "curl /v3/domains"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-domain", "internal": true },
									{ "name": "example.com", "internal": false }
								]
							}`)
						case strings.HasPrefix(command, "curl /routing/v1/router_groups"):
							fmt.Fprintln(execution.Stdout, `[
								{ "name": "other-router-group", "type": "http" },
								{ "name": "some-router-group", "type": "tcp" }
							]`)
						case strings.HasPrefix(command, "curl /v3/spaces"):
							fmt.Fprintln(execution.Stdout, `{
								"resources": [
									{ "name": "other-app", "guid": "other-space-guid" },
									{ "name": "some-app", "guid": "some-space-guid" }
								]
							}`)
						case strings.HasPrefix(command, "curl /v3/routes"):
							fmt.Fprintln(execution.Stdout, `{ "resources": [
								{ "protocol": "http", "port": null },
								{ "protocol": "tcp", "port": 5555 }
							] }`)
						case strings.HasPrefix(command, "curl /v3/security_groups"):
							fmt.Fprintln(execution.Stdout, `{ "resources": [
								{ "name": "some-default-network" },
								{ "name": "switchblade-network" }
							] }`)

						case strings.HasPrefix(command, "bind-service"):
							fmt.Fprintln(execution.Stdout, "could not bind service")
							return errors.New("exit status 1")
						}
						return nil
					}
				})

				it("returns an error and the build logs", func() {
					logs := bytes.NewBuffer(nil)

					_, err := setup.
						WithServices(map[string]map[string]interface{}{
							"some-service": {
								"some-key": "some-value",
							},
						}).
						Run(logs, filepath.Join(workspace, "some-home"), "some-app", "/some/path/to/my/app")
					Expect(err).To(MatchError(ContainSubstring("failed to bind-service: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("could not bind service")))
				})
			})
		})
	})
}
