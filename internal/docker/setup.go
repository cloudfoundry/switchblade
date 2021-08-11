package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	CFLinuxFS3DockerImage        = "cloudfoundry/cflinuxfs3:latest"
	BuildpackAppLifecycleRepoURL = "https://github.com/cloudfoundry/buildpackapplifecycle/archive/refs/heads/master.zip"

	InternalNetworkName = "switchblade-internal"
	BridgeNetworkName   = "bridge"
)

type SetupPhase interface {
	Run(ctx context.Context, logs io.Writer, name, path string) (containerID string, err error)
	WithBuildpacks(buildpacks ...string) SetupPhase
	WithEnv(env map[string]string) SetupPhase
	WithoutInternetAccess() SetupPhase
}

//go:generate faux --interface SetupClient --output fakes/setup_client.go
type SetupClient interface {
	ImagePull(ctx context.Context, ref string, options types.ImagePullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.ContainerCreateCreatedBody, error)
	CopyToContainer(ctx context.Context, containerID, dstPath string, content io.Reader, options types.CopyToContainerOptions) error
}

//go:generate faux --interface LifecycleBuilder --output fakes/lifecycle_builder.go
type LifecycleBuilder interface {
	Build(sourceURI, workspace string) (path string, err error)
}

//go:generate faux --interface BuildpacksBuilder --output fakes/buildpacks_builder.go
type BuildpacksBuilder interface {
	Order() (order string, skipDetect bool, err error)
	Build(workspace, name string) (path string, err error)
	WithBuildpacks(buildpacks ...string) BuildpacksBuilder
}

//go:generate faux --interface Archiver --output fakes/archiver.go
type Archiver interface {
	WithPrefix(prefix string) Archiver
	Compress(input, output string) error
}

//go:generate faux --interface SetupNetworkManager --output fakes/setup_network_manager.go
type SetupNetworkManager interface {
	Create(ctx context.Context, name, driver string, internal bool) error
	Connect(ctx context.Context, containerID, name string) error
}

type Setup struct {
	client             SetupClient
	lifecycle          LifecycleBuilder
	archiver           Archiver
	buildpacks         BuildpacksBuilder
	networks           SetupNetworkManager
	workspace          string
	env                map[string]string
	disconnectInternet bool
}

func NewSetup(client SetupClient, lifecycle LifecycleBuilder, buildpacks BuildpacksBuilder, archiver Archiver, networks SetupNetworkManager, workspace string) Setup {
	return Setup{
		client:     client,
		lifecycle:  lifecycle,
		buildpacks: buildpacks,
		archiver:   archiver,
		networks:   networks,
		workspace:  workspace,
	}
}

func (s Setup) Run(ctx context.Context, logs io.Writer, name, path string) (string, error) {
	lifecycle, err := s.lifecycle.Build(BuildpackAppLifecycleRepoURL, filepath.Join(s.workspace, "lifecycle"))
	if err != nil {
		return "", fmt.Errorf("failed to build lifecycle: %w", err)
	}

	buildpacks, err := s.buildpacks.Build(filepath.Join(s.workspace, "buildpacks"), name)
	if err != nil {
		return "", fmt.Errorf("failed to build buildpacks: %w", err)
	}

	source := filepath.Join(s.workspace, "source", fmt.Sprintf("%s.tar.gz", name))
	err = s.archiver.WithPrefix("/tmp/app").Compress(path, source)
	if err != nil {
		return "", fmt.Errorf("failed to archive source code: %w", err)
	}

	pullLogs, err := s.client.ImagePull(ctx, CFLinuxFS3DockerImage, types.ImagePullOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to pull base image: %w", err)
	}
	defer pullLogs.Close()

	_, err = io.Copy(logs, pullLogs)
	if err != nil {
		return "", fmt.Errorf("failed to copy image pull logs: %w", err)
	}

	err = s.networks.Create(ctx, InternalNetworkName, "bridge", true)
	if err != nil {
		return "", fmt.Errorf("failed to create network: %w", err)
	}

	env := []string{"CF_STACK=cflinuxfs3"}
	for key, value := range s.env {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	order, skipDetect, err := s.buildpacks.Order()
	if err != nil {
		return "", fmt.Errorf("failed to determine buildpack ordering: %w", err)
	}

	containerConfig := container.Config{
		Image: CFLinuxFS3DockerImage,
		Cmd: []string{
			"/tmp/lifecycle/builder",
			fmt.Sprintf("--buildpackOrder=%s", order),
			fmt.Sprintf("--skipDetect=%t", skipDetect),
			"--buildDir=/tmp/app",
			"--outputDroplet=/tmp/droplet",
			"--outputMetadata=/tmp/result.json",
			"--buildpacksDir=/tmp/buildpacks",
		},
		User:       "vcap",
		Env:        env,
		WorkingDir: "/home/vcap",
	}

	hostConfig := container.HostConfig{
		NetworkMode: container.NetworkMode(InternalNetworkName),
	}

	resp, err := s.client.ContainerCreate(ctx, &containerConfig, &hostConfig, nil, nil, name)
	if err != nil {
		return "", fmt.Errorf("failed to create staging container: %w", err)
	}

	if !s.disconnectInternet {
		err = s.networks.Connect(ctx, resp.ID, BridgeNetworkName)
		if err != nil {
			return "", fmt.Errorf("failed to connect container to network: %w", err)
		}
	}

	for _, tarballPath := range []string{lifecycle, buildpacks, source} {
		tarball, err := os.Open(tarballPath)
		if err != nil {
			return "", fmt.Errorf("failed to open tarball: %w", err)
		}

		err = s.client.CopyToContainer(ctx, resp.ID, "/", tarball, types.CopyToContainerOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to copy tarball to container: %w", err)
		}

		err = tarball.Close()
		if err != nil && !errors.Is(err, os.ErrClosed) {
			return "", fmt.Errorf("failed to close tarball: %w", err)
		}
	}

	return resp.ID, nil
}

func (s Setup) WithBuildpacks(buildpacks ...string) SetupPhase {
	s.buildpacks = s.buildpacks.WithBuildpacks(buildpacks...)
	return s
}

func (s Setup) WithEnv(env map[string]string) SetupPhase {
	s.env = env
	return s
}

func (s Setup) WithoutInternetAccess() SetupPhase {
	s.disconnectInternet = true
	return s
}
