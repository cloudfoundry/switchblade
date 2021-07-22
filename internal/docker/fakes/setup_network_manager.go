package fakes

import (
	gocontext "context"
	"sync"
)

type SetupNetworkManager struct {
	ConnectCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         gocontext.Context
			ContainerID string
			Name        string
		}
		Returns struct {
			Error error
		}
		Stub func(gocontext.Context, string, string) error
	}
	CreateCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx      gocontext.Context
			Name     string
			Driver   string
			Internal bool
		}
		Returns struct {
			Error error
		}
		Stub func(gocontext.Context, string, string, bool) error
	}
}

func (f *SetupNetworkManager) Connect(param1 gocontext.Context, param2 string, param3 string) error {
	f.ConnectCall.Lock()
	defer f.ConnectCall.Unlock()
	f.ConnectCall.CallCount++
	f.ConnectCall.Receives.Ctx = param1
	f.ConnectCall.Receives.ContainerID = param2
	f.ConnectCall.Receives.Name = param3
	if f.ConnectCall.Stub != nil {
		return f.ConnectCall.Stub(param1, param2, param3)
	}
	return f.ConnectCall.Returns.Error
}
func (f *SetupNetworkManager) Create(param1 gocontext.Context, param2 string, param3 string, param4 bool) error {
	f.CreateCall.Lock()
	defer f.CreateCall.Unlock()
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
