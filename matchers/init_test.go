package matchers_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestMatchers(t *testing.T) {
	suite := spec.New("switchblade/matchers", spec.Report(report.Terminal{}), spec.Parallel())
	suite("ContainLines", testContainLines)
	suite("Server", testServe)
	suite.Run(t)
}
