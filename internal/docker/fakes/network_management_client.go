package fakes

import (
	"context"
	"sync"

	"github.com/docker/docker/api/types/network"
)

type NetworkManagementClient struct {
	NetworkConnectCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx         context.Context
			NetworkID   string
			ContainerID string
			Config      *network.EndpointSettings
		}
		Returns struct {
			Error error
		}
		Stub func(context.Context, string, string, *network.EndpointSettings) error
	}
	NetworkCreateCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx     context.Context
			Name    string
			Options network.CreateOptions
		}
		Returns struct {
			NetworkCreateResponse network.CreateResponse
			Error                 error
		}
		Stub func(context.Context, string, network.CreateOptions) (network.CreateResponse, error)
	}
	NetworkListCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx     context.Context
			Options network.ListOptions
		}
		Returns struct {
			NetworkResourceSlice []network.Inspect
			Error                error
		}
		Stub func(context.Context, network.ListOptions) ([]network.Inspect, error)
	}
	NetworkRemoveCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			Ctx       context.Context
			NetworkID string
		}
		Returns struct {
			Error error
		}
		Stub func(context.Context, string) error
	}
}

func (f *NetworkManagementClient) NetworkConnect(param1 context.Context, param2 string, param3 string, param4 *network.EndpointSettings) error {
	f.NetworkConnectCall.mutex.Lock()
	defer f.NetworkConnectCall.mutex.Unlock()
	f.NetworkConnectCall.CallCount++
	f.NetworkConnectCall.Receives.Ctx = param1
	f.NetworkConnectCall.Receives.NetworkID = param2
	f.NetworkConnectCall.Receives.ContainerID = param3
	f.NetworkConnectCall.Receives.Config = param4
	if f.NetworkConnectCall.Stub != nil {
		return f.NetworkConnectCall.Stub(param1, param2, param3, param4)
	}
	return f.NetworkConnectCall.Returns.Error
}
func (f *NetworkManagementClient) NetworkCreate(param1 context.Context, param2 string, param3 network.CreateOptions) (network.CreateResponse, error) {
	f.NetworkCreateCall.mutex.Lock()
	defer f.NetworkCreateCall.mutex.Unlock()
	f.NetworkCreateCall.CallCount++
	f.NetworkCreateCall.Receives.Ctx = param1
	f.NetworkCreateCall.Receives.Name = param2
	f.NetworkCreateCall.Receives.Options = param3
	if f.NetworkCreateCall.Stub != nil {
		return f.NetworkCreateCall.Stub(param1, param2, param3)
	}
	return f.NetworkCreateCall.Returns.NetworkCreateResponse, f.NetworkCreateCall.Returns.Error
}
func (f *NetworkManagementClient) NetworkList(param1 context.Context, param2 network.ListOptions) ([]network.Inspect, error) {
	f.NetworkListCall.mutex.Lock()
	defer f.NetworkListCall.mutex.Unlock()
	f.NetworkListCall.CallCount++
	f.NetworkListCall.Receives.Ctx = param1
	f.NetworkListCall.Receives.Options = param2
	if f.NetworkListCall.Stub != nil {
		return f.NetworkListCall.Stub(param1, param2)
	}
	return f.NetworkListCall.Returns.NetworkResourceSlice, f.NetworkListCall.Returns.Error
}
func (f *NetworkManagementClient) NetworkRemove(param1 context.Context, param2 string) error {
	f.NetworkRemoveCall.mutex.Lock()
	defer f.NetworkRemoveCall.mutex.Unlock()
	f.NetworkRemoveCall.CallCount++
	f.NetworkRemoveCall.Receives.Ctx = param1
	f.NetworkRemoveCall.Receives.NetworkID = param2
	if f.NetworkRemoveCall.Stub != nil {
		return f.NetworkRemoveCall.Stub(param1, param2)
	}
	return f.NetworkRemoveCall.Returns.Error
}
