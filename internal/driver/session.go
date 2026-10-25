// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
// Copyright (C) 2021 Schneider Electric
// Copyright (C) 2024 YIQISOFT
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

const sessionCloseTimeout = 5 * time.Second

// secretGetter is the subset of the EdgeX secret provider used for OPC UA credentials.
type secretGetter interface {
	GetSecret(secretName string, keys ...string) (map[string]string, error)
}

// uaSession is the closable OPC UA session held in the client cache.
type uaSession interface {
	Close(ctx context.Context) error
}

// sessionInfo is the identity of one shared OPC UA session.
type sessionInfo struct {
	Endpoint   string
	Policy     string
	Mode       string
	SecretName string
}

func (s sessionInfo) Key() string {
	return s.Endpoint + "\x00" + s.Policy + "\x00" + s.Mode + "\x00" + s.SecretName
}

// clientSlot is one OPC UA session and the EdgeX devices that use it.
// Close runs only when the last device is gone and no command is in flight,
// or from Stop. A second Close on the same session is the race gopcua documents
// around CloseSession.
type clientSlot struct {
	client   uaSession
	broken   uaSession
	refs     map[string]struct{}
	inflight int
	closing  bool
	closed   bool
	dialing  chan struct{}
}

// transportError is a failed OPC UA call, as opposed to a bad status on one node.
// Only a transport error drops the cached session.
type transportError struct {
	err error
}

func (e *transportError) Error() string {
	return e.err.Error()
}

func (e *transportError) Unwrap() error {
	return e.err
}

func transportFailure(err error) error {
	return &transportError{err: err}
}

func isTransportError(err error) bool {
	var target *transportError
	return errors.As(err, &target)
}

func (d *Driver) ensureMapsLocked() {
	if d.clients == nil {
		d.clients = make(map[string]*clientSlot)
	}
	if d.deviceKeys == nil {
		d.deviceKeys = make(map[string]string)
	}
}

func (d *Driver) serverConfig() OPCUAServerConfig {
	var cfg OPCUAServerConfig
	if d.serviceConfig != nil {
		cfg = d.serviceConfig.OPCUAServer
	}
	if cfg.Policy == "" {
		cfg.Policy = "None"
	}
	if cfg.Mode == "" {
		cfg.Mode = "None"
	}
	return cfg
}

func protocolString(protocols map[string]models.ProtocolProperties, key string) (string, bool) {
	properties, ok := protocols[Protocol]
	if !ok {
		return "", false
	}
	raw, ok := properties[key]
	if !ok || raw == nil {
		return "", false
	}
	value, ok := raw.(string)
	if !ok || value == "" {
		return "", false
	}
	return value, true
}

func (d *Driver) sessionInfo(protocols map[string]models.ProtocolProperties) (sessionInfo, error) {
	endpoint, err := FetchEndpoint(protocols)
	if err != nil {
		return sessionInfo{}, err
	}
	cfg := d.serverConfig()
	policy := cfg.Policy
	mode := cfg.Mode
	if value, ok := protocolString(protocols, Policy); ok {
		policy = value
	}
	if value, ok := protocolString(protocols, Mode); ok {
		mode = value
	}
	secret, _ := protocolString(protocols, SecretName)
	if _, ok := policies[policy]; !ok {
		return sessionInfo{}, fmt.Errorf("invalid OPC UA security policy %q", policy)
	}
	if _, ok := modes[mode]; !ok {
		return sessionInfo{}, fmt.Errorf("invalid OPC UA security mode %q", mode)
	}
	if (policy != "None" || mode != "None") && (cfg.CertFile == "" || cfg.KeyFile == "") {
		return sessionInfo{}, fmt.Errorf("CertFile and KeyFile are required when Policy or Mode is not None")
	}
	return sessionInfo{Endpoint: endpoint, Policy: policy, Mode: mode, SecretName: secret}, nil
}

func (d *Driver) subscriptionDevice(name string) bool {
	return d.serviceConfig != nil && name != "" && name == d.serviceConfig.OPCUAServer.DeviceName
}

func (d *Driver) trackDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	info, err := d.sessionInfo(protocols)
	if err != nil {
		return err
	}
	key := info.Key()
	d.mu.Lock()
	d.ensureMapsLocked()
	if prev, ok := d.deviceKeys[deviceName]; ok && prev != key {
		d.mu.Unlock()
		d.releaseDevice(deviceName)
		d.mu.Lock()
		d.ensureMapsLocked()
	}
	d.deviceKeys[deviceName] = key
	slot := d.clients[key]
	if slot == nil || slot.closed {
		slot = &clientSlot{refs: map[string]struct{}{}}
		d.clients[key] = slot
	}
	if slot.refs == nil {
		slot.refs = map[string]struct{}{}
	}
	slot.refs[deviceName] = struct{}{}
	slot.closing = false
	d.mu.Unlock()
	return nil
}

