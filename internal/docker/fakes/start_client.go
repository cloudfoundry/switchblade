package fakes

import (
	"context"
	"io"
	"sync"

	"github.com/docker/docker/api/types"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type StartClient struct {
	ContainerCreateCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx              context.Context
			Config           *container.Config
			HostConfig       *container.HostConfig
			NetworkingConfig *network.NetworkingConfig
			Platform         *v1.Platform
			ContainerName    string
		}
		Returns struct {
			ContainerCreateCreatedBody container.ContainerCreateCreatedBody
			Error                      error
		}
		Stub func(context.Context, *container.Config, *container.HostConfig, *network.NetworkingConfig, *v1.Platform, string) (container.ContainerCreateCreatedBody, error)
	}
	ContainerInspectCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			ContainerID string
		}
		Returns struct {
			ContainerJSON types.ContainerJSON
			Error         error
		}
		Stub func(context.Context, string) (types.ContainerJSON, error)
	}
	ContainerStartCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			ContainerID string
			Options     types.ContainerStartOptions
		}
		Returns struct {
			Error error
		}
		Stub func(context.Context, string, types.ContainerStartOptions) error
	}
	CopyToContainerCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			ContainerID string
			DstPath     string
			Content     io.Reader
			Options     types.CopyToContainerOptions
		}
		Returns struct {
			Error error
		}
		Stub func(context.Context, string, string, io.Reader, types.CopyToContainerOptions) error
	}
}

func (f *StartClient) ContainerCreate(param1 context.Context, param2 *container.Config, param3 *container.HostConfig, param4 *network.NetworkingConfig, param5 *v1.Platform, param6 string) (container.ContainerCreateCreatedBody, error) {
	f.ContainerCreateCall.mutex.Lock()
	defer f.ContainerCreateCall.mutex.Unlock()
	f.ContainerCreateCall.CallCount++
	f.ContainerCreateCall.Receives.Ctx = param1
	f.ContainerCreateCall.Receives.Config = param2
	f.ContainerCreateCall.Receives.HostConfig = param3
	f.ContainerCreateCall.Receives.NetworkingConfig = param4
	f.ContainerCreateCall.Receives.Platform = param5
	f.ContainerCreateCall.Receives.ContainerName = param6
	if f.ContainerCreateCall.Stub != nil {
		return f.ContainerCreateCall.Stub(param1, param2, param3, param4, param5, param6)
	}
	return f.ContainerCreateCall.Returns.ContainerCreateCreatedBody, f.ContainerCreateCall.Returns.Error
}
func (f *StartClient) ContainerInspect(param1 context.Context, param2 string) (types.ContainerJSON, error) {
	f.ContainerInspectCall.mutex.Lock()
	defer f.ContainerInspectCall.mutex.Unlock()
	f.ContainerInspectCall.CallCount++
	f.ContainerInspectCall.Receives.Ctx = param1
	f.ContainerInspectCall.Receives.ContainerID = param2
	if f.ContainerInspectCall.Stub != nil {
		return f.ContainerInspectCall.Stub(param1, param2)
	}
	return f.ContainerInspectCall.Returns.ContainerJSON, f.ContainerInspectCall.Returns.Error
}
func (f *StartClient) ContainerStart(param1 context.Context, param2 string, param3 types.ContainerStartOptions) error {
	f.ContainerStartCall.mutex.Lock()
	defer f.ContainerStartCall.mutex.Unlock()
	f.ContainerStartCall.CallCount++
	f.ContainerStartCall.Receives.Ctx = param1
	f.ContainerStartCall.Receives.ContainerID = param2
	f.ContainerStartCall.Receives.Options = param3
	if f.ContainerStartCall.Stub != nil {
		return f.ContainerStartCall.Stub(param1, param2, param3)
	}
	return f.ContainerStartCall.Returns.Error
}
func (f *StartClient) CopyToContainer(param1 context.Context, param2 string, param3 string, param4 io.Reader, param5 types.CopyToContainerOptions) error {
	f.CopyToContainerCall.mutex.Lock()
	defer f.CopyToContainerCall.mutex.Unlock()
	f.CopyToContainerCall.CallCount++
	f.CopyToContainerCall.Receives.Ctx = param1
	f.CopyToContainerCall.Receives.ContainerID = param2
	f.CopyToContainerCall.Receives.DstPath = param3
	f.CopyToContainerCall.Receives.Content = param4
	f.CopyToContainerCall.Receives.Options = param5
	if f.CopyToContainerCall.Stub != nil {
		return f.CopyToContainerCall.Stub(param1, param2, param3, param4, param5)
	}
	return f.CopyToContainerCall.Returns.Error
}
