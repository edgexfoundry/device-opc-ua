// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2021 Schneider Electric
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"github.com/edgexfoundry/go-mod-core-contracts/v2/clients/logger"
	"testing"

	"github.com/edgexfoundry/device-opcua-go/internal/config"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

func Test_startSubscriptionListener(t *testing.T) {
	t.Run("create context and exit", func(t *testing.T) {
		d := NewProtocolDriver().(*Driver)
		d.serviceConfig = &config.ServiceConfig{}
		d.serviceConfig.OPCUAServer.Writable.Resources = "IntVarTest1"

		err := d.startSubscriptionListener()
		if err == nil {
			t.Error("expected err to exist in test environment")
		}

		d.ctxCancel()
	})
}

func Test_onIncomingDataListener(t *testing.T) {
	t.Run("set reading and exit", func(t *testing.T) {
		d := NewProtocolDriver().(*Driver)
		d.serviceConfig = &config.ServiceConfig{}
		d.serviceConfig.OPCUAServer.DeviceName = "Test"

		err := d.onIncomingDataReceived("42", "TestResource", nil)
		if err == nil {
			t.Error("expected err to exist in test environment")
		}
	})
}

func TestDriver_getClient(t *testing.T) {
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
			d := NewProtocolDriver().(*Driver)
			d.Logger = logger.MockLogger{}
			d.serviceConfig = &config.ServiceConfig{}
			_, err := d.getClient(tt.device)
			if (err != nil) != tt.wantErr {
				t.Errorf("Driver.getClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDriver_handleDataChange(t *testing.T) {
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
			d := NewProtocolDriver().(*Driver)
			d.serviceConfig = &config.ServiceConfig{}
			d.handleDataChange(tt.dcn)
		})
	}
}