func (d *Driver) releaseDevice(deviceName string) {
	d.mu.Lock()
	d.ensureMapsLocked()
	key, ok := d.deviceKeys[deviceName]
	if !ok {
		d.mu.Unlock()
		return
	}
	delete(d.deviceKeys, deviceName)
	slot := d.clients[key]
	if slot == nil {
		d.mu.Unlock()
		return
	}
	delete(slot.refs, deviceName)
	if len(slot.refs) > 0 {
		d.mu.Unlock()
		return
	}
	slot.closing = true
	if slot.inflight > 0 {
		d.mu.Unlock()
		return
	}
	closing := detachLocked(slot)
	delete(d.clients, key)
	d.mu.Unlock()
	closeSessions(closing)
}

func (d *Driver) dialer() func(context.Context, sessionInfo) (uaSession, error) {
	if d.dial != nil {
		return d.dial
	}
	return d.dialOPCUA
}

// acquire returns the shared session for this device and counts one in-flight use.
// The caller must call endUse. The subscription listener uses the same session and
// must not Close it.
func (d *Driver) acquire(ctx context.Context, deviceName string, protocols map[string]models.ProtocolProperties) (uaSession, string, error) {
	if err := d.trackDevice(deviceName, protocols); err != nil {
		return nil, "", err
	}
	info, err := d.sessionInfo(protocols)
	if err != nil {
		return nil, "", err
	}
	key := info.Key()
	for {
		d.mu.Lock()
		if d.stopped {
			d.mu.Unlock()
			return nil, "", fmt.Errorf("service attempts to stop and unable to handle new request")
		}
		d.ensureMapsLocked()
		if d.deviceKeys[deviceName] != key {
			d.mu.Unlock()
			return nil, "", fmt.Errorf("device %s was removed while connecting", deviceName)
		}
		slot := d.clients[key]
		if slot == nil || slot.closed {
			slot = &clientSlot{refs: map[string]struct{}{deviceName: {}}}
			d.clients[key] = slot
		}
		if slot.client != nil && !slot.closing {
			slot.inflight++
			client := slot.client
			d.mu.Unlock()
			return client, key, nil
		}
		if slot.dialing != nil {
			wait := slot.dialing
			d.mu.Unlock()
			select {
			case <-wait:
				continue
			case <-ctx.Done():
				return nil, "", ctx.Err()
			}
		}
		slot.dialing = make(chan struct{})
		d.mu.Unlock()

		client, dialErr := d.dialer()(ctx, info)

		d.mu.Lock()
		close(slot.dialing)
		slot.dialing = nil
		current := d.clients[key]
		stale := current != slot
		if dialErr != nil || stale || d.stopped || slot.closing || slot.closed || len(slot.refs) == 0 || d.deviceKeys[deviceName] != key {
			d.mu.Unlock()
			if client != nil {
				_ = client.Close(ctx)
			}
			if dialErr != nil {
				return nil, "", dialErr
			}
			if stale && !d.stopped {
				continue
			}
			return nil, "", fmt.Errorf("opcua session was closed while connecting")
		}
		slot.client = client
		slot.inflight++
		d.mu.Unlock()
		return client, key, nil
	}
}

func (d *Driver) acquireClient(ctx context.Context, deviceName string, protocols map[string]models.ProtocolProperties) (*opcua.Client, string, error) {
	session, key, err := d.acquire(ctx, deviceName, protocols)
	if err != nil {
		return nil, "", err
	}
	client, ok := session.(*opcua.Client)
	if !ok {
		d.endUse(key)
		return nil, "", fmt.Errorf("unexpected OPC UA client type")
	}
	return client, key, nil
}

func (d *Driver) endUse(key string) {
	d.mu.Lock()
	d.ensureMapsLocked()
	slot := d.clients[key]
	if slot == nil {
		d.mu.Unlock()
		return
	}
	if slot.inflight > 0 {
		slot.inflight--
	}
	if slot.inflight > 0 {
		d.mu.Unlock()
		return
	}
	var closing []uaSession
	if slot.broken != nil {
		closing = append(closing, slot.broken)
		slot.broken = nil
	}
	if slot.closing || len(slot.refs) == 0 {
		closing = append(closing, detachLocked(slot)...)
		delete(d.clients, key)
	}
	d.mu.Unlock()
	closeSessions(closing)
}

