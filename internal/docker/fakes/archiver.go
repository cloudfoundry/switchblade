package fakes

import (
	"sync"

	"github.com/cloudfoundry/switchblade/internal/docker"
)

type Archiver struct {
	CompressCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Input  string
			Output string
		}
		Returns struct {
			Error error
		}
		Stub func(string, string) error
	}
	WithPrefixCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Prefix string
		}
		Returns struct {
			Archiver docker.Archiver
		}
		Stub func(string) docker.Archiver
	}
}

func (f *Archiver) Compress(param1 string, param2 string) error {
	f.CompressCall.Lock()
	defer f.CompressCall.Unlock()
	f.CompressCall.CallCount++
	f.CompressCall.Receives.Input = param1
	f.CompressCall.Receives.Output = param2
	if f.CompressCall.Stub != nil {
		return f.CompressCall.Stub(param1, param2)
	}
	return f.CompressCall.Returns.Error
}
func (f *Archiver) WithPrefix(param1 string) docker.Archiver {
	f.WithPrefixCall.Lock()
	defer f.WithPrefixCall.Unlock()
	f.WithPrefixCall.CallCount++
	f.WithPrefixCall.Receives.Prefix = param1
	if f.WithPrefixCall.Stub != nil {
		return f.WithPrefixCall.Stub(param1)
	}
	return f.WithPrefixCall.Returns.Archiver
}
