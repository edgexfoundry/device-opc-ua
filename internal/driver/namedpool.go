package driver

import (
	"context"
	"fmt"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"log/slog"
	"sync"
	"time"
)

type NamedConnectionPool struct {
	isActive          bool
	pools             map[string]*UaConnectionPool
	rejectionStrategy RejectionStrategy
	clientInfo        *ClientInfo
	rwMu              sync.RWMutex
}

type RejectionStrategy string

const (
	RejectNewConnection RejectionStrategy = "reject"
	// WaitForConnection is the default behavior when pool is full
	WaitForConnection   RejectionStrategy = "wait"
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
	default:
		return nil, fmt.Errorf("No available ua client connection in the pool for %s ", info.EndpointURL)
	}
}

func (p *NamedConnectionPool) CheckUpdatesAndDoUpdate(deviceName string, info *ConnectionInfo) {

	pool, created := p.pools[deviceName]
	if !created {
		return
	}
	if !info.Equals(pool.connectionInfo) {
		p.TerminateNamedPool(deviceName)
	}
}

// Reset method aims to update clientInfo all pools are using, so pools should be recreated with that updated
// clientInfo. This method should block the creation of new pool instances.
func (p *NamedConnectionPool) Reset(info *ClientInfo) {
	p.rwMu.Lock()
	defer p.rwMu.Unlock()

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
		p.rwMu.RLocker()
		defer p.rwMu.RUnlock()
		pool = newConnectionPool(info, p.clientInfo)
		p.pools[deviceName] = pool
	}
	return pool
}

func createUaConnection(connectionInfo *ConnectionInfo, clientConfig *ClientInfo) (*opcua.Client, error) {
	var (
		endpoint          = connectionInfo.EndpointURL
		securityPolicy    = connectionInfo.SecurityPolicy
		securityMode      = connectionInfo.SecurityMode
		authType          = connectionInfo.AuthType
		authUsername      = connectionInfo.Username
		authPassword      = connectionInfo.Password
		autoReconnect     = connectionInfo.AutoReconnect
		reconnectInterval = connectionInfo.ReconnectInterval
		applicationUri    = clientConfig.ApplicationURI
		certificateFile   = clientConfig.CertFile
		privateKeyFile    = clientConfig.KeyFile
	)
	opts := []opcua.Option{
		opcua.SecurityPolicy(securityPolicy.String()),
		opcua.SecurityModeString(securityMode.String()),
		opcua.AutoReconnect(autoReconnect),
		opcua.ReconnectInterval(reconnectInterval),
	}
	// check client certificate and apply
	if len(certificateFile) > 0 && len(applicationUri) > 0 && len(privateKeyFile) > 0 {
		opts = append(opts,
			opcua.CertificateFile(certificateFile),
			opcua.PrivateKeyFile(privateKeyFile),
			opcua.ApplicationURI(applicationUri),
			opcua.ProductURI(applicationUri),
			opcua.Lifetime(3600*time.Second),
			opcua.SessionTimeout(3600*time.Second))
	}
	// authentication type
	switch authType {
	case AuthTypeAnonymous:
		opts = append(opts, opcua.AuthAnonymous())
	case AuthTypeUsername:
		opts = append(opts, opcua.AuthUsername(authUsername, authPassword))
	default:
		return nil, fmt.Errorf("auth type %s not supported yet", authType)
	}
	ctx := context.Background()
	endpoints, err := opcua.GetEndpoints(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	ep := opcua.SelectEndpoint(endpoints, securityPolicy.String(), ua.MessageSecurityModeFromString(securityMode.String()))
	if ep == nil {
		return nil, fmt.Errorf("No exact security configuration match is found, \nendpoint: %s \nsecurity policy: %s\nsecurity mode: %s",
			endpoint, securityPolicy, securityMode)
	}
	opts = append(opts, opcua.SecurityFromEndpoint(ep, ua.UserTokenTypeFromString(authType.String())))

	client, err := opcua.NewClient(endpoint, opts...)
	if err != nil {
		return nil, err
	}
	if err = client.Connect(ctx); err != nil {
		return nil, err
	}
	return client, nil
}
