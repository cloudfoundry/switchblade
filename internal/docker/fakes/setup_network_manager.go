package fakes

import (
	"context"
	"sync"
)

type SetupNetworkManager struct {
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
	CreateCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx      context.Context
			Name     string
			Driver   string
			Internal bool
		}
		Returns struct {
			Error error
		}
		Stub func(context.Context, string, string, bool) error
	}
}

func (f *SetupNetworkManager) Connect(param1 context.Context, param2 string, param3 string) error {
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
func (f *SetupNetworkManager) Create(param1 context.Context, param2 string, param3 string, param4 bool) error {
	f.CreateCall.mutex.Lock()
	defer f.CreateCall.mutex.Unlock()
	f.CreateCall.CallCount++
	f.CreateCall.Receives.Ctx = param1
	f.CreateCall.Receives.Name = param2
	f.CreateCall.Receives.Driver = param3
	f.CreateCall.Receives.Internal = param4
	if f.CreateCall.Stub != nil {
		return f.CreateCall.Stub(param1, param2, param3, param4)
	}
	return f.CreateCall.Returns.Error
}
