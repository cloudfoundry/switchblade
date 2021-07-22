package fakes

import (
	"io"
	"sync"
)

type CloudFoundryStagePhase struct {
	RunCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Logs io.Writer
			Home string
			Name string
		}
		Returns struct {
			Url string
			Err error
		}
		Stub func(io.Writer, string, string) (string, error)
	}
}

func (f *CloudFoundryStagePhase) Run(param1 io.Writer, param2 string, param3 string) (string, error) {
	f.RunCall.Lock()
	defer f.RunCall.Unlock()
	f.RunCall.CallCount++
	f.RunCall.Receives.Logs = param1
	f.RunCall.Receives.Home = param2
	f.RunCall.Receives.Name = param3
	if f.RunCall.Stub != nil {
		return f.RunCall.Stub(param1, param2, param3)
	}
	return f.RunCall.Returns.Url, f.RunCall.Returns.Err
}
