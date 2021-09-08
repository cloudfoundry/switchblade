package fakes

import (
	"context"
	"sync"
)

type TeardownNetworkManager struct {
	DeleteCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx  context.Context
			Name string
		}
		Returns struct {
			Error error
		}
		Stub func(context.Context, string) error
	}
}

func (f *TeardownNetworkManager) Delete(param1 context.Context, param2 string) error {
	f.DeleteCall.mutex.Lock()
	defer f.DeleteCall.mutex.Unlock()
	f.DeleteCall.CallCount++
	f.DeleteCall.Receives.Ctx = param1
	f.DeleteCall.Receives.Name = param2
	if f.DeleteCall.Stub != nil {
		return f.DeleteCall.Stub(param1, param2)
	}
	return f.DeleteCall.Returns.Error
}
