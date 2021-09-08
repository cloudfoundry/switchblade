package fakes

import "sync"

type CloudFoundryTeardownPhase struct {
	RunCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Home string
			Name string
		}
		Returns struct {
			Error error
		}
		Stub func(string, string) error
	}
}

func (f *CloudFoundryTeardownPhase) Run(param1 string, param2 string) error {
	f.RunCall.mutex.Lock()
	defer f.RunCall.mutex.Unlock()
	f.RunCall.CallCount++
	f.RunCall.Receives.Home = param1
	f.RunCall.Receives.Name = param2
	if f.RunCall.Stub != nil {
		return f.RunCall.Stub(param1, param2)
	}
	return f.RunCall.Returns.Error
}
