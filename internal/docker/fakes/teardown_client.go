package fakes

import (
	gocontext "context"
	"sync"

	"github.com/docker/docker/api/types"
)

type TeardownClient struct {
	ContainerRemoveCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         gocontext.Context
			ContainerID string
			Options     types.ContainerRemoveOptions
		}
		Returns struct {
			Error error
		}
		Stub func(gocontext.Context, string, types.ContainerRemoveOptions) error
	}
}

func (f *TeardownClient) ContainerRemove(param1 gocontext.Context, param2 string, param3 types.ContainerRemoveOptions) error {
	f.ContainerRemoveCall.Lock()
	defer f.ContainerRemoveCall.Unlock()
	f.ContainerRemoveCall.CallCount++
	f.ContainerRemoveCall.Receives.Ctx = param1
	f.ContainerRemoveCall.Receives.ContainerID = param2
	f.ContainerRemoveCall.Receives.Options = param3
	if f.ContainerRemoveCall.Stub != nil {
		return f.ContainerRemoveCall.Stub(param1, param2, param3)
	}
	return f.ContainerRemoveCall.Returns.Error
}
