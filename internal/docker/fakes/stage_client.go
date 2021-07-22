package fakes

import (
	"context"
	"io"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

type StageClient struct {
	ContainerLogsCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx       context.Context
			Container string
			Options   types.ContainerLogsOptions
		}
		Returns struct {
			ReadCloser io.ReadCloser
			Error      error
		}
		Stub func(context.Context, string, types.ContainerLogsOptions) (io.ReadCloser, error)
	}
	ContainerRemoveCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			ContainerID string
			Options     types.ContainerRemoveOptions
		}
		Returns struct {
			Error error
		}
		Stub func(context.Context, string, types.ContainerRemoveOptions) error
	}
	ContainerStartCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			ContainerID string
			Options     types.ContainerStartOptions
		}
		Returns struct {
			Error error
		}
		Stub func(context.Context, string, types.ContainerStartOptions) error
	}
	ContainerWaitCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			ContainerID string
			Condition   container.WaitCondition
		}
		Returns struct {
			ContainerWaitOKBodyChannel <-chan container.ContainerWaitOKBody
			ErrorChannel               <-chan error
		}
		Stub func(context.Context, string, container.WaitCondition) (<-chan container.ContainerWaitOKBody, <-chan error)
	}
	CopyFromContainerCall struct {
		sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			ContainerID string
			SrcPath     string
		}
		Returns struct {
			ReadCloser        io.ReadCloser
			ContainerPathStat types.ContainerPathStat
			Error             error
		}
		Stub func(context.Context, string, string) (io.ReadCloser, types.ContainerPathStat, error)
	}
}

func (f *StageClient) ContainerLogs(param1 context.Context, param2 string, param3 types.ContainerLogsOptions) (io.ReadCloser, error) {
	f.ContainerLogsCall.Lock()
	defer f.ContainerLogsCall.Unlock()
	f.ContainerLogsCall.CallCount++
	f.ContainerLogsCall.Receives.Ctx = param1
	f.ContainerLogsCall.Receives.Container = param2
	f.ContainerLogsCall.Receives.Options = param3
	if f.ContainerLogsCall.Stub != nil {
		return f.ContainerLogsCall.Stub(param1, param2, param3)
	}
	return f.ContainerLogsCall.Returns.ReadCloser, f.ContainerLogsCall.Returns.Error
}
func (f *StageClient) ContainerRemove(param1 context.Context, param2 string, param3 types.ContainerRemoveOptions) error {
	f.ContainerRemoveCall.Lock()
	defer f.ContainerRemoveCall.Unlock()
	f.ContainerRemoveCall.CallCount++
	f.ContainerRemoveCall.Receives.Ctx = param1
	f.ContainerRemoveCall.Receives.ContainerID = param2
	f.ContainerRemoveCall.Receives.Options = param3
	if f.ContainerRemoveCall.Stub != nil {
		return f.ContainerRemoveCall.Stub(param1, param2, param3)
	}
	return f.ContainerRemoveCall.Returns.Error
}
func (f *StageClient) ContainerStart(param1 context.Context, param2 string, param3 types.ContainerStartOptions) error {
	f.ContainerStartCall.Lock()
	defer f.ContainerStartCall.Unlock()
	f.ContainerStartCall.CallCount++
	f.ContainerStartCall.Receives.Ctx = param1
	f.ContainerStartCall.Receives.ContainerID = param2
	f.ContainerStartCall.Receives.Options = param3
	if f.ContainerStartCall.Stub != nil {
		return f.ContainerStartCall.Stub(param1, param2, param3)
	}
	return f.ContainerStartCall.Returns.Error
}
func (f *StageClient) ContainerWait(param1 context.Context, param2 string, param3 container.WaitCondition) (<-chan container.ContainerWaitOKBody, <-chan error) {
	f.ContainerWaitCall.Lock()
	defer f.ContainerWaitCall.Unlock()
	f.ContainerWaitCall.CallCount++
	f.ContainerWaitCall.Receives.Ctx = param1
	f.ContainerWaitCall.Receives.ContainerID = param2
	f.ContainerWaitCall.Receives.Condition = param3
	if f.ContainerWaitCall.Stub != nil {
		return f.ContainerWaitCall.Stub(param1, param2, param3)
	}
	return f.ContainerWaitCall.Returns.ContainerWaitOKBodyChannel, f.ContainerWaitCall.Returns.ErrorChannel
}
func (f *StageClient) CopyFromContainer(param1 context.Context, param2 string, param3 string) (io.ReadCloser, types.ContainerPathStat, error) {
	f.CopyFromContainerCall.Lock()
	defer f.CopyFromContainerCall.Unlock()
	f.CopyFromContainerCall.CallCount++
	f.CopyFromContainerCall.Receives.Ctx = param1
	f.CopyFromContainerCall.Receives.ContainerID = param2
	f.CopyFromContainerCall.Receives.SrcPath = param3
	if f.CopyFromContainerCall.Stub != nil {
		return f.CopyFromContainerCall.Stub(param1, param2, param3)
	}
	return f.CopyFromContainerCall.Returns.ReadCloser, f.CopyFromContainerCall.Returns.ContainerPathStat, f.CopyFromContainerCall.Returns.Error
}
