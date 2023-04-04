package fakes

import (
	"context"
	"io"
	"sync"

	"github.com/cloudfoundry/switchblade/internal/docker"
)

type DockerStartPhase struct {
	RunCall struct {
		mutex     sync.Mutex
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
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Env map[string]string
		}
		Returns struct {
			StartPhase docker.StartPhase
		}
		Stub func(map[string]string) docker.StartPhase
	}
	WithServicesCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Services map[string]map[string]interface {
			}
		}
		Returns struct {
			StartPhase docker.StartPhase
		}
		Stub func(map[string]map[string]interface {
		}) docker.StartPhase
	}
	WithStackCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Stack string
		}
		Returns struct {
			StartPhase docker.StartPhase
		}
		Stub func(string) docker.StartPhase
	}
	WithStartCommandCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Command string
		}
		Returns struct {
			StartPhase docker.StartPhase
		}
		Stub func(string) docker.StartPhase
	}
}

func (f *DockerStartPhase) Run(param1 context.Context, param2 io.Writer, param3 string, param4 string) (string, string, error) {
	f.RunCall.mutex.Lock()
	defer f.RunCall.mutex.Unlock()
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
	f.WithEnvCall.mutex.Lock()
	defer f.WithEnvCall.mutex.Unlock()
	f.WithEnvCall.CallCount++
	f.WithEnvCall.Receives.Env = param1
	if f.WithEnvCall.Stub != nil {
		return f.WithEnvCall.Stub(param1)
	}
	return f.WithEnvCall.Returns.StartPhase
}
func (f *DockerStartPhase) WithServices(param1 map[string]map[string]interface {
}) docker.StartPhase {
	f.WithServicesCall.mutex.Lock()
	defer f.WithServicesCall.mutex.Unlock()
	f.WithServicesCall.CallCount++
	f.WithServicesCall.Receives.Services = param1
	if f.WithServicesCall.Stub != nil {
		return f.WithServicesCall.Stub(param1)
	}
	return f.WithServicesCall.Returns.StartPhase
}
func (f *DockerStartPhase) WithStack(param1 string) docker.StartPhase {
	f.WithStackCall.mutex.Lock()
	defer f.WithStackCall.mutex.Unlock()
	f.WithStackCall.CallCount++
	f.WithStackCall.Receives.Stack = param1
	if f.WithStackCall.Stub != nil {
		return f.WithStackCall.Stub(param1)
	}
	return f.WithStackCall.Returns.StartPhase
}
func (f *DockerStartPhase) WithStartCommand(param1 string) docker.StartPhase {
	f.WithStartCommandCall.mutex.Lock()
	defer f.WithStartCommandCall.mutex.Unlock()
	f.WithStartCommandCall.CallCount++
	f.WithStartCommandCall.Receives.Command = param1
	if f.WithStartCommandCall.Stub != nil {
		return f.WithStartCommandCall.Stub(param1)
	}
	return f.WithStartCommandCall.Returns.StartPhase
}
