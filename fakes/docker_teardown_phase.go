package fakes

import (
	gocontext "context"
	"sync"
)

type DockerTeardownPhase struct {
	RunCall struct {
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

func (f *DockerTeardownPhase) Run(param1 gocontext.Context, param2 string) error {
	f.RunCall.Lock()
	defer f.RunCall.Unlock()
	f.RunCall.CallCount++
	f.RunCall.Receives.Ctx = param1
	f.RunCall.Receives.Name = param2
	if f.RunCall.Stub != nil {
		return f.RunCall.Stub(param1, param2)
	}
	return f.RunCall.Returns.Error
}
