// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2021 Schneider Electric
// Copyright (C) 2024 YIQISOFT
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"testing"

	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
)

func TestDriver_updateWritableConfig(t *testing.T) {
	type args struct {
		rawWritableConfig interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "NOK - bad configuration",
			args: args{rawWritableConfig: nil},
		},
		{
			name: "OK - good configuration",
			args: args{rawWritableConfig: &WritableInfo{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Driver{
				Logger:        &logger.MockLogger{},
				serviceConfig: &ServiceConfig{},
			}
			d.updateWritableConfig(tt.args.rawWritableConfig)
		})
	}
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
			name:    "OK - device add success",
			args:    args{deviceName: "Test"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewProtocolDriver().(*Driver)
			d.Logger = &logger.MockLogger{}
			d.serviceConfig = &ServiceConfig{OPCUAServer: OPCUAServerConfig{DeviceName: tt.args.deviceName}}
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
			name:    "OK - device update success",
			args:    args{deviceName: "Test"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Driver{
				Logger: &logger.MockLogger{},
			}
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
			args:    args{deviceName: "Test"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Driver{
				Logger: &logger.MockLogger{},
			}
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
			ctx := context.Background()
			_, cancel := context.WithCancel(ctx)
			d := &Driver{
				Logger:    &logger.MockLogger{},
				ctxCancel: cancel,
			}
			if err := d.Stop(tt.args.force); (err != nil) != tt.wantErr {
				t.Errorf("Driver.Stop() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getNodeID(t *testing.T) {
	type args struct {
		attrs map[string]interface{}
		id    string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "NOK - key does not exist",
			args:    args{attrs: map[string]interface{}{NODE: "ns=2"}, id: "fail"},
			want:    "",
			wantErr: true,
		},
		{
			name:    "OK - node id returned",
			args:    args{attrs: map[string]interface{}{NODE: "ns=2;s=edgex/int32/var0"}, id: NODE},
			want:    "ns=2;s=edgex/int32/var0",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getNodeID(tt.args.attrs, tt.args.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildNodeID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("buildNodeID() = %v, want %v", got, tt.want)
			}
		})
	}
}
