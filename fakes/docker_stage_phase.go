package fakes

import (
	gocontext "context"
	"io"
	"sync"
)

type DockerStagePhase struct {
	RunCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         gocontext.Context
			Logs        io.Writer
			ContainerID string
			Name        string
		}
		Returns struct {
			Command string
			Err     error
		}
		Stub func(gocontext.Context, io.Writer, string, string) (string, error)
	}
}

func (f *DockerStagePhase) Run(param1 gocontext.Context, param2 io.Writer, param3 string, param4 string) (string, error) {
	f.RunCall.Lock()
	defer f.RunCall.Unlock()
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