// dropSession forgets a broken session so the next command dials again.
// Other cache keys are left alone. The close waits until every in-flight use ends.
func (d *Driver) dropSession(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	slot := d.clients[key]
	if slot == nil || slot.closed || slot.client == nil || slot.broken != nil {
		return
	}
	slot.broken = slot.client
	slot.client = nil
}

func detachLocked(slot *clientSlot) []uaSession {
	if slot == nil || slot.closed {
		return nil
	}
	slot.closed = true
	var out []uaSession
	if slot.client != nil {
		out = append(out, slot.client)
		slot.client = nil
	}
	if slot.broken != nil {
		out = append(out, slot.broken)
		slot.broken = nil
	}
	return out
}

func closeSessions(sessions []uaSession) {
	if len(sessions) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), sessionCloseTimeout)
	defer cancel()
	for _, session := range sessions {
		if session != nil {
			_ = session.Close(ctx)
		}
	}
}

func (d *Driver) waitKeyIdle(key string) {
	deadline := time.Now().Add(sessionCloseTimeout)
	for {
		d.mu.Lock()
		n := 0
		if slot := d.clients[key]; slot != nil {
			n = slot.inflight
		}
		d.mu.Unlock()
		if n == 0 || time.Now().After(deadline) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (d *Driver) waitIdle() {
	deadline := time.Now().Add(sessionCloseTimeout)
	for {
		d.mu.Lock()
		n := 0
		for _, slot := range d.clients {
			n += slot.inflight
		}
		d.mu.Unlock()
		if n == 0 || time.Now().After(deadline) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (d *Driver) credentials(secretName string) (string, string, error) {
	provider := d.secrets
	if provider == nil {
		if d.sdkService == nil {
			return "", "", fmt.Errorf("secret provider is not available")
		}
		provider = d.sdkService.SecretProvider()
	}
	data, err := provider.GetSecret(secretName, usernameKey, passwordKey)
	if err != nil {
		return "", "", fmt.Errorf("unable to get OPC UA credentials for secret %s: %w", secretName, err)
	}
	user := data[usernameKey]
	pass := data[passwordKey]
	if user == "" || pass == "" {
		return "", "", fmt.Errorf("secret %s must contain username and password", secretName)
	}
	return user, pass, nil
}

func (d *Driver) authOptions(ep *ua.EndpointDescription, info sessionInfo) ([]opcua.Option, error) {
	if info.SecretName == "" {
		return []opcua.Option{
			opcua.AuthAnonymous(),
			opcua.SecurityFromEndpoint(ep, ua.UserTokenTypeAnonymous),
		}, nil
	}
	user, pass, err := d.credentials(info.SecretName)
	if err != nil {
		return nil, err
	}
	return []opcua.Option{
		opcua.AuthUsername(user, pass),
		opcua.SecurityFromEndpoint(ep, ua.UserTokenTypeUserName),
	}, nil
}

func (d *Driver) dialOPCUA(ctx context.Context, info sessionInfo) (uaSession, error) {
	endpoints, err := opcua.GetEndpoints(ctx, info.Endpoint)
	if err != nil {
		return nil, err
	}
	ep, err := opcua.SelectEndpoint(endpoints, info.Policy, ua.MessageSecurityModeFromString(info.Mode))
	if err != nil {
		return nil, err
	}
	if ep == nil {
		return nil, fmt.Errorf("failed to find suitable OPC UA endpoint")
	}
	ep.EndpointURL = info.Endpoint

	opts := []opcua.Option{
		opcua.SecurityPolicy(info.Policy),
		opcua.SecurityModeString(info.Mode),
	}
	cfg := d.serverConfig()
	if info.Policy != "None" || info.Mode != "None" {
		opts = append(opts, opcua.CertificateFile(cfg.CertFile), opcua.PrivateKeyFile(cfg.KeyFile))
	}
	auth, err := d.authOptions(ep, info)
	if err != nil {
		return nil, err
	}
	opts = append(opts, auth...)

	client, err := opcua.NewClient(ep.EndpointURL, opts...)
	if err != nil {
		return nil, err
	}
	if err := client.Connect(ctx); err != nil {
		_ = client.Close(ctx)
		return nil, err
	}
	return client, nil
}
