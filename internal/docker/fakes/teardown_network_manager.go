package fakes

import (
	gocontext "context"
	"sync"
)

type TeardownNetworkManager struct {
	DeleteCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx  gocontext.Context
			Name string
		}
		Returns struct {
			Error error
		}
		Stub func(gocontext.Context, string) error
	}
}

func (f *TeardownNetworkManager) Delete(param1 gocontext.Context, param2 string) error {
	f.DeleteCall.Lock()
	defer f.DeleteCall.Unlock()
	f.DeleteCall.CallCount++
	f.DeleteCall.Receives.Ctx = param1
	f.DeleteCall.Receives.Name = param2
	if f.DeleteCall.Stub != nil {
		return f.DeleteCall.Stub(param1, param2)
	}
	return f.DeleteCall.Returns.Error
}
