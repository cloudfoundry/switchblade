package cloudfoundry_test

import (
	"testing"

	"github.com/onsi/gomega/format"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestCloudFoundry(t *testing.T) {
	format.MaxLength = 0

	suite := spec.New("switchblade/internal/cloudfoundry", spec.Report(report.Terminal{}), spec.Parallel())
	suite("Initialize", testInitialize)
	suite("Setup", testSetup)
	suite("Stage", testStage)
	suite("Teardown", testTeardown)
	suite.Run(t)
}
