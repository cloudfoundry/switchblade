package fakes

import (
	"context"
	"io"
	"sync"

	"github.com/docker/docker/api/types/container"
)

type LogsClient struct {
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
}

func (f *LogsClient) ContainerLogs(ctx context.Context, containerName string, options container.LogsOptions) (io.ReadCloser, error) {
	f.ContainerLogsCall.mutex.Lock()
	defer f.ContainerLogsCall.mutex.Unlock()
	f.ContainerLogsCall.CallCount++
	f.ContainerLogsCall.Receives.Ctx = ctx
	f.ContainerLogsCall.Receives.Container = containerName
	f.ContainerLogsCall.Receives.Options = options
	if f.ContainerLogsCall.Stub != nil {
		return f.ContainerLogsCall.Stub(ctx, containerName, options)
	}
	return f.ContainerLogsCall.Returns.ReadCloser, f.ContainerLogsCall.Returns.Error
}
