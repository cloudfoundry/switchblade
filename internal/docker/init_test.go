package docker_test

import (
	"testing"

	"github.com/onsi/gomega/format"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestDocker(t *testing.T) {
	format.MaxLength = 0

	suite := spec.New("switchblade/internal/docker", spec.Report(report.Terminal{}), spec.Parallel())
	suite("BuildpacksCache", testBuildpacksCache)
	suite("BuildpacksManager", testBuildpacksManager)
	suite("BuildpacksRegistry", testBuildpacksRegistry)
	suite("Deinitialize", testDeinitialize)
	suite("Initialize", testInitialize)
	suite("LifecycleManager", testLifecycleManager)
	suite("NetworkManager", testNetworkManager)
	suite("Setup", testSetup)
	suite("Stage", testStage)
	suite("Start", testStart)
	suite("TGZArchiver", testTGZArchiver)
	suite("Teardown", testTeardown)
	suite.Run(t)
}

type copyToContainerInvocation struct {
	ContainerID string
	DstPath     string
	Content     string
}

type copyFromContainerInvocation struct {
	ContainerID string
	SrcPath     string
}

type cacheFetchInvocation struct {
	URL string
}
