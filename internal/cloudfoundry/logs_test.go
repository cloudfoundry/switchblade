package cloudfoundry_test

import (
	"fmt"
	"testing"

	"github.com/cloudfoundry/switchblade/internal/cloudfoundry"
	"github.com/cloudfoundry/switchblade/internal/cloudfoundry/fakes"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testLogs(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		cli *fakes.Executable
	)

	it.Before(func() {
		cli = &fakes.Executable{}
	})

	context("FetchRecentLogs", func() {
		it("fetches recent logs using cf logs --recent", func() {
			cli.ExecuteCall.Stub = func(execution pexec.Execution) error {
				if len(execution.Args) > 0 && execution.Args[0] == "logs" {
					fmt.Fprintln(execution.Stdout, "Log line 1")
					fmt.Fprintln(execution.Stdout, "Log line 2")
					fmt.Fprintln(execution.Stdout, "Log line 3")
				}
				return nil
			}

			logs, err := cloudfoundry.FetchRecentLogs(cli, "/tmp/some-home", "some-app")
			Expect(err).NotTo(HaveOccurred())
			Expect(logs).To(ContainSubstring("Log line 1"))
			Expect(logs).To(ContainSubstring("Log line 2"))
			Expect(logs).To(ContainSubstring("Log line 3"))

			Expect(cli.ExecuteCall.CallCount).To(Equal(1))
			Expect(cli.ExecuteCall.Receives.Execution.Args).To(Equal([]string{"logs", "some-app", "--recent"}))
			Expect(cli.ExecuteCall.Receives.Execution.Env).To(ContainElement(ContainSubstring("CF_HOME=/tmp/some-home")))
		})

		context("when cf logs command fails", func() {
			it.Before(func() {
				cli.ExecuteCall.Returns.Error = fmt.Errorf("cf logs failed")
			})

			it("returns an error", func() {
				_, err := cloudfoundry.FetchRecentLogs(cli, "/tmp/some-home", "some-app")
				Expect(err).To(MatchError(ContainSubstring("failed to retrieve logs")))
				Expect(err).To(MatchError(ContainSubstring("cf logs failed")))
			})
		})
	})
}
