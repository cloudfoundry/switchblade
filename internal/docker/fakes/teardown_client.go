package fakes

import (
	"context"
	"sync"

	"github.com/docker/docker/api/types/container"
)

type TeardownClient struct {
	ContainerRemoveCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			ContainerID string
			Options     container.RemoveOptions
		}
		Returns struct {
			Error error
		}
		Stub func(context.Context, string, container.RemoveOptions) error
	}
}

func (f *TeardownClient) ContainerRemove(param1 context.Context, param2 string, param3 container.RemoveOptions) error {
	f.ContainerRemoveCall.mutex.Lock()
	defer f.ContainerRemoveCall.mutex.Unlock()
	f.ContainerRemoveCall.CallCount++
	f.ContainerRemoveCall.Receives.Ctx = param1
	f.ContainerRemoveCall.Receives.ContainerID = param2
	f.ContainerRemoveCall.Receives.Options = param3
	if f.ContainerRemoveCall.Stub != nil {
		return f.ContainerRemoveCall.Stub(param1, param2, param3)
	}
	return f.ContainerRemoveCall.Returns.Error
}
