package fakes

import (
	"context"
	"io"
	"sync"
)

type DockerStagePhase struct {
	RunCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			Logs        io.Writer
			ContainerID string
			Name        string
		}
		Returns struct {
			Command string
			Err     error
		}
		Stub func(context.Context, io.Writer, string, string) (string, error)
	}
}

func (f *DockerStagePhase) Run(param1 context.Context, param2 io.Writer, param3 string, param4 string) (string, error) {
	f.RunCall.mutex.Lock()
	defer f.RunCall.mutex.Unlock()
	f.RunCall.CallCount++
	f.RunCall.Receives.Ctx = param1
	f.RunCall.Receives.Logs = param2
	f.RunCall.Receives.ContainerID = param3
	f.RunCall.Receives.Name = param4
	if f.RunCall.Stub != nil {
		return f.RunCall.Stub(param1, param2, param3, param4)
	}
	return f.RunCall.Returns.Command, f.RunCall.Returns.Err
}
