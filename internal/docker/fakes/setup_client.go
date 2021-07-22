package fakes

import (
	gocontext "context"
	"io"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type SetupClient struct {
	ContainerCreateCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx              gocontext.Context
			Config           *container.Config
			HostConfig       *container.HostConfig
			NetworkingConfig *network.NetworkingConfig
			Platform         *specs.Platform
			ContainerName    string
		}
		Returns struct {
			ContainerCreateCreatedBody container.ContainerCreateCreatedBody
			Error                      error
		}
		Stub func(gocontext.Context, *container.Config, *container.HostConfig, *network.NetworkingConfig, *specs.Platform, string) (container.ContainerCreateCreatedBody, error)
	}
	CopyToContainerCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         gocontext.Context
			ContainerID string
			DstPath     string
			Content     io.Reader
			Options     types.CopyToContainerOptions
		}
		Returns struct {
			Error error
		}
		Stub func(gocontext.Context, string, string, io.Reader, types.CopyToContainerOptions) error
	}
	ImagePullCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx     gocontext.Context
			Ref     string
			Options types.ImagePullOptions
		}
		Returns struct {
			ReadCloser io.ReadCloser
			Error      error
		}
		Stub func(gocontext.Context, string, types.ImagePullOptions) (io.ReadCloser, error)
	}
}

func (f *SetupClient) ContainerCreate(param1 gocontext.Context, param2 *container.Config, param3 *container.HostConfig, param4 *network.NetworkingConfig, param5 *specs.Platform, param6 string) (container.ContainerCreateCreatedBody, error) {
	f.ContainerCreateCall.Lock()
	defer f.ContainerCreateCall.Unlock()
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
func (f *SetupClient) CopyToContainer(param1 gocontext.Context, param2 string, param3 string, param4 io.Reader, param5 types.CopyToContainerOptions) error {
	f.CopyToContainerCall.Lock()
	defer f.CopyToContainerCall.Unlock()
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
func (f *SetupClient) ImagePull(param1 gocontext.Context, param2 string, param3 types.ImagePullOptions) (io.ReadCloser, error) {
	f.ImagePullCall.Lock()
	defer f.ImagePullCall.Unlock()
	f.ImagePullCall.CallCount++
	f.ImagePullCall.Receives.Ctx = param1
	f.ImagePullCall.Receives.Ref = param2
	f.ImagePullCall.Receives.Options = param3
	if f.ImagePullCall.Stub != nil {
		return f.ImagePullCall.Stub(param1, param2, param3)
	}
	return f.ImagePullCall.Returns.ReadCloser, f.ImagePullCall.Returns.Error
}
