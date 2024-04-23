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
	"github.com/edgexfoundry/device-sdk-go/v3/pkg/interfaces"
	"testing"

	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
)

func initSimpleDriver() *Driver {
	testDriver := NewProtocolDriver().(*Driver)
	sdkService := mock.NewDeviceSdk()
	sdkService.(*mock.DeviceServiceSdk).LoadCustomConfigImpl = func(customConfig interfaces.UpdatableConfig, sectionName string) error {
		return nil
	}
	err := driver.Initialize(sdkService)
	if err != nil {
		panic(err)
	}
	return testDriver
}

func initDriver(clientPkPath string, clientCertPath string) *Driver {
	testDriver := NewProtocolDriver().(*Driver)
	sdkService := mock.NewDeviceSdk()
	sdkService.(*mock.DeviceServiceSdk).LoadCustomConfigImpl = func(customConfig interfaces.UpdatableConfig, sectionName string) error {
		customConfig.(*ServiceConfig).OPCUAServer = ClientInfo{
			CertFile: clientCertPath,
			KeyFile:  clientPkPath,
		}
		return nil
	}
	err := driver.Initialize(sdkService)

	if err != nil {
		panic(err)
	}
	fmt.Println("test driver initiated")
	return testDriver
}

func TestDriver_AddDevice(t *testing.T) {
	type args struct {
		deviceName string
		protocols  map[string]models.ProtocolProperties
		adminState models.AdminState
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "OK - device add success",
			args: args{
				deviceName: "Test",
				protocols: map[string]models.ProtocolProperties{
					Protocol: {
						EndpointField:       "opc.tcp://120.0.0.1:48408",
						SecurityPolicyField: SecurityPolicyNone,
						SecurityModeField:   SecurityModeSign,
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := initSimpleDriver()
			if err := d.AddDevice(tt.args.deviceName, tt.args.protocols, tt.args.adminState); (err != nil) != tt.wantErr {
				t.Errorf("Driver.AddDevice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDriver_UpdateDevice(t *testing.T) {
	type args struct {
		deviceName string
		protocols  map[string]models.ProtocolProperties
		adminState models.AdminState
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "OK - device update success",
			args: args{deviceName: "DeviceForTest", protocols: map[string]models.ProtocolProperties{
				Protocol: {
					EndpointField:       "opc.tcp://127.0.0.1:48409",
					SecurityPolicyField: SecurityPolicyNone,
					SecurityModeField:   SecurityModeNone,
				},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := initSimpleDriver()
			if err := d.UpdateDevice(tt.args.deviceName, tt.args.protocols, tt.args.adminState); (err != nil) != tt.wantErr {
				t.Errorf("Driver.UpdateDevice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDriver_RemoveDevice(t *testing.T) {
	type args struct {
		deviceName string
		protocols  map[string]models.ProtocolProperties
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "OK - device removal success",
			args:    args{deviceName: "DeviceForTest"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := initSimpleDriver()
			if err := d.RemoveDevice(tt.args.deviceName, tt.args.protocols); (err != nil) != tt.wantErr {
				t.Errorf("Driver.RemoveDevice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDriver_Stop(t *testing.T) {
	type args struct {
		force bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "OK - device stopped",
			args:    args{force: false},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := initSimpleDriver()
			if err := d.Stop(tt.args.force); (err != nil) != tt.wantErr {
				t.Errorf("Driver.Stop() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
