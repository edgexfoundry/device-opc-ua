package driver

import (
	"context"
	"fmt"
	"github.com/gopcua/opcua"
	"golang.org/x/sync/semaphore"
	"runtime"
	"sync"
)

type UaConnectionPool struct {
	terminated     bool
	connectionInfo *ConnectionInfo
	clientInfo     *ClientInfo
	semMax         *semaphore.Weighted
	pool           sync.Pool
}

func newConnectionPool(connectionInfo *ConnectionInfo, clientInfo *ClientInfo) *UaConnectionPool {
	maxSize := connectionInfo.MaxPoolSize
	maxSizePtr := &maxSize
	if maxSizePtr == nil || maxSize == 0 {
		maxSize = 1
	}
	pool := &UaConnectionPool{
		terminated:     false,
		connectionInfo: connectionInfo,
		clientInfo:     clientInfo,
		semMax:         semaphore.NewWeighted(int64(maxSize)),
		pool:           sync.Pool{},
	}
	pool.pool.New = func() interface{} {
		client, err := createUaConnection(pool.connectionInfo, pool.clientInfo)
		if err != nil {
			panic(err)
		}
		runtime.SetFinalizer(client, func(client *opcua.Client) {
			closeConnection(client)
		})
		return client
	}
	return pool
}

func (p *UaConnectionPool) Borrow() (*UaPooledClientWrapper, error) {
	_ = p.semMax.Acquire(context.Background(), 1)
	if p.terminated {
		p.semMax.Release(1)
		return nil, fmt.Errorf("pool is terminated")
	}
	client := p.pool.Get().(*opcua.Client)
	wrapper := wrapperPool.Get().(*UaPooledClientWrapper)
	wrapper.client = client
	wrapper.holder = p
	return wrapper, nil
}

func (p *UaConnectionPool) TryBorrow() *UaPooledClientWrapper {
	if !p.terminated && p.semMax.TryAcquire(1) {
		client := p.pool.Get().(*opcua.Client)
		wrapper := wrapperPool.Get().(*UaPooledClientWrapper)
		wrapper.client = client
		wrapper.holder = p
		return wrapper
	}
	return nil
}

func (p *UaConnectionPool) Return(wrapper *UaPooledClientWrapper) {
	defer p.semMax.Release(1)

	if p.terminated {
		closeConnection(wrapper.client)
		wrapper.client = nil
		wrapper.holder = nil
		wrapper = nil
		return
	}

	wrapper.holder = nil
	if wrapper.invalid {
		wrapper.invalid = false
	} else {
		client := wrapper.client
		if !checkConnection(client) {
			panic("returned client is closed")
		}
		p.pool.Put(client)
	}
	wrapper.client = nil
	wrapperPool.Put(wrapper)
}

func (p *UaConnectionPool) IsTerminated() bool {
	return p.terminated
}

func (p *UaConnectionPool) Terminate() {
	p.terminated = true
	p.clientInfo = nil
	p.connectionInfo = nil
}

func checkConnection(client *opcua.Client) bool {
	return client.State() == opcua.Connected
}

func closeConnection(client *opcua.Client) {
	if client.State() != opcua.Closed {
		_ = client.Close(context.Background())
	}
}
