package cloudfoundry_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudfoundry/switchblade/internal/cloudfoundry"
	"github.com/cloudfoundry/switchblade/internal/cloudfoundry/fakes"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

func testInitialize(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("Run", func() {
		var (
			initialize cloudfoundry.Initialize

			executable *fakes.Executable
			executions []pexec.Execution
		)

		it.Before(func() {
			executable = &fakes.Executable{}
			executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
				executions = append(executions, execution)

				args := strings.Join(execution.Args, " ")
				switch {
				case strings.Contains(args, "curl /v3/buildpacks?names=some-buildpack-name"):
					fmt.Fprint(execution.Stdout, `{"resources":[{"position": 123}]}`)

				case strings.Contains(args, "curl /v3/buildpacks?names=other-buildpack-name"):
					fmt.Fprint(execution.Stdout, `{"resources":[{"position": 234}]}`)
				}

				return nil
			}

			initialize = cloudfoundry.NewInitialize(executable, "test-stack")
		})

		it("updates the buildpack", func() {
			err := initialize.Run([]cloudfoundry.Buildpack{
				{
					Name: "some-buildpack-name",
					URI:  "some-buildpack-uri",
				},
				{
					Name: "other-buildpack-name",
					URI:  "other-buildpack-uri",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(executions).To(HaveLen(6))
			Expect(executions[0]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"curl", "/v3/buildpacks?names=some-buildpack-name"}),
			}))
			Expect(executions[1]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"delete-buildpack", "-f", "some-buildpack-name", "-s", "test-stack"}),
			}))
			Expect(executions[2]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"create-buildpack", "some-buildpack-name", "some-buildpack-uri", "123"}),
			}))
			Expect(executions[3]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"curl", "/v3/buildpacks?names=other-buildpack-name"}),
			}))
			Expect(executions[4]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"delete-buildpack", "-f", "other-buildpack-name", "-s", "test-stack"}),
			}))
			Expect(executions[5]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"create-buildpack", "other-buildpack-name", "other-buildpack-uri", "234"}),
			}))
		})

		context("when a buildpack does not exist", func() {
			it.Before(func() {
				executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
					executions = append(executions, execution)

					args := strings.Join(execution.Args, " ")
					switch {
					case strings.Contains(args, "curl /v3/buildpacks?names=some-buildpack-name"):
						return errors.New("no such buildpack")

					case strings.Contains(args, "curl /v3/buildpacks?names=other-buildpack-name"):
						fmt.Fprint(execution.Stdout, `{"resources":[{"position": 234}]}`)
					}

					return nil
				}
			})

			it("creates the buildpack", func() {
				err := initialize.Run([]cloudfoundry.Buildpack{
					{
						Name: "some-buildpack-name",
						URI:  "some-buildpack-uri",
					},
					{
						Name: "other-buildpack-name",
						URI:  "other-buildpack-uri",
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(5))
				Expect(executions[0]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"curl", "/v3/buildpacks?names=some-buildpack-name"}),
				}))
				Expect(executions[1]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"create-buildpack", "some-buildpack-name", "some-buildpack-uri", "1000"}),
				}))
				Expect(executions[2]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"curl", "/v3/buildpacks?names=other-buildpack-name"}),
				}))
				Expect(executions[3]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"delete-buildpack", "-f", "other-buildpack-name", "-s", "test-stack"}),
				}))
				Expect(executions[4]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"create-buildpack", "other-buildpack-name", "other-buildpack-uri", "234"}),
				}))
			})
		})

		context("failure cases", func() {
			context("when the buildpack JSON cannot be parsed", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						executions = append(executions, execution)

						args := strings.Join(execution.Args, " ")
						switch {
						case strings.Contains(args, "curl /v3/buildpacks?names=some-buildpack-name"):
							fmt.Fprint(execution.Stdout, "%%%")
						}

						return nil
					}
				})

				it("returns an error", func() {
					err := initialize.Run([]cloudfoundry.Buildpack{
						{
							Name: "some-buildpack-name",
							URI:  "some-buildpack-uri",
						},
						{
							Name: "other-buildpack-name",
							URI:  "other-buildpack-uri",
						},
					})
					Expect(err).To(MatchError(ContainSubstring("failed to parse buildpacks")))
					Expect(err).To(MatchError(ContainSubstring("invalid character '%'")))
				})
			})

			context("when a buildpack cannot be deleted", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						executions = append(executions, execution)

						args := strings.Join(execution.Args, " ")
						switch {
						case strings.Contains(args, "curl /v3/buildpacks?names=some-buildpack-name"):
							fmt.Fprintln(execution.Stdout, "{}")

						case strings.Contains(args, "delete-buildpack -f some-buildpack-name"):
							fmt.Fprintln(execution.Stderr, "something bad happened")
							return errors.New("delete-buildpack failed")
						}

						return nil
					}
				})

				it("returns an error", func() {
					err := initialize.Run([]cloudfoundry.Buildpack{
						{
							Name: "some-buildpack-name",
							URI:  "some-buildpack-uri",
						},
						{
							Name: "other-buildpack-name",
							URI:  "other-buildpack-uri",
						},
					})
					Expect(err).To(MatchError("failed to delete buildpack: delete-buildpack failed\n\nOutput:\n{}\nsomething bad happened\n"))
				})
			})

			context("when a buildpack cannot be created", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						executions = append(executions, execution)

						args := strings.Join(execution.Args, " ")
						switch {
						case strings.Contains(args, "curl /v3/buildpacks?names=some-buildpack-name"):
							return errors.New("no such buildpack")

						case strings.Contains(args, "create-buildpack some-buildpack-name"):
							fmt.Fprintln(execution.Stderr, "something bad happened")
							return errors.New("create-buildpack failed")
						}

						return nil
					}
				})

				it("returns an error", func() {
					err := initialize.Run([]cloudfoundry.Buildpack{
						{
							Name: "some-buildpack-name",
							URI:  "some-buildpack-uri",
						},
						{
							Name: "other-buildpack-name",
							URI:  "other-buildpack-uri",
						},
					})
					Expect(err).To(MatchError("failed to create buildpack: create-buildpack failed\n\nOutput:\nsomething bad happened\n"))
				})
			})
		})
	})
}
