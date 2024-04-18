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
