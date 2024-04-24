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
	"fmt"
	"log/slog"
	"reflect"
	"sync"
)

type NamedConnectionPool struct {
	isActive          bool
	pools             map[string]*UaConnectionPool
	rejectionStrategy RejectionStrategy
	clientInfo        *ClientInfo
	mu                sync.Mutex
}

type RejectionStrategy string

const (
	// RejectNewConnection is needed if the pool is full and new request suppose to be rejected directly
	RejectNewConnection RejectionStrategy = "reject"
	// WaitForConnection is the default behavior when pool is full
	WaitForConnection RejectionStrategy = "wait"
	// CreateNewConnection if pool is full, create a new connection directly that was not managed by pool,
	// this may lead to inefficiency even resource leakage
	CreateNewConnection RejectionStrategy = "create"
)

func New(strategy RejectionStrategy, clientInfo *ClientInfo) *NamedConnectionPool {
	return &NamedConnectionPool{
		isActive:          true,
		rejectionStrategy: strategy,
		clientInfo:        clientInfo,
		pools:             make(map[string]*UaConnectionPool),
	}
}

func (p *NamedConnectionPool) GetConnection(deviceName string, info *ConnectionInfo) (ClientWrapper, error) {

	var err error

	pool := p.getPool(deviceName, info)
	wrapper := pool.TryBorrow()
	if nil != wrapper {
		return interface{}(wrapper).(ClientWrapper), nil
	}
	switch p.rejectionStrategy {
	case CreateNewConnection:
		unsafe, e := p.getConnectionUnsafe(info)
		if nil != e {
			return nil, e
		}
		return unsafe.(ClientWrapper), nil
	case WaitForConnection:
		wrapper, err = pool.Borrow()
		if err != nil {
			return nil, err
		}
		return interface{}(wrapper).(ClientWrapper), nil
	case RejectNewConnection:
		return nil, fmt.Errorf("No available ua client connection in the pool for %s ", info.EndpointURL)
	default:
		return nil, fmt.Errorf("unknown rejection strategy %s", p.rejectionStrategy)
	}
}

func (p *NamedConnectionPool) CheckUpdatesAndDoUpdate(deviceName string, info *ConnectionInfo) {

	pool, created := p.pools[deviceName]
	if !created {
		return
	}
	if !reflect.DeepEqual(info, pool.connectionInfo) {
		p.TerminateNamedPool(deviceName)
	}
}

// Reset method aims to update clientInfo all pools are using, so pools should be recreated with that updated
// clientInfo. This method should block the creation of new pool instances.
func (p *NamedConnectionPool) Reset(info *ClientInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.clientInfo = info
	for device, pool := range p.pools {
		pool.Terminate()
		delete(p.pools, device)
	}
}

// TerminateNamedPool has two purposes:
//  1. when device is locked, the connection pool should be terminated
//  2. when device connection info is updated, the best way is to terminate the pool, in case that connection info of existing
//     connections is not updated
func (p *NamedConnectionPool) TerminateNamedPool(deviceName string) {
	pool, ok := p.pools[deviceName]
	if ok {
		pool.Terminate()
		delete(p.pools, deviceName)
	}
}

func (p *NamedConnectionPool) getConnectionUnsafe(info *ConnectionInfo) (interface{}, error) {
	slog.Warn("Get ua connection directly, it may cause performance problem, ep: " + info.EndpointURL)
	connection, err := createUaConnection(info, p.clientInfo)
	if nil != err {
		return nil, err
	}
	wrapper := &UnsafeWrapper{
		UaPooledClientWrapper{
			client:  connection,
			invalid: false,
			holder:  nil,
		},
	}
	return wrapper, nil
}

func (p *NamedConnectionPool) getPool(deviceName string, info *ConnectionInfo) *UaConnectionPool {
	pool, ok := p.pools[deviceName]
	if !ok {
		p.mu.Lock()
		defer p.mu.Unlock()
		pool, ok = p.pools[deviceName]
		if !ok {
			pool = newConnectionPool(info, p.clientInfo)
			p.pools[deviceName] = pool
		}
	}
	return pool
}
