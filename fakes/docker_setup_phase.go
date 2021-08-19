package fakes

import (
	gocontext "context"
	"io"
	"sync"

	"github.com/ryanmoran/switchblade/internal/docker"
)

type DockerSetupPhase struct {
	RunCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx  gocontext.Context
			Logs io.Writer
			Name string
			Path string
		}
		Returns struct {
			ContainerID string
			Err         error
		}
		Stub func(gocontext.Context, io.Writer, string, string) (string, error)
	}
	WithBuildpacksCall struct {
		sync.Mutex
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
		sync.Mutex
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
		sync.Mutex
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
	WithoutInternetAccessCall struct {
		sync.Mutex
		CallCount int
		Returns   struct {
			SetupPhase docker.SetupPhase
		}
		Stub func() docker.SetupPhase
	}
}

func (f *DockerSetupPhase) Run(param1 gocontext.Context, param2 io.Writer, param3 string, param4 string) (string, error) {
	f.RunCall.Lock()
	defer f.RunCall.Unlock()
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
	f.WithBuildpacksCall.Lock()
	defer f.WithBuildpacksCall.Unlock()
	f.WithBuildpacksCall.CallCount++
	f.WithBuildpacksCall.Receives.Buildpacks = param1
	if f.WithBuildpacksCall.Stub != nil {
		return f.WithBuildpacksCall.Stub(param1...)
	}
	return f.WithBuildpacksCall.Returns.SetupPhase
}
func (f *DockerSetupPhase) WithEnv(param1 map[string]string) docker.SetupPhase {
	f.WithEnvCall.Lock()
	defer f.WithEnvCall.Unlock()
	f.WithEnvCall.CallCount++
	f.WithEnvCall.Receives.Env = param1
	if f.WithEnvCall.Stub != nil {
		return f.WithEnvCall.Stub(param1)
	}
	return f.WithEnvCall.Returns.SetupPhase
}
func (f *DockerSetupPhase) WithServices(param1 map[string]map[string]interface {
}) docker.SetupPhase {
	f.WithServicesCall.Lock()
	defer f.WithServicesCall.Unlock()
	f.WithServicesCall.CallCount++
	f.WithServicesCall.Receives.Services = param1
	if f.WithServicesCall.Stub != nil {
		return f.WithServicesCall.Stub(param1)
	}
	return f.WithServicesCall.Returns.SetupPhase
}
func (f *DockerSetupPhase) WithoutInternetAccess() docker.SetupPhase {
	f.WithoutInternetAccessCall.Lock()
	defer f.WithoutInternetAccessCall.Unlock()
	f.WithoutInternetAccessCall.CallCount++
	if f.WithoutInternetAccessCall.Stub != nil {
		return f.WithoutInternetAccessCall.Stub()
	}
	return f.WithoutInternetAccessCall.Returns.SetupPhase
}
