package fakes

import (
	"context"
	"sync"
)

type DockerTeardownPhase struct {
	RunCall struct {
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

func (f *DockerTeardownPhase) Run(param1 context.Context, param2 string) error {
	f.RunCall.mutex.Lock()
	defer f.RunCall.mutex.Unlock()
	f.RunCall.CallCount++
	f.RunCall.Receives.Ctx = param1
	f.RunCall.Receives.Name = param2
	if f.RunCall.Stub != nil {
		return f.RunCall.Stub(param1, param2)
	}
	return f.RunCall.Returns.Error
}
