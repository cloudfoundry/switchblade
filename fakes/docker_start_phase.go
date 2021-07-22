package fakes

import (
	"context"
	"io"
	"sync"

	"github.com/ryanmoran/switchblade/internal/docker"
)

type DockerStartPhase struct {
	RunCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx     context.Context
			Logs    io.Writer
			Name    string
			Command string
		}
		Returns struct {
			ExternalURL string
			InternalURL string
			Err         error
		}
		Stub func(context.Context, io.Writer, string, string) (string, string, error)
	}
	WithEnvCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Env map[string]string
		}
		Returns struct {
			StartPhase docker.StartPhase
		}
		Stub func(map[string]string) docker.StartPhase
	}
}

func (f *DockerStartPhase) Run(param1 context.Context, param2 io.Writer, param3 string, param4 string) (string, string, error) {
	f.RunCall.Lock()
	defer f.RunCall.Unlock()
	f.RunCall.CallCount++
	f.RunCall.Receives.Ctx = param1
	f.RunCall.Receives.Logs = param2
	f.RunCall.Receives.Name = param3
	f.RunCall.Receives.Command = param4
	if f.RunCall.Stub != nil {
		return f.RunCall.Stub(param1, param2, param3, param4)
	}
	return f.RunCall.Returns.ExternalURL, f.RunCall.Returns.InternalURL, f.RunCall.Returns.Err
}
func (f *DockerStartPhase) WithEnv(param1 map[string]string) docker.StartPhase {
	f.WithEnvCall.Lock()
	defer f.WithEnvCall.Unlock()
	f.WithEnvCall.CallCount++
	f.WithEnvCall.Receives.Env = param1
	if f.WithEnvCall.Stub != nil {
		return f.WithEnvCall.Stub(param1)
	}
	return f.WithEnvCall.Returns.StartPhase
}
