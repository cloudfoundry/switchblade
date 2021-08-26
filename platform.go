package switchblade

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/client"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/ryanmoran/switchblade/internal/cloudfoundry"
	"github.com/ryanmoran/switchblade/internal/docker"
)

type Buildpack struct {
	Name string
	URI  string
}

type Service map[string]interface{}

type Platform struct {
	initialize InitializeProcess

	Deploy DeployProcess
	Delete DeleteProcess
}

type DeployProcess interface {
	WithBuildpacks(buildpacks ...string) DeployProcess
	WithEnv(env map[string]string) DeployProcess
	WithoutInternetAccess() DeployProcess
	WithServices(map[string]Service) DeployProcess

	Execute(name, path string) (Deployment, fmt.Stringer, error)
}

type DeleteProcess interface {
	Execute(name string) error
}

type InitializeProcess interface {
	Execute(buildpacks ...Buildpack) error
}

type PlatformType string

const (
	CloudFoundry PlatformType = "cf"
	Docker       PlatformType = "docker"
)

func NewPlatform(platformType PlatformType, token string) (Platform, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Platform{}, err
	}

	switch platformType {
	case CloudFoundry:
		cli := pexec.NewExecutable("cf")

		initialize := cloudfoundry.NewInitialize(cli)
		setup := cloudfoundry.NewSetup(cli, filepath.Join(home, ".cf"))
		stage := cloudfoundry.NewStage(cli)
		teardown := cloudfoundry.NewTeardown(cli)

		return NewCloudFoundry(initialize, setup, stage, teardown, os.TempDir()), nil
	case Docker:
		client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return Platform{}, err
		}

		workspace := filepath.Join(home, ".switchblade")

		golang := pexec.NewExecutable("go")
		archiver := docker.NewTGZArchiver()
		lifecycleManager := docker.NewLifecycleManager(golang, archiver)
		buildpacksCache := docker.NewBuildpacksCache(filepath.Join(workspace, "buildpacks-cache"))
		buildpacksRegistry := docker.NewBuildpacksRegistry("https://api.github.com", token)
		buildpacksManager := docker.NewBuildpacksManager(archiver, buildpacksCache, buildpacksRegistry)
		networkManager := docker.NewNetworkManager(client)

		initialize := docker.NewInitialize(buildpacksRegistry)
		setup := docker.NewSetup(client, lifecycleManager, buildpacksManager, archiver, networkManager, workspace)
		stage := docker.NewStage(client, archiver, workspace)
		start := docker.NewStart(client, networkManager, workspace)
		teardown := docker.NewTeardown(client, networkManager, workspace)

		return NewDocker(initialize, setup, stage, start, teardown), nil
	}

	return Platform{}, fmt.Errorf("unknown platform type: %q", platformType)
}

func (p Platform) Initialize(buildpacks ...Buildpack) error {
	return p.initialize.Execute(buildpacks...)
}
