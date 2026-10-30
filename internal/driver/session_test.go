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
	"sync/atomic"
	"testing"

	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/gopcua/opcua/ua"
)

type fakeSession struct {
	closes atomic.Int32
}

func (f *fakeSession) Close(context.Context) error {
	f.closes.Add(1)
	return nil
}

func (f *fakeSession) closed() bool {
	return f.closes.Load() > 0
}

type secretFunc func(secretName string, keys ...string) (map[string]string, error)

func (f secretFunc) GetSecret(secretName string, keys ...string) (map[string]string, error) {
	return f(secretName, keys...)
}

func endpointProtocols(endpoint string) map[string]models.ProtocolProperties {
	return map[string]models.ProtocolProperties{
		Protocol: {Endpoint: endpoint},
	}
}

func protocolsWith(endpoint, policy, mode, secret string) map[string]models.ProtocolProperties {
	props := models.ProtocolProperties{Endpoint: endpoint}
	if policy != "" {
		props[Policy] = policy
	}
	if mode != "" {
		props[Mode] = mode
	}
	if secret != "" {
		props[SecretName] = secret
	}
	return map[string]models.ProtocolProperties{Protocol: props}
}

func testDriver() *Driver {
	return &Driver{
		Logger:        &logger.MockLogger{},
		serviceConfig: &ServiceConfig{OPCUAServer: OPCUAServerConfig{DeviceName: "sub", Policy: "None", Mode: "None"}},
	}
}

func (d *Driver) useDialCounter(t *testing.T) *atomic.Int32 {
	t.Helper()
	var dials atomic.Int32
	d.dial = func(context.Context, sessionInfo) (uaSession, error) {
		dials.Add(1)
		return &fakeSession{}, nil
	}
	return &dials
}

