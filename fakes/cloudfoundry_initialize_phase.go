package fakes

import (
	"sync"

	"github.com/cloudfoundry/switchblade/internal/cloudfoundry"
)

type CloudFoundryInitializePhase struct {
	RunCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			BuildpackSlice []cloudfoundry.Buildpack
		}
		Returns struct {
			Error error
		}
		Stub func([]cloudfoundry.Buildpack) error
	}
}

func (f *CloudFoundryInitializePhase) Run(param1 []cloudfoundry.Buildpack) error {
	f.RunCall.Lock()
	defer f.RunCall.Unlock()
	f.RunCall.CallCount++
	f.RunCall.Receives.BuildpackSlice = param1
	if f.RunCall.Stub != nil {
		return f.RunCall.Stub(param1)
	}
	return f.RunCall.Returns.Error
}
