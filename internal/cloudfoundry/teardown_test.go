package cloudfoundry_test

import (
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
)

func testTeardown(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("Run", func() {
		var (
			teardown cloudfoundry.Teardown

			executable *fakes.Executable
			workspace  string

			executions []pexec.Execution
		)

		it.Before(func() {
			executable = &fakes.Executable{}
			executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
				executions = append(executions, execution)
				return nil
			}

			var err error
			workspace, err = os.MkdirTemp("", "workspace")
			Expect(err).NotTo(HaveOccurred())

			err = os.MkdirAll(filepath.Join(workspace, "some-home"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			teardown = cloudfoundry.NewTeardown(executable)
		})

		it("deletes the org, security-group, and config", func() {
			err := teardown.Run(filepath.Join(workspace, "some-home"), "some-app")
			Expect(err).NotTo(HaveOccurred())

			Expect(executions).To(HaveLen(2))
			Expect(executions[0]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"delete-org", "some-app", "-f"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))
			Expect(executions[1]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"delete-security-group", "some-app", "-f"}),
				"Env":  ContainElement(fmt.Sprintf("CF_HOME=%s", filepath.Join(workspace, "some-home"))),
			}))

			Expect(filepath.Join(workspace, "some-home")).NotTo(BeADirectory())
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
					err := teardown.Run(filepath.Join(workspace, "some-home"), "some-app")
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
					err := teardown.Run(filepath.Join(workspace, "some-home"), "some-app")
					Expect(err).To(MatchError(ContainSubstring("failed to delete-security-group: exit status 1")))
					Expect(err).To(MatchError(ContainSubstring("Could not delete security group")))
				})
			})
		})
	})
}
