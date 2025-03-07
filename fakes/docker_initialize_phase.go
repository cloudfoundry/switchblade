package fakes

import (
	"sync"

	"github.com/cloudfoundry/switchblade/internal/docker"
)

type DockerInitializePhase struct {
	RunCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			BuildpackSlice []docker.Buildpack
		}
		Returns struct {
			Error error
		}
		Stub func([]docker.Buildpack) error
	}
}

func (f *DockerInitializePhase) Run(param1 []docker.Buildpack) error {
	f.RunCall.mutex.Lock()
	defer f.RunCall.mutex.Unlock()
	f.RunCall.CallCount++
	f.RunCall.Receives.BuildpackSlice = param1
	if f.RunCall.Stub != nil {
		return f.RunCall.Stub(param1)
	}
	return f.RunCall.Returns.Error
}
