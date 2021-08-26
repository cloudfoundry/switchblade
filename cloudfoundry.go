package switchblade

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/ryanmoran/switchblade/internal/cloudfoundry"
)

//go:generate faux --interface CloudFoundryInitializePhase --output fakes/cloudfoundry_initialize_phase.go
type CloudFoundryInitializePhase interface {
	cloudfoundry.InitializePhase
}

//go:generate faux --interface CloudFoundrySetupPhase --output fakes/cloudfoundry_setup_phase.go
type CloudFoundrySetupPhase interface {
	cloudfoundry.SetupPhase
}

//go:generate faux --interface CloudFoundryStagePhase --output fakes/cloudfoundry_stage_phase.go
type CloudFoundryStagePhase interface {
	cloudfoundry.StagePhase
}

//go:generate faux --interface CloudFoundryTeardownPhase --output fakes/cloudfoundry_teardown_phase.go
type CloudFoundryTeardownPhase interface {
	cloudfoundry.TeardownPhase
}

func NewCloudFoundry(initialize cloudfoundry.InitializePhase, setup cloudfoundry.SetupPhase, stage cloudfoundry.StagePhase, teardown cloudfoundry.TeardownPhase, workspace string) Platform {
	return Platform{
		initialize: CloudFoundryInitializeProcess{initialize: initialize},
		Deploy:     CloudFoundryDeployProcess{setup: setup, stage: stage, workspace: workspace},
		Delete:     CloudFoundryDeleteProcess{teardown: teardown, workspace: workspace},
	}
}

type CloudFoundryInitializeProcess struct {
	initialize cloudfoundry.InitializePhase
}

func (p CloudFoundryInitializeProcess) Execute(buildpacks ...Buildpack) error {
	var bps []cloudfoundry.Buildpack
	for _, buildpack := range buildpacks {
		bps = append(bps, cloudfoundry.Buildpack{
			Name: buildpack.Name,
			URI:  buildpack.URI,
		})
	}

	return p.initialize.Run(bps)
}

type CloudFoundryDeployProcess struct {
	setup     cloudfoundry.SetupPhase
	stage     cloudfoundry.StagePhase
	workspace string
}

func (p CloudFoundryDeployProcess) WithBuildpacks(buildpacks ...string) DeployProcess {
	p.setup = p.setup.WithBuildpacks(buildpacks...)
	return p
}

func (p CloudFoundryDeployProcess) WithEnv(env map[string]string) DeployProcess {
	p.setup = p.setup.WithEnv(env)
	return p
}

func (p CloudFoundryDeployProcess) WithoutInternetAccess() DeployProcess {
	p.setup = p.setup.WithoutInternetAccess()
	return p
}

func (p CloudFoundryDeployProcess) WithServices(services map[string]Service) DeployProcess {
	s := make(map[string]map[string]interface{})
	for name, service := range services {
		s[name] = service
	}

	p.setup = p.setup.WithServices(s)
	return p
}

func (p CloudFoundryDeployProcess) Execute(name, source string) (Deployment, fmt.Stringer, error) {
	logs := bytes.NewBuffer(nil)
	home := filepath.Join(p.workspace, name)

	internalURL, err := p.setup.Run(logs, home, name, source)
	if err != nil {
		return Deployment{}, logs, err
	}

	externalURL, err := p.stage.Run(logs, home, name)
	if err != nil {
		return Deployment{}, logs, err
	}

	return Deployment{
		Name:        name,
		ExternalURL: externalURL,
		InternalURL: internalURL,
	}, logs, nil
}

type CloudFoundryDeleteProcess struct {
	teardown  cloudfoundry.TeardownPhase
	workspace string
}

func (p CloudFoundryDeleteProcess) Execute(name string) error {
	return p.teardown.Run(filepath.Join(p.workspace, name), name)
}
