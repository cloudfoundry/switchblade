package fakes

import "sync"

type DockerDeinitializePhase struct {
	RunCall struct {
		mutex     sync.Mutex
		CallCount int
		Returns   struct {
			Error error
		}
		Stub func() error
	}
}

func (f *DockerDeinitializePhase) Run() error {
	f.RunCall.mutex.Lock()
	defer f.RunCall.mutex.Unlock()
	f.RunCall.CallCount++
	if f.RunCall.Stub != nil {
		return f.RunCall.Stub()
	}
	return f.RunCall.Returns.Error
}
