package fakes

import (
	"context"
	"sync"
)

type StartNetworkManager struct {
	ConnectCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			ContainerID string
			Name        string
		}
		Returns struct {
			Error error
		}
		Stub func(context.Context, string, string) error
	}
}

func (f *StartNetworkManager) Connect(param1 context.Context, param2 string, param3 string) error {
	f.ConnectCall.mutex.Lock()
	defer f.ConnectCall.mutex.Unlock()
	f.ConnectCall.CallCount++
	f.ConnectCall.Receives.Ctx = param1
	f.ConnectCall.Receives.ContainerID = param2
	f.ConnectCall.Receives.Name = param3
	if f.ConnectCall.Stub != nil {
		return f.ConnectCall.Stub(param1, param2, param3)
	}
	return f.ConnectCall.Returns.Error
}
