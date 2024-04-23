/*
 * Copyright (c) 2024.  liushenglong_8597@outlook.com.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
	if maxSize == 0 {
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
		runtime.SetFinalizer(client, func(cli *opcua.Client) {
			closeConnection(cli)
		})
		return client
	}
	return pool
}

// Borrow will block until a client is available or the pool is terminated.
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

// TryBorrow returns immediately with a wrapper if available, or nil when the pool is full.
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

// Return wrapped client to the pool
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
		closeConnection(wrapper.client)
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
