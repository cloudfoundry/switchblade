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
				return nil
			}

			initialize = cloudfoundry.NewInitialize(executable)
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

			Expect(executions).To(HaveLen(2))
			Expect(executions[0]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"update-buildpack", "some-buildpack-name", "-p", "some-buildpack-uri"}),
			}))
			Expect(executions[1]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"update-buildpack", "other-buildpack-name", "-p", "other-buildpack-uri"}),
			}))
		})

		context("when a buildpack does not exist", func() {
			it.Before(func() {
				executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
					executions = append(executions, execution)

					if strings.Contains(strings.Join(execution.Args, " "), "update-buildpack some-buildpack-name") {
						return errors.New("no such buildpack")
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

				Expect(executions).To(HaveLen(3))
				Expect(executions[0]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"update-buildpack", "some-buildpack-name", "-p", "some-buildpack-uri"}),
				}))
				Expect(executions[1]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"create-buildpack", "some-buildpack-name", "some-buildpack-uri", "1000"}),
				}))
				Expect(executions[2]).To(MatchFields(IgnoreExtras, Fields{
					"Args": Equal([]string{"update-buildpack", "other-buildpack-name", "-p", "other-buildpack-uri"}),
				}))
			})
		})

		context("when a buildpack cannot be created", func() {
			it.Before(func() {
				executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
					executions = append(executions, execution)

					if strings.Contains(strings.Join(execution.Args, " "), "update-buildpack some-buildpack-name") {
						return errors.New("no such buildpack")
					}

					if strings.Contains(strings.Join(execution.Args, " "), "create-buildpack some-buildpack-name") {
						fmt.Fprintln(execution.Stderr, "something bad happened")
						return errors.New("create-buildpack failed")
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
				Expect(err).To(MatchError("failed to create buildpack: create-buildpack failed\n\nOutput:\nsomething bad happened\n"))
			})
		})
	})
}
