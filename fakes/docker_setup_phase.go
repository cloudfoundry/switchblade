package fakes

import (
	"context"
	"io"
	"sync"

	"github.com/cloudfoundry/switchblade/internal/docker"
)

type DockerSetupPhase struct {
	RunCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx  context.Context
			Logs io.Writer
			Name string
			Path string
		}
		Returns struct {
			ContainerID string
			Err         error
		}
		Stub func(context.Context, io.Writer, string, string) (string, error)
	}
	WithBuildpacksCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Buildpacks []string
		}
		Returns struct {
			SetupPhase docker.SetupPhase
		}
		Stub func(...string) docker.SetupPhase
	}
	WithEnvCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Env map[string]string
		}
		Returns struct {
			SetupPhase docker.SetupPhase
		}
		Stub func(map[string]string) docker.SetupPhase
	}
	WithServicesCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Services map[string]map[string]interface {
			}
		}
		Returns struct {
			SetupPhase docker.SetupPhase
		}
		Stub func(map[string]map[string]interface {
		}) docker.SetupPhase
	}
	WithStackCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Stack string
		}
		Returns struct {
			SetupPhase docker.SetupPhase
		}
		Stub func(string) docker.SetupPhase
	}
	WithoutInternetAccessCall struct {
		mutex     sync.Mutex
		CallCount int
		Returns   struct {
			SetupPhase docker.SetupPhase
		}
		Stub func() docker.SetupPhase
	}
}

func (f *DockerSetupPhase) Run(param1 context.Context, param2 io.Writer, param3 string, param4 string) (string, error) {
	f.RunCall.mutex.Lock()
	defer f.RunCall.mutex.Unlock()
	f.RunCall.CallCount++
	f.RunCall.Receives.Ctx = param1
	f.RunCall.Receives.Logs = param2
	f.RunCall.Receives.Name = param3
	f.RunCall.Receives.Path = param4
	if f.RunCall.Stub != nil {
		return f.RunCall.Stub(param1, param2, param3, param4)
	}
	return f.RunCall.Returns.ContainerID, f.RunCall.Returns.Err
}
func (f *DockerSetupPhase) WithBuildpacks(param1 ...string) docker.SetupPhase {
	f.WithBuildpacksCall.mutex.Lock()
	defer f.WithBuildpacksCall.mutex.Unlock()
	f.WithBuildpacksCall.CallCount++
	f.WithBuildpacksCall.Receives.Buildpacks = param1
	if f.WithBuildpacksCall.Stub != nil {
		return f.WithBuildpacksCall.Stub(param1...)
	}
	return f.WithBuildpacksCall.Returns.SetupPhase
}
func (f *DockerSetupPhase) WithEnv(param1 map[string]string) docker.SetupPhase {
	f.WithEnvCall.mutex.Lock()
	defer f.WithEnvCall.mutex.Unlock()
	f.WithEnvCall.CallCount++
	f.WithEnvCall.Receives.Env = param1
	if f.WithEnvCall.Stub != nil {
		return f.WithEnvCall.Stub(param1)
	}
	return f.WithEnvCall.Returns.SetupPhase
}
func (f *DockerSetupPhase) WithServices(param1 map[string]map[string]interface {
}) docker.SetupPhase {
	f.WithServicesCall.mutex.Lock()
	defer f.WithServicesCall.mutex.Unlock()
	f.WithServicesCall.CallCount++
	f.WithServicesCall.Receives.Services = param1
	if f.WithServicesCall.Stub != nil {
		return f.WithServicesCall.Stub(param1)
	}
	return f.WithServicesCall.Returns.SetupPhase
}
func (f *DockerSetupPhase) WithStack(param1 string) docker.SetupPhase {
	f.WithStackCall.mutex.Lock()
	defer f.WithStackCall.mutex.Unlock()
	f.WithStackCall.CallCount++
	f.WithStackCall.Receives.Stack = param1
	if f.WithStackCall.Stub != nil {
		return f.WithStackCall.Stub(param1)
	}
	return f.WithStackCall.Returns.SetupPhase
}
func (f *DockerSetupPhase) WithoutInternetAccess() docker.SetupPhase {
	f.WithoutInternetAccessCall.mutex.Lock()
	defer f.WithoutInternetAccessCall.mutex.Unlock()
	f.WithoutInternetAccessCall.CallCount++
	if f.WithoutInternetAccessCall.Stub != nil {
		return f.WithoutInternetAccessCall.Stub()
	}
	return f.WithoutInternetAccessCall.Returns.SetupPhase
}
