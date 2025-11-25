package fakes

import (
	"context"
	"io"
	"sync"

	"github.com/docker/docker/api/types/container"
)

type StageClient struct {
	ContainerLogsCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx       context.Context
			Container string
			Options   container.LogsOptions
		}
		Returns struct {
			ReadCloser io.ReadCloser
			Error      error
		}
		Stub func(context.Context, string, container.LogsOptions) (io.ReadCloser, error)
	}
	ContainerRemoveCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			ContainerID string
			Options     container.RemoveOptions
		}
		Returns struct {
			Error error
		}
		Stub func(context.Context, string, container.RemoveOptions) error
	}
	ContainerStartCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			ContainerID string
			Options     container.StartOptions
		}
		Returns struct {
			Error error
		}
		Stub func(context.Context, string, container.StartOptions) error
	}
	ContainerWaitCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			ContainerID string
			Condition   container.WaitCondition
		}
		Returns struct {
			WaitResponseChannel <-chan container.WaitResponse
			ErrorChannel        <-chan error
		}
		Stub func(context.Context, string, container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	}
	CopyFromContainerCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			ContainerID string
			SrcPath     string
		}
		Returns struct {
			ReadCloser        io.ReadCloser
			ContainerPathStat container.PathStat
			Error             error
		}
		Stub func(context.Context, string, string) (io.ReadCloser, container.PathStat, error)
	}
}

func (f *StageClient) ContainerLogs(param1 context.Context, param2 string, param3 container.LogsOptions) (io.ReadCloser, error) {
	f.ContainerLogsCall.mutex.Lock()
	defer f.ContainerLogsCall.mutex.Unlock()
	f.ContainerLogsCall.CallCount++
	f.ContainerLogsCall.Receives.Ctx = param1
	f.ContainerLogsCall.Receives.Container = param2
	f.ContainerLogsCall.Receives.Options = param3
	if f.ContainerLogsCall.Stub != nil {
		return f.ContainerLogsCall.Stub(param1, param2, param3)
	}
	return f.ContainerLogsCall.Returns.ReadCloser, f.ContainerLogsCall.Returns.Error
}
func (f *StageClient) ContainerRemove(param1 context.Context, param2 string, param3 container.RemoveOptions) error {
	f.ContainerRemoveCall.mutex.Lock()
	defer f.ContainerRemoveCall.mutex.Unlock()
	f.ContainerRemoveCall.CallCount++
	f.ContainerRemoveCall.Receives.Ctx = param1
	f.ContainerRemoveCall.Receives.ContainerID = param2
	f.ContainerRemoveCall.Receives.Options = param3
	if f.ContainerRemoveCall.Stub != nil {
		return f.ContainerRemoveCall.Stub(param1, param2, param3)
	}
	return f.ContainerRemoveCall.Returns.Error
}
func (f *StageClient) ContainerStart(param1 context.Context, param2 string, param3 container.StartOptions) error {
	f.ContainerStartCall.mutex.Lock()
	defer f.ContainerStartCall.mutex.Unlock()
	f.ContainerStartCall.CallCount++
	f.ContainerStartCall.Receives.Ctx = param1
	f.ContainerStartCall.Receives.ContainerID = param2
	f.ContainerStartCall.Receives.Options = param3
	if f.ContainerStartCall.Stub != nil {
		return f.ContainerStartCall.Stub(param1, param2, param3)
	}
	return f.ContainerStartCall.Returns.Error
}
func (f *StageClient) ContainerWait(param1 context.Context, param2 string, param3 container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	f.ContainerWaitCall.mutex.Lock()
	defer f.ContainerWaitCall.mutex.Unlock()
	f.ContainerWaitCall.CallCount++
	f.ContainerWaitCall.Receives.Ctx = param1
	f.ContainerWaitCall.Receives.ContainerID = param2
	f.ContainerWaitCall.Receives.Condition = param3
	if f.ContainerWaitCall.Stub != nil {
		return f.ContainerWaitCall.Stub(param1, param2, param3)
	}
	return f.ContainerWaitCall.Returns.WaitResponseChannel, f.ContainerWaitCall.Returns.ErrorChannel
}
func (f *StageClient) CopyFromContainer(param1 context.Context, param2 string, param3 string) (io.ReadCloser, container.PathStat, error) {
	f.CopyFromContainerCall.mutex.Lock()
	defer f.CopyFromContainerCall.mutex.Unlock()
	f.CopyFromContainerCall.CallCount++
	f.CopyFromContainerCall.Receives.Ctx = param1
	f.CopyFromContainerCall.Receives.ContainerID = param2
	f.CopyFromContainerCall.Receives.SrcPath = param3
	if f.CopyFromContainerCall.Stub != nil {
		return f.CopyFromContainerCall.Stub(param1, param2, param3)
	}
	return f.CopyFromContainerCall.Returns.ReadCloser, f.CopyFromContainerCall.Returns.ContainerPathStat, f.CopyFromContainerCall.Returns.Error
}
