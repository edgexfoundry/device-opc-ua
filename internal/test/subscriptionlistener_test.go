// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2021 Schneider Electric
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"github.com/edgexfoundry/device-opcua-go/internal/config"
	"github.com/edgexfoundry/device-opcua-go/internal/driver"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"github.com/pkg/errors"
	"testing"
)

func Test_startSubscriptionListener(t *testing.T) {
	t.Run("create context and exit", func(t *testing.T) {
		d := driver.NewProtocolDriver().(*driver.Driver)
		d.ServiceConfig = &config.ServiceConfig{}
		d.ServiceConfig.OPCUAServer.Writable.Resources = "IntVarTest1"

		err := d.StartSubscriptionListener()
		if err == nil {
			t.Error("expected err to exist in test environment")
		}

		d.CtxCancel()
	})
}

func Test_onIncomingDataListener(t *testing.T) {
	t.Run("set reading and exit", func(t *testing.T) {
		d := driver.NewProtocolDriver().(*driver.Driver)
		d.ServiceConfig = &config.ServiceConfig{}
		d.ServiceConfig.OPCUAServer.DeviceName = "Test"

		err := d.OnIncomingDataReceived("42", "TestResource", nil)
		if err == nil {
			t.Error("expected err to exist in test environment")
		}
	})
}

// These types serve as mocks for client closing, client state retrieving and subscription cancelling operations.
type (
	mockClientCloser struct {
		error error
	}
)

type (
	mockSubCanceller struct {
		error error
	}
)

type (
	mockClient struct {
		returnState opcua.ConnState
	}
)

func (mcc mockClientCloser) Close() error                     { return mcc.error }
func (msc mockSubCanceller) Cancel(ctx context.Context) error { return msc.error }
func (mc mockClient) State() opcua.ConnState                  { return mc.returnState }

func TestCheckClientState(t *testing.T) {
	tests := []struct {
		name             string
		serviceConfig    *config.ServiceConfig
		mockClient       driver.ClientState
		secondMockClient driver.ClientState
	}{
		{
			name:          "OK - Client state Reconnecting should be extracted.",
			serviceConfig: &config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{}},
			mockClient:    mockClient{returnState: opcua.Reconnecting},
		},
		{
			name:          "OK - Client state Disconnected should be extracted.",
			serviceConfig: &config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{}},
			mockClient:    mockClient{returnState: opcua.Disconnected},
		},
		{
			name:          "OK - Client state Connected should be extracted.",
			serviceConfig: &config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{}},
			mockClient:    mockClient{returnState: opcua.Connected},
		},
		{
			name:          "OK - Client state should be ignored if client is",
			serviceConfig: &config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{}},
			mockClient:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := driver.NewProtocolDriver().(*driver.Driver)
			d.Logger = logger.MockLogger{}
			d.ServiceConfig = &config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{ConnRetryWaitTime: 1}}
			driver.HandleCurrentClientState(d, tt.mockClient)

			if tt.mockClient != nil && driver.ActualClientState != tt.mockClient.State() {
				t.Errorf("Expected ActualClientState to be %v, got %v", tt.mockClient.State(), driver.ActualClientState)
				return
			} else if tt.mockClient == nil && (driver.ActualClientState != opcua.Closed || driver.LastClientState != opcua.Closed) {
				t.Error("Expected both client states do be closed when client is nil")
				return
			}

			//reset
			driver.ActualClientState = opcua.Closed
			driver.LastClientState = opcua.Closed
		})
	}
}

func TestCloseClient(t *testing.T) {
	tests := []struct {
		name          string
		serviceConfig *config.ServiceConfig
		closer        driver.ClientCloser
		wantErr       bool
	}{
		{
			name:          "OK - Client should be closed without error.",
			serviceConfig: &config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{}},
			closer:        mockClientCloser{error: nil},
			wantErr:       false,
		},
		{
			name:          "NOK - Error while closing client should be catched and handled.",
			serviceConfig: &config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{}},
			closer:        mockClientCloser{error: errors.New("Random client closing error!")},
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := driver.NewProtocolDriver().(*driver.Driver)
			d.Logger = logger.MockLogger{}
			d.ServiceConfig = &config.ServiceConfig{}
			err := driver.CloseClientConnection(d, tt.closer)
			if (err != nil) != tt.wantErr {
				t.Errorf("Driver.getClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
func TestCancelSubscription(t *testing.T) {
	tests := []struct {
		name          string
		serviceConfig *config.ServiceConfig
		canceller     driver.SubscriptionCanceller
		wantErr       bool
	}{
		{
			name:          "OK - Subscription should be cancelled without error.",
			serviceConfig: &config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{}},
			canceller:     mockSubCanceller{error: nil},
			wantErr:       false,
		},
		{
			name:          "NOK - Error while cancelling subscription should be catched and handled.",
			serviceConfig: &config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{}},
			canceller:     mockSubCanceller{error: errors.New("Random subscription cancellation error!")},
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := driver.NewProtocolDriver().(*driver.Driver)
			ctx := context.Background()
			d.Logger = logger.MockLogger{}
			d.ServiceConfig = &config.ServiceConfig{}
			err := driver.CancelSubscription(d, tt.canceller, ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Driver.getClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDriverGetClient(t *testing.T) {
	tests := []struct {
		name          string
		serviceConfig *config.ServiceConfig
		device        models.Device
		want          *opcua.Client
		wantErr       bool
	}{
		{
			name:          "NOK - no endpoint configured",
			serviceConfig: &config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{}},
			device: models.Device{
				Protocols: make(map[string]models.ProtocolProperties),
			},
			wantErr: true,
		},
		{
			name:          "NOK - no server connection",
			serviceConfig: &config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{}},
			device: models.Device{
				Protocols: map[string]models.ProtocolProperties{
					"opcua": {"Endpoint": "opc.tcp://test"},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := driver.NewProtocolDriver().(*driver.Driver)
			d.Logger = logger.MockLogger{}
			d.ServiceConfig = &config.ServiceConfig{}
			_, err := d.GetClient(tt.device)
			if (err != nil) != tt.wantErr {
				t.Errorf("Driver.getClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDriverHandleDataChange(t *testing.T) {
	tests := []struct {
		name        string
		resourceMap map[uint32]string
		dcn         *ua.DataChangeNotification
	}{
		{
			name: "OK - no monitored items",
			dcn:  &ua.DataChangeNotification{MonitoredItems: make([]*ua.MonitoredItemNotification, 0)},
		},
		{
			name:        "OK - call onIncomingDataReceived",
			resourceMap: map[uint32]string{123456: "TestResource"},
			dcn: &ua.DataChangeNotification{
				MonitoredItems: []*ua.MonitoredItemNotification{
					{ClientHandle: 123456, Value: &ua.DataValue{Value: ua.MustVariant("42")}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := driver.NewProtocolDriver().(*driver.Driver)
			d.ServiceConfig = &config.ServiceConfig{}
			d.HandleDataChange(tt.dcn)
		})
	}
}
