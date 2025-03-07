package fakes

import (
	"context"
	"sync"
)

type InitializeNetworkManager struct {
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

func (f *InitializeNetworkManager) Create(param1 context.Context, param2 string, param3 string, param4 bool) error {
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
