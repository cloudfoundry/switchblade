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
		Stub func([]docker.Buildpack)
	}
}

func (f *DockerInitializePhase) Run(param1 []docker.Buildpack) {
	f.RunCall.mutex.Lock()
	defer f.RunCall.mutex.Unlock()
	f.RunCall.CallCount++
	f.RunCall.Receives.BuildpackSlice = param1
	if f.RunCall.Stub != nil {
		f.RunCall.Stub(param1)
	}
}
