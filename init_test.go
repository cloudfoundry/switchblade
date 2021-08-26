package switchblade_test

import (
	"testing"

	"github.com/onsi/gomega/format"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestSwitchblade(t *testing.T) {
	format.MaxLength = 0

	suite := spec.New("switchblade", spec.Report(report.Terminal{}), spec.Parallel())
	suite("CloudFoundry", testCloudFoundry)
	suite("Docker", testDocker)
	suite("Platform", testPlatform)
	suite("RandomName", testRandomName)
	suite("Source", testSource)
	suite.Run(t)
}
