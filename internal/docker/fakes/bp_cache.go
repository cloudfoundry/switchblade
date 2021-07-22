package fakes

import (
	"io"
	"sync"
)

type BPCache struct {
	FetchCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Url string
		}
		Returns struct {
			ReadCloser io.ReadCloser
			Error      error
		}
		Stub func(string) (io.ReadCloser, error)
	}
}

func (f *BPCache) Fetch(param1 string) (io.ReadCloser, error) {
	f.FetchCall.Lock()
	defer f.FetchCall.Unlock()
	f.FetchCall.CallCount++
	f.FetchCall.Receives.Url = param1
	if f.FetchCall.Stub != nil {
		return f.FetchCall.Stub(param1)
	}
	return f.FetchCall.Returns.ReadCloser, f.FetchCall.Returns.Error
}
