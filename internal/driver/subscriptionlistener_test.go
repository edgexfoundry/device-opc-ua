// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2021 Schneider Electric
// Copyright (C) 2024 YIQISOFT
// Copyright (C) 2024 liushenglong_8597@outlook.com
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"fmt"
	"github.com/edgexfoundry/device-opc-ua/internal/mock"
	"github.com/edgexfoundry/device-opc-ua/internal/test"
	"github.com/edgexfoundry/device-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/gopcua/opcua/ua"
)

func initDriverWithWatchableResources(clientPkPath string, clientCertPath string) *Driver {
	testDriver := NewProtocolDriver().(*Driver)
	sdkService := mock.NewDeviceSdk()
	sdkService.(*mock.DeviceServiceSdk).LoadCustomConfigImpl = func(customConfig interfaces.UpdatableConfig, sectionName string) error {
		customConfig.(*ServiceConfig).OPCUAServer = ClientInfo{
			CertFile: clientCertPath,
			KeyFile:  clientPkPath,
		}
		return nil
	}
	_, _ = sdkService.AddDeviceProfile(models.DeviceProfile{
		Description: "Test Profile(Watchable)",
		Name:        "test_watchable_pf_1",
		DeviceResources: []models.DeviceResource{{
			Name:     "IntVarTest1",
			IsHidden: false,
			Properties: models.ResourceProperties{
				ValueType: common.ValueTypeFloat32,
			},
			Attributes: map[string]interface{}{
				WATCHABLE: true,
				NODE:      "",
			},
		}},
		DeviceCommands: nil,
	})

	_, _ = sdkService.AddDevice(models.Device{
		Name:           "IntVarTest1_Device",
		AdminState:     models.Unlocked,
		OperatingState: models.Up,
		Protocols: map[string]models.ProtocolProperties{
			Protocol: {
				EndpointField:       test.Protocol + test.Address,
				SecurityPolicyField: SecurityPolicyNone,
			},
		},
		ServiceName: "device-opcua",
		ProfileName: "test_watchable_pf_1",
		AutoEvents:  make([]models.AutoEvent, 0),
	})

	err := driver.Initialize(sdkService)
	if err != nil {
		panic(err)
	}
	fmt.Println("test driver initiated")
	return testDriver
}

func Test_existingWatchableResourceSubscribed(t *testing.T) {
	certs, err := test.CreateCerts()
	if err != nil {
		t.Errorf("Failed to create certificates: %v", err)
	}
	defer test.Clean(certs)
	server := test.NewServer("../test/opcua_server.py", certs.ServerPKPath, certs.ServerDERCertPath)
	defer server.Close()
	t.Run("OK - ExistingWatchableResourceSubscribed", func(t *testing.T) {
		d := initDriverWithWatchableResources(certs.ClientPKPath, certs.ClientPEMCertPath)
		time.Sleep(1 * time.Second)
		assert.Len(t, d.ctxCancel, 1)
		assert.Len(t, d.resourceMap, 1)
		d.cleanupListeners()
	})
}

func Test_AddWatchableDevice(t *testing.T) {
	props := map[string]models.ProtocolProperties{
		Protocol: {
			EndpointField:       test.Protocol + test.Address,
			SecurityPolicyField: SecurityPolicyNone,
		},
	}
	device := models.Device{
		Name:           "IntVarTest2_Device",
		AdminState:     models.Unlocked,
		OperatingState: models.Up,
		Protocols:      props,
		ServiceName:    "device-opcua",
		ProfileName:    "test_watchable_pf_1",
		AutoEvents:     make([]models.AutoEvent, 0),
	}

	certs, err := test.CreateCerts()
	if err != nil {
		t.Errorf("Failed to create certificates: %v", err)
	}
	defer test.Clean(certs)

	server := test.NewServer("../test/opcua_server.py", certs.ServerPKPath, certs.ServerDERCertPath)
	defer server.Close()
	t.Run("OK - AddWatchableDevice", func(t *testing.T) {
		d := initDriverWithWatchableResources(certs.ClientPKPath, certs.ClientPEMCertPath)
		_, _ = d.sdkService.AddDevice(device)
		if err := d.AddDevice(device.Name, props, device.AdminState); err != nil {
			t.Errorf("AddDevice failed")
		}
		time.Sleep(1 * time.Second)
		assert.Len(t, d.ctxCancel, 2)
		assert.Len(t, d.resourceMap, 2)
		d.cleanupListeners()
	})
}

