package fakes

import (
	"io"
	"sync"

	"github.com/cloudfoundry/switchblade/internal/cloudfoundry"
)

type CloudFoundrySetupPhase struct {
	RunCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Logs   io.Writer
			Home   string
			Name   string
			Source string
		}
		Returns struct {
			Url string
			Err error
		}
		Stub func(io.Writer, string, string, string) (string, error)
	}
	WithBuildpacksCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Buildpacks []string
		}
		Returns struct {
			SetupPhase cloudfoundry.SetupPhase
		}
		Stub func(...string) cloudfoundry.SetupPhase
	}
	WithEnvCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Env map[string]string
		}
		Returns struct {
			SetupPhase cloudfoundry.SetupPhase
		}
		Stub func(map[string]string) cloudfoundry.SetupPhase
	}
	WithServicesCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Services map[string]map[string]interface {
			}
		}
		Returns struct {
			SetupPhase cloudfoundry.SetupPhase
		}
		Stub func(map[string]map[string]interface {
		}) cloudfoundry.SetupPhase
	}
	WithoutInternetAccessCall struct {
		sync.Mutex
		CallCount int
		Returns   struct {
			SetupPhase cloudfoundry.SetupPhase
		}
		Stub func() cloudfoundry.SetupPhase
	}
}

func (f *CloudFoundrySetupPhase) Run(param1 io.Writer, param2 string, param3 string, param4 string) (string, error) {
	f.RunCall.Lock()
	defer f.RunCall.Unlock()
	f.RunCall.CallCount++
	f.RunCall.Receives.Logs = param1
	f.RunCall.Receives.Home = param2
	f.RunCall.Receives.Name = param3
	f.RunCall.Receives.Source = param4
	if f.RunCall.Stub != nil {
		return f.RunCall.Stub(param1, param2, param3, param4)
	}
	return f.RunCall.Returns.Url, f.RunCall.Returns.Err
}
func (f *CloudFoundrySetupPhase) WithBuildpacks(param1 ...string) cloudfoundry.SetupPhase {
	f.WithBuildpacksCall.Lock()
	defer f.WithBuildpacksCall.Unlock()
	f.WithBuildpacksCall.CallCount++
	f.WithBuildpacksCall.Receives.Buildpacks = param1
	if f.WithBuildpacksCall.Stub != nil {
		return f.WithBuildpacksCall.Stub(param1...)
	}
	return f.WithBuildpacksCall.Returns.SetupPhase
}
func (f *CloudFoundrySetupPhase) WithEnv(param1 map[string]string) cloudfoundry.SetupPhase {
	f.WithEnvCall.Lock()
	defer f.WithEnvCall.Unlock()
	f.WithEnvCall.CallCount++
	f.WithEnvCall.Receives.Env = param1
	if f.WithEnvCall.Stub != nil {
		return f.WithEnvCall.Stub(param1)
	}
	return f.WithEnvCall.Returns.SetupPhase
}
func (f *CloudFoundrySetupPhase) WithServices(param1 map[string]map[string]interface {
}) cloudfoundry.SetupPhase {
	f.WithServicesCall.Lock()
	defer f.WithServicesCall.Unlock()
	f.WithServicesCall.CallCount++
	f.WithServicesCall.Receives.Services = param1
	if f.WithServicesCall.Stub != nil {
		return f.WithServicesCall.Stub(param1)
	}
	return f.WithServicesCall.Returns.SetupPhase
}
func (f *CloudFoundrySetupPhase) WithoutInternetAccess() cloudfoundry.SetupPhase {
	f.WithoutInternetAccessCall.Lock()
	defer f.WithoutInternetAccessCall.Unlock()
	f.WithoutInternetAccessCall.CallCount++
	if f.WithoutInternetAccessCall.Stub != nil {
		return f.WithoutInternetAccessCall.Stub()
	}
	return f.WithoutInternetAccessCall.Returns.SetupPhase
}
