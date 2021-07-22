package fakes

import (
	gocontext "context"
	"sync"
)

type StartNetworkManager struct {
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
}

func (f *StartNetworkManager) Connect(param1 gocontext.Context, param2 string, param3 string) error {
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