func TestSharedSessionClosesOnLastDevice(t *testing.T) {
	d := testDriver()
	dials := d.useDialCounter(t)
	protocols := protocolsWith("opc.tcp://plc:4840", "", "", "opcua/plc")
	if err := d.AddDevice("a", protocols, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.AddDevice("b", protocols, ""); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	first, keyA, err := d.acquire(ctx, "a", protocols)
	if err != nil {
		t.Fatal(err)
	}
	d.endUse(keyA)
	second, keyB, err := d.acquire(ctx, "b", protocols)
	if err != nil {
		t.Fatal(err)
	}
	d.endUse(keyB)
	if first != second {
		t.Fatal("devices with the same key opened two sessions")
	}
	if dials.Load() != 1 {
		t.Fatalf("dials = %d, want 1", dials.Load())
	}

	if err := d.RemoveDevice("a", protocols); err != nil {
		t.Fatal(err)
	}
	if first.(*fakeSession).closed() {
		t.Fatal("removing one device closed the shared session")
	}
	again, keyB, err := d.acquire(ctx, "b", protocols)
	if err != nil {
		t.Fatal(err)
	}
	d.endUse(keyB)
	if again != first {
		t.Fatal("read after removing the other device opened a new session")
	}
	if dials.Load() != 1 {
		t.Fatalf("dials after one removal = %d, want 1", dials.Load())
	}

	if err := d.RemoveDevice("b", protocols); err != nil {
		t.Fatal(err)
	}
	if got := first.(*fakeSession).closes.Load(); got != 1 {
		t.Fatalf("last removal closes = %d, want 1", got)
	}

	_, _, err = d.acquire(ctx, "b", protocols)
	if err != nil {
		t.Fatal(err)
	}
	if dials.Load() != 2 {
		t.Fatalf("dials after last removal = %d, want 2", dials.Load())
	}
}

func TestUpdateDeviceKeepsSessionWhileAnotherDeviceHoldsTheKey(t *testing.T) {
	d := testDriver()
	dials := d.useDialCounter(t)
	shared := protocolsWith("opc.tcp://plc:4840", "", "", "opcua/plc")
	other := protocolsWith("opc.tcp://other:4840", "", "", "opcua/plc")
	if err := d.AddDevice("a", shared, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.AddDevice("b", shared, ""); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	session, key, err := d.acquire(ctx, "a", shared)
	if err != nil {
		t.Fatal(err)
	}
	d.endUse(key)

	if err := d.UpdateDevice("a", other, ""); err != nil {
		t.Fatal(err)
	}
	if session.(*fakeSession).closed() {
		t.Fatal("moving one device closed the session the other device still holds")
	}
	held, key, err := d.acquire(ctx, "b", shared)
	if err != nil {
		t.Fatal(err)
	}
	d.endUse(key)
	if held != session || dials.Load() != 1 {
		t.Fatalf("shared session was replaced, dials=%d", dials.Load())
	}

	moved, _, err := d.acquire(ctx, "a", other)
	if err != nil {
		t.Fatal(err)
	}
	if moved == session || dials.Load() != 2 {
		t.Fatal("updated device did not open its own session")
	}
}

func TestUpdateDeviceSameKeyKeepsSession(t *testing.T) {
	d := testDriver()
	dials := d.useDialCounter(t)
	protocols := endpointProtocols("opc.tcp://plc:4840")
	if err := d.AddDevice("a", protocols, ""); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	session, key, err := d.acquire(ctx, "a", protocols)
	if err != nil {
		t.Fatal(err)
	}
	d.endUse(key)
	if err := d.UpdateDevice("a", protocols, ""); err != nil {
		t.Fatal(err)
	}
	again, key, err := d.acquire(ctx, "a", protocols)
	if err != nil {
		t.Fatal(err)
	}
	d.endUse(key)
	if again != session || session.(*fakeSession).closed() || dials.Load() != 1 {
		t.Fatal("metadata update without a key change recycled the session")
	}
}

func TestInFlightUseDelaysClose(t *testing.T) {
	d := testDriver()
	d.useDialCounter(t)
	protocols := endpointProtocols("opc.tcp://plc:4840")
	if err := d.AddDevice("a", protocols, ""); err != nil {
		t.Fatal(err)
	}
	session, key, err := d.acquire(context.Background(), "a", protocols)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.RemoveDevice("a", protocols); err != nil {
		t.Fatal(err)
	}
	if session.(*fakeSession).closed() {
		t.Fatal("session closed while a command was in flight")
	}
	d.endUse(key)
	if got := session.(*fakeSession).closes.Load(); got != 1 {
		t.Fatalf("closes after the command = %d, want 1", got)
	}
}

func TestStopClosesEachSessionOnce(t *testing.T) {
	d := testDriver()
	d.useDialCounter(t)
	firstProtocols := endpointProtocols("opc.tcp://one:4840")
	secondProtocols := endpointProtocols("opc.tcp://two:4840")
	if err := d.AddDevice("a", firstProtocols, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.AddDevice("b", secondProtocols, ""); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	first, keyA, err := d.acquire(ctx, "a", firstProtocols)
	if err != nil {
		t.Fatal(err)
	}
	d.endUse(keyA)
	second, keyB, err := d.acquire(ctx, "b", secondProtocols)
	if err != nil {
		t.Fatal(err)
	}
	d.endUse(keyB)
	if err := d.Stop(true); err != nil {
		t.Fatal(err)
	}
	if first.(*fakeSession).closes.Load() != 1 || second.(*fakeSession).closes.Load() != 1 {
		t.Fatalf("stop closes = %d and %d, want 1 and 1", first.(*fakeSession).closes.Load(), second.(*fakeSession).closes.Load())
	}
	if err := d.Stop(true); err != nil {
		t.Fatal(err)
	}
	if first.(*fakeSession).closes.Load() != 1 || second.(*fakeSession).closes.Load() != 1 {
		t.Fatal("second stop closed a session again")
	}
}

func TestListenerEndDoesNotCloseSharedSession(t *testing.T) {
	d := testDriver()
	d.serviceConfig.OPCUAServer.DeviceName = "a"
	d.useDialCounter(t)
	protocols := endpointProtocols("opc.tcp://plc:4840")
	if err := d.AddDevice("a", protocols, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.AddDevice("b", protocols, ""); err != nil {
		t.Fatal(err)
	}
	// The listener holds one in-flight use and releases it on cancel. It does not Close.
	session, key, err := d.acquire(context.Background(), "a", protocols)
	if err != nil {
		t.Fatal(err)
	}
	d.cleanup()
	d.endUse(key)
	if session.(*fakeSession).closed() {
		t.Fatal("ending the subscription listener closed the shared read session")
	}
}

func TestAnonymousDoesNotReadSecret(t *testing.T) {
	d := testDriver()
	d.secrets = secretFunc(func(string, ...string) (map[string]string, error) {
		t.Fatal("anonymous session read the secret store")
		return nil, nil
	})
	if _, err := d.authOptions(nil, sessionInfo{}); err != nil {
		t.Fatal(err)
	}
}

func TestSecretNameReadsUsernamePassword(t *testing.T) {
	d := testDriver()
	var gotName string
	d.secrets = secretFunc(func(secretName string, keys ...string) (map[string]string, error) {
		gotName = secretName
		if len(keys) != 2 || keys[0] != usernameKey || keys[1] != passwordKey {
			t.Fatalf("secret keys = %v", keys)
		}
		return map[string]string{usernameKey: "alice", passwordKey: "secret"}, nil
	})
	if _, err := d.authOptions(&ua.EndpointDescription{}, sessionInfo{SecretName: "opcua/plc"}); err != nil {
		t.Fatal(err)
	}
	if gotName != "opcua/plc" {
		t.Fatalf("secret name = %s", gotName)
	}
}

func TestServicePolicyIsFallback(t *testing.T) {
	d := testDriver()
	d.serviceConfig.OPCUAServer.Policy = "Basic256Sha256"
	d.serviceConfig.OPCUAServer.Mode = "SignAndEncrypt"
	d.serviceConfig.OPCUAServer.CertFile = "cert.pem"
	d.serviceConfig.OPCUAServer.KeyFile = "key.pem"
	info, err := d.sessionInfo(endpointProtocols("opc.tcp://plc:4840"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Policy != "Basic256Sha256" || info.Mode != "SignAndEncrypt" || info.SecretName != "" {
		t.Fatalf("fallback info = %+v", info)
	}
	overridden, err := d.sessionInfo(protocolsWith("opc.tcp://plc:4840", "None", "None", ""))
	if err != nil {
		t.Fatal(err)
	}
	if overridden.Policy != "None" || overridden.Mode != "None" {
		t.Fatalf("device policy did not override the service config: %+v", overridden)
	}
}

func TestNonStringProtocolPropertyIsRejected(t *testing.T) {
	d := testDriver()
	for _, key := range []string{Policy, Mode, SecretName} {
		protocols := endpointProtocols("opc.tcp://plc:4840")
		protocols[Protocol][key] = 123
		if _, err := d.sessionInfo(protocols); err == nil {
			t.Fatalf("%s accepted a non-string value", key)
		}
	}
	protocols := endpointProtocols("opc.tcp://plc:4840")
	protocols[Protocol][SecretName] = ""
	info, err := d.sessionInfo(protocols)
	if err != nil {
		t.Fatal(err)
	}
	if info.SecretName != "" || info.Policy != "None" || info.Mode != "None" {
		t.Fatalf("empty property was not treated as unset: %+v", info)
	}
}

func TestSecondDropReplacesBrokenSession(t *testing.T) {
	d := testDriver()
	var sessions []*fakeSession
	d.dial = func(context.Context, sessionInfo) (uaSession, error) {
		session := &fakeSession{}
		sessions = append(sessions, session)
		return session, nil
	}
	protocols := endpointProtocols("opc.tcp://plc:4840")
	if err := d.AddDevice("listener", protocols, ""); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	first, key, err := d.acquire(ctx, "listener", protocols)
	if err != nil {
		t.Fatal(err)
	}
	d.dropSession(key)
	second, key, err := d.acquire(ctx, "listener", protocols)
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatal("dial did not replace the dropped session")
	}
	d.dropSession(key)
	third, key, err := d.acquire(ctx, "listener", protocols)
	if err != nil {
		t.Fatal(err)
	}
	if third == first || third == second {
		t.Fatal("second transport failure reused a dead session")
	}
	d.endUse(key)
	d.endUse(key)
	d.endUse(key)
	if sessions[0].closes.Load() != 1 || sessions[1].closes.Load() != 1 {
		t.Fatalf("retired sessions closes = %d and %d, want 1 and 1", sessions[0].closes.Load(), sessions[1].closes.Load())
	}
	if sessions[2].closed() {
		t.Fatal("live replacement was closed while the device still holds it")
	}
}

func TestCleanupCancelsTheInstalledListener(t *testing.T) {
	d := testDriver()
	ctx, ok := d.installListenerContext()
	if !ok {
		t.Fatal("listener context was not installed")
	}
	d.cleanup()
	if ctx.Err() == nil {
		t.Fatal("cleanup left the listener context active")
	}
	d.cleanup()
}

func TestStoppedServiceDiscardsNewListener(t *testing.T) {
	d := testDriver()
	d.mu.Lock()
	d.stopped = true
	d.mu.Unlock()
	if _, ok := d.installListenerContext(); ok {
		t.Fatal("stopped service installed a listener")
	}
	d.mu.Lock()
	stored := d.ctxCancel != nil
	d.mu.Unlock()
	if stored {
		t.Fatal("stopped service stored a cancel function")
	}
}

func TestReplacingListenerCancelsThePreviousOne(t *testing.T) {
	d := testDriver()
	first, ok := d.installListenerContext()
	if !ok {
		t.Fatal("first listener was not installed")
	}
	second, ok := d.installListenerContext()
	if !ok {
		t.Fatal("replacement listener was not installed")
	}
	if first.Err() == nil {
		t.Fatal("previous listener stayed active")
	}
	if second.Err() != nil {
		t.Fatal("replacement listener was cancelled by the install")
	}
	d.cleanup()
	if second.Err() == nil {
		t.Fatal("cleanup did not cancel the replacement listener")
	}
}
