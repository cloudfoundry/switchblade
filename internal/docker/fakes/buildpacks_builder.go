package fakes

import (
	"sync"

	"github.com/ryanmoran/switchblade/internal/docker"
)

type BuildpacksBuilder struct {
	BuildCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Workspace string
			Name      string
		}
		Returns struct {
			Path string
			Err  error
		}
		Stub func(string, string) (string, error)
	}
	OrderCall struct {
		sync.Mutex
		CallCount int
		Returns   struct {
			Order      string
			SkipDetect bool
			Err        error
		}
		Stub func() (string, bool, error)
	}
	WithBuildpacksCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Buildpacks []string
		}
		Returns struct {
			BuildpacksBuilder docker.BuildpacksBuilder
		}
		Stub func(...string) docker.BuildpacksBuilder
	}
}

func (f *BuildpacksBuilder) Build(param1 string, param2 string) (string, error) {
	f.BuildCall.Lock()
	defer f.BuildCall.Unlock()
	f.BuildCall.CallCount++
	f.BuildCall.Receives.Workspace = param1
	f.BuildCall.Receives.Name = param2
	if f.BuildCall.Stub != nil {
		return f.BuildCall.Stub(param1, param2)
	}
	return f.BuildCall.Returns.Path, f.BuildCall.Returns.Err
}
func (f *BuildpacksBuilder) Order() (string, bool, error) {
	f.OrderCall.Lock()
	defer f.OrderCall.Unlock()
	f.OrderCall.CallCount++
	if f.OrderCall.Stub != nil {
		return f.OrderCall.Stub()
	}
	return f.OrderCall.Returns.Order, f.OrderCall.Returns.SkipDetect, f.OrderCall.Returns.Err
}
func (f *BuildpacksBuilder) WithBuildpacks(param1 ...string) docker.BuildpacksBuilder {
	f.WithBuildpacksCall.Lock()
	defer f.WithBuildpacksCall.Unlock()
	f.WithBuildpacksCall.CallCount++
	f.WithBuildpacksCall.Receives.Buildpacks = param1
	if f.WithBuildpacksCall.Stub != nil {
		return f.WithBuildpacksCall.Stub(param1...)
	}
	return f.WithBuildpacksCall.Returns.BuildpacksBuilder
}