func Test_CtxRemovedWhenDeviceRemoved(t *testing.T) {
	certs, err := test.CreateCerts()
	if err != nil {
		t.Errorf("Failed to create certificates: %v", err)
	}
	defer test.Clean(certs)

	server := test.NewServer("../test/opcua_server.py", certs.ServerPKPath, certs.ServerDERCertPath)
	defer server.Close()
	t.Run("OK - CtxRemovedWhenDeviceRemoved", func(t *testing.T) {
		d := initDriverWithWatchableResources(certs.ClientPKPath, certs.ClientPEMCertPath)

		time.Sleep(100 * time.Millisecond)
		assert.Len(t, d.ctxCancel, 1)
		d.RemoveDevice("IntVarTest1_Device", make(map[string]models.ProtocolProperties))
		time.Sleep(100 * time.Millisecond)
		assert.Len(t, d.ctxCancel, 0)
		d.cleanupListeners()
	})
}

func Test_onIncomingDataListener(t *testing.T) {
	certs, err := test.CreateCerts()
	if err != nil {
		t.Errorf("Failed to create certificates: %v", err)
	}
	defer test.Clean(certs)

	server := test.NewServer("../test/opcua_server.py", certs.ServerPKPath, certs.ServerDERCertPath)
	defer server.Close()
	t.Run("set reading and exit", func(t *testing.T) {
		d := initDriverWithWatchableResources(certs.ClientPKPath, certs.ClientPEMCertPath)

		go func() {
			select {
			case value := <-d.sdkService.AsyncValuesChannel():
				fmt.Printf("value: %v\n", value)
			default:
				fmt.Printf("no value\n")
			}
		}()

		err := d.onIncomingDataReceived(42.00, ResourceDetail{
			deviceName: "IntVarTest1_Device",
			command: &CommandInfo{
				resourceName: "test_watchable_pf_1",
				watchable:    true,
				nodeId:       "",
				methodId:     "",
				objectId:     "",
				inputMap:     nil,
				valueType:    common.ValueTypeFloat32,
			},
		})
		if err != nil {
			t.Error("onIncomingDataReceived failed")
		}
		time.Sleep(100 * time.Millisecond)
		d.cleanupListeners()
	})
}

func TestDriver_handleDataChange(t *testing.T) {
	tests := []struct {
		name        string
		resourceMap map[uint32]ResourceDetail
		dcn         *ua.DataChangeNotification
	}{
		{
			name:        "OK - no monitored items",
			resourceMap: make(map[uint32]ResourceDetail),
			dcn:         &ua.DataChangeNotification{MonitoredItems: make([]*ua.MonitoredItemNotification, 0)},
		},
		{
			name: "OK - call onIncomingDataReceived",
			resourceMap: map[uint32]ResourceDetail{123456: {
				deviceName: "TestResource",
				command: &CommandInfo{
					resourceName: "TestResource",
					watchable:    true,
					nodeId:       "",
					methodId:     "",
					objectId:     "",
					inputMap:     nil,
					valueType:    common.ValueTypeUint8,
				},
			}},
			dcn: &ua.DataChangeNotification{
				MonitoredItems: []*ua.MonitoredItemNotification{
					{ClientHandle: 123456, Value: &ua.DataValue{Value: ua.MustVariant("42")}},
				},
			},
		},
	}
	certs, err := test.CreateCerts()
	if err != nil {
		t.Errorf("Failed to create certificates: %v", err)
	}
	defer test.Clean(certs)

	server := test.NewServer("../test/opcua_server.py", certs.ServerPKPath, certs.ServerDERCertPath)
	defer server.Close()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := initDriverWithWatchableResources(certs.ClientPKPath, certs.ClientPEMCertPath)
			go func() {
				select {
				case value := <-d.sdkService.AsyncValuesChannel():
					fmt.Printf("value: %v\n", value)
				default:
					fmt.Printf("no value\n")
				}
			}()
			d.resourceMap = tt.resourceMap
			d.handleDataChange(tt.dcn)
			time.Sleep(100 * time.Millisecond)
			d.cleanupListeners()
		})
	}
}
