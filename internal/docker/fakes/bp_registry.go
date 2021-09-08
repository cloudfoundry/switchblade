package fakes

import (
	"sync"

	"github.com/cloudfoundry/switchblade/internal/docker"
)

type BPRegistry struct {
	ListCall struct {
		mutex     sync.Mutex
		CallCount int
		Returns   struct {
			BuildpackSlice []docker.Buildpack
			Error          error
		}
		Stub func() ([]docker.Buildpack, error)
	}
	OverrideCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			BuildpackSlice []docker.Buildpack
		}
		Stub func(...docker.Buildpack)
	}
}

func (f *BPRegistry) List() ([]docker.Buildpack, error) {
	f.ListCall.mutex.Lock()
	defer f.ListCall.mutex.Unlock()
	f.ListCall.CallCount++
	if f.ListCall.Stub != nil {
		return f.ListCall.Stub()
	}
	return f.ListCall.Returns.BuildpackSlice, f.ListCall.Returns.Error
}
func (f *BPRegistry) Override(param1 ...docker.Buildpack) {
	f.OverrideCall.mutex.Lock()
	defer f.OverrideCall.mutex.Unlock()
	f.OverrideCall.CallCount++
	f.OverrideCall.Receives.BuildpackSlice = param1
	if f.OverrideCall.Stub != nil {
		f.OverrideCall.Stub(param1...)
	}
}
