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
	"github.com/gopcua/opcua"
	"sync"
)

type ClientWrapper interface {
	GetClient() *opcua.Client
	Close()
	// SetInvalid is used to mark current ua connection is no longer available, if fatal error happens plz remember to
	// mark it as invalid
	SetInvalid()
}

var wrapperPool = sync.Pool{New: func() interface{} { return new(UaPooledClientWrapper) }}

type UaPooledClientWrapper struct {
	client  *opcua.Client
	holder  *UaConnectionPool
	invalid bool
}

func (w *UaPooledClientWrapper) GetClient() *opcua.Client {
	return w.client
}

func (w *UaPooledClientWrapper) Close() {
	w.holder.Return(w)
}

func (w *UaPooledClientWrapper) SetInvalid() {
	w.invalid = true
}

// UnsafeWrapper is used to wrap directly created ua clients
type UnsafeWrapper struct {
	UaPooledClientWrapper
}

func (w *UnsafeWrapper) GetClient() *opcua.Client {
	return w.client
}

func (w *UnsafeWrapper) Close() {
	_ = w.client.Close(context.Background())
}

func (w *UnsafeWrapper) SetInvalid() {
	// do nothing
}
