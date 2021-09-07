package switchblade

import (
	"bytes"
	"context"
	"fmt"

	"github.com/cloudfoundry/switchblade/internal/docker"
)

//go:generate faux --interface DockerInitializePhase --output fakes/docker_initialize_phase.go
type DockerInitializePhase interface {
	docker.InitializePhase
}

//go:generate faux --interface DockerSetupPhase --output fakes/docker_setup_phase.go
type DockerSetupPhase interface {
	docker.SetupPhase
}

//go:generate faux --interface DockerStagePhase --output fakes/docker_stage_phase.go
type DockerStagePhase interface {
	docker.StagePhase
}

//go:generate faux --interface DockerStartPhase --output fakes/docker_start_phase.go
type DockerStartPhase interface {
	docker.StartPhase
}

//go:generate faux --interface DockerTeardownPhase --output fakes/docker_teardown_phase.go
type DockerTeardownPhase interface {
	docker.TeardownPhase
}

func NewDocker(initialize docker.InitializePhase, setup docker.SetupPhase, stage docker.StagePhase, start docker.StartPhase, teardown docker.TeardownPhase) Platform {
	return Platform{
		initialize: DockerInitializeProcess{initialize: initialize},
		Deploy:     DockerDeployProcess{setup: setup, stage: stage, start: start},
		Delete:     DockerDeleteProcess{teardown: teardown},
	}
}

type DockerInitializeProcess struct {
	initialize docker.InitializePhase
}

func (p DockerInitializeProcess) Execute(buildpacks ...Buildpack) error {
	var bps []docker.Buildpack
	for _, buildpack := range buildpacks {
		bps = append(bps, docker.Buildpack{
			Name: buildpack.Name,
			URI:  buildpack.URI,
		})
	}

	p.initialize.Run(bps)

	return nil
}

type DockerDeployProcess struct {
	setup docker.SetupPhase
	stage docker.StagePhase
	start docker.StartPhase
}

func (p DockerDeployProcess) WithBuildpacks(buildpacks ...string) DeployProcess {
	p.setup = p.setup.WithBuildpacks(buildpacks...)
	return p
}

func (p DockerDeployProcess) WithEnv(env map[string]string) DeployProcess {
	p.setup = p.setup.WithEnv(env)
	p.start = p.start.WithEnv(env)
	return p
}

func (p DockerDeployProcess) WithoutInternetAccess() DeployProcess {
	p.setup = p.setup.WithoutInternetAccess()
	return p
}

func (p DockerDeployProcess) WithServices(services map[string]Service) DeployProcess {
	s := make(map[string]map[string]interface{})
	for name, service := range services {
		s[name] = service
	}

	p.setup = p.setup.WithServices(s)
	p.start = p.start.WithServices(s)
	return p
}

func (p DockerDeployProcess) Execute(name, path string) (Deployment, fmt.Stringer, error) {
	ctx := context.Background()
	logs := bytes.NewBuffer(nil)

	containerID, err := p.setup.Run(ctx, logs, name, path)
	if err != nil {
		return Deployment{}, logs, fmt.Errorf("failed to run setup phase: %w\n\nOutput:\n%s", err, logs)
	}

	command, err := p.stage.Run(ctx, logs, containerID, name)
	if err != nil {
		return Deployment{}, logs, fmt.Errorf("failed to run stage phase: %w\n\nOutput:\n%s", err, logs)
	}

	externalURL, internalURL, err := p.start.Run(ctx, logs, name, command)
	if err != nil {
		return Deployment{}, logs, fmt.Errorf("failed to run start phase: %w\n\nOutput:\n%s", err, logs)
	}

	return Deployment{
		Name:        name,
		ExternalURL: externalURL,
		InternalURL: internalURL,
	}, logs, nil
}

type DockerDeleteProcess struct {
	teardown docker.TeardownPhase
}

func (p DockerDeleteProcess) Execute(name string) error {
	ctx := context.Background()

	err := p.teardown.Run(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to run teardown phase: %w", err)
	}

	return nil
}
