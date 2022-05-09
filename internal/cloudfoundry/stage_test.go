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

func testStage(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("Run", func() {
		var (
			stage cloudfoundry.Stage

			executable *fakes.Executable
			workspace  string

			executions []pexec.Execution
		)

		it.Before(func() {
			var err error
			workspace, err = os.MkdirTemp("", "workspace")
			Expect(err).NotTo(HaveOccurred())

			executable = &fakes.Executable{}
			executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
				executions = append(executions, execution)

				command := strings.Join(execution.Args, " ")
				switch {
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

				return nil
			}

			stage = cloudfoundry.NewStage(executable)
		})

		it.After(func() {
			Expect(os.RemoveAll(workspace)).To(Succeed())
		})

		it("stages the app", func() {
			logs := bytes.NewBuffer(nil)

			url, err := stage.Run(logs, filepath.Join(workspace, "some-home"), "some-app")
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("http://some-app.example.com/some/path"))

			Expect(executions).To(HaveLen(4))
			Expect(executions[0]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"start", "some-app"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[1]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"app", "some-app", "--guid"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[2]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"curl", "/v2/apps/some-app-guid/routes"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[3]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"curl", "/v2/shared_domains/some-domain-guid"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))

			Expect(logs).To(ContainLines("Starting app..."))
		})

		context("failure cases", func() {
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
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(logs, filepath.Join(workspace, "some-home"), "some-app")
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
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(logs, filepath.Join(workspace, "some-home"), "some-app")
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
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(logs, filepath.Join(workspace, "some-home"), "some-app")
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
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(logs, filepath.Join(workspace, "some-home"), "some-app")
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
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(logs, filepath.Join(workspace, "some-home"), "some-app")
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
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(logs, filepath.Join(workspace, "some-home"), "some-app")
					Expect(err).To(MatchError(ContainSubstring("failed to parse domain: invalid character '%'")))

					Expect(logs).To(ContainSubstring("Some log output"))
				})
			})
		})
	})
}
