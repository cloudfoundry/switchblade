package fakes

import "sync"

type LifecycleBuilder struct {
	BuildCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			SourceURI string
			Workspace string
		}
		Returns struct {
			Path string
			Err  error
		}
		Stub func(string, string) (string, error)
	}
}

func (f *LifecycleBuilder) Build(param1 string, param2 string) (string, error) {
	f.BuildCall.mutex.Lock()
	defer f.BuildCall.mutex.Unlock()
	f.BuildCall.CallCount++
	f.BuildCall.Receives.SourceURI = param1
	f.BuildCall.Receives.Workspace = param2
	if f.BuildCall.Stub != nil {
		return f.BuildCall.Stub(param1, param2)
	}
	return f.BuildCall.Returns.Path, f.BuildCall.Returns.Err
}
