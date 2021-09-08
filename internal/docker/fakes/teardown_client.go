package fakes

import (
	"context"
	"sync"

	"github.com/docker/docker/api/types"
)

type TeardownClient struct {
	ContainerRemoveCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			ContainerID string
			Options     types.ContainerRemoveOptions
		}
		Returns struct {
			Error error
		}
		Stub func(context.Context, string, types.ContainerRemoveOptions) error
	}
}

func (f *TeardownClient) ContainerRemove(param1 context.Context, param2 string, param3 types.ContainerRemoveOptions) error {
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
