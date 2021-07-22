package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

type StagePhase interface {
	Run(ctx context.Context, logs io.Writer, containerID, name string) (command string, err error)
}

//go:generate faux --interface StageClient --output fakes/stage_client.go
type StageClient interface {
	ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.ContainerWaitOKBody, <-chan error)
	ContainerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error)
	CopyFromContainer(ctx context.Context, containerID, srcPath string) (io.ReadCloser, types.ContainerPathStat, error)
	ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error
}

type Stage struct {
	client    StageClient
	workspace string
}

func NewStage(client StageClient, workspace string) Stage {
	return Stage{
		client:    client,
		workspace: workspace,
	}
}

func (s Stage) Run(ctx context.Context, logs io.Writer, containerID, name string) (string, error) {
	err := s.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		panic(err)
	}

	var status container.ContainerWaitOKBody
	onExit, onErr := s.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-onErr:
		if err != nil {
			panic(err)
		}
	case status = <-onExit:
	}

	containerLogs, err := s.client.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		panic(err)
	}
	defer containerLogs.Close()

	_, err = stdcopy.StdCopy(logs, logs, containerLogs)
	if err != nil {
		panic(err)
	}

	if status.StatusCode != 0 {
		err = s.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			panic(err)
		}

		return "", fmt.Errorf("App staging failed: container exited with non-zero status code (%d)", status.StatusCode)
	}

	droplet, _, err := s.client.CopyFromContainer(ctx, containerID, "/tmp/droplet")
	if err != nil {
		panic(err)
	}
	defer droplet.Close()

	err = os.MkdirAll(filepath.Join(s.workspace, "droplets"), os.ModePerm)
	if err != nil {
		panic(err)
	}

	dropletFile, err := os.Create(filepath.Join(s.workspace, "droplets", fmt.Sprintf("%s.tar.gz", name)))
	if err != nil {
		panic(err)
	}
	defer dropletFile.Close()

	tr := tar.NewReader(droplet)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		if hdr.Name == "droplet" {
			_, err = io.Copy(dropletFile, tr)
			if err != nil {
				panic(err)
			}
		}
	}

	result, _, err := s.client.CopyFromContainer(ctx, containerID, "/tmp/result.json")
	if err != nil {
		panic(err)
	}
	defer result.Close()

	buffer := bytes.NewBuffer(nil)

	tr = tar.NewReader(result)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		if hdr.Name == "result.json" {
			_, err = io.Copy(buffer, tr)
			if err != nil {
				panic(err)
			}
		}
	}

	var resultContent struct {
		Processes []struct {
			Type    string `json:"type"`
			Command string `json:"command"`
		} `json:"processes"`
	}
	err = json.NewDecoder(buffer).Decode(&resultContent)
	if err != nil {
		panic(err)
	}

	var command string
	for _, process := range resultContent.Processes {
		if process.Type == "web" {
			command = process.Command
		}
	}

	err = s.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
	if err != nil {
		panic(err)
	}

	return command, nil
}
