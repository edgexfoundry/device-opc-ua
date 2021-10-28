// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2021 Schneider Electric
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"reflect"
	"testing"

	"github.com/edgexfoundry/device-opcua-go/internal/config"
	"github.com/edgexfoundry/device-opcua-go/internal/test"
	sdkModel "github.com/edgexfoundry/device-sdk-go/v2/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/models"
)

func TestDriver_HandleReadCommands(t *testing.T) {
	type args struct {
		deviceName string
		protocols  map[string]models.ProtocolProperties
		reqs       []sdkModel.CommandRequest
	}
	tests := []struct {
		name    string
		args    args
		want    []*sdkModel.CommandValue
		wantErr bool
	}{
		{
			name: "NOK - no endpoint defined",
			args: args{
				deviceName: "Test",
				protocols:  map[string]models.ProtocolProperties{config.Protocol: {}},
				reqs:       []sdkModel.CommandRequest{{DeviceResourceName: "TestVar1"}},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "NOK - invalid endpoint defined",
			args: args{
				deviceName: "Test",
				protocols: map[string]models.ProtocolProperties{
					config.Protocol: {config.Endpoint: test.Protocol + "unknown"},
				},
				reqs: []sdkModel.CommandRequest{{DeviceResourceName: "TestVar1"}},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "NOK - non-existent variable",
			args: args{
				deviceName: "Test",
				protocols: map[string]models.ProtocolProperties{
					config.Protocol: {config.Endpoint: test.Protocol + test.Address},
				},
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "TestVar1",
					Attributes:         map[string]interface{}{NAMESPACE: "2", SYMBOL: "fake"},
					Type:               common.ValueTypeInt32,
				}},
			},
			want:    make([]*sdkModel.CommandValue, 1),
			wantErr: true,
		},
		{
			name: "NOK - read command - invalid node id",
			args: args{
				deviceName: "Test",
				protocols: map[string]models.ProtocolProperties{
					config.Protocol: {config.Endpoint: test.Protocol + test.Address},
				},
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "TestResource1",
					Attributes:         map[string]interface{}{NAMESPACE: "2"},
					Type:               common.ValueTypeInt32,
				}},
			},
			want:    make([]*sdkModel.CommandValue, 1),
			wantErr: true,
		},
		{
			name: "NOK - method call - invalid node id",
			args: args{
				deviceName: "Test",
				protocols: map[string]models.ProtocolProperties{
					config.Protocol: {config.Endpoint: test.Protocol + test.Address},
				},
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "TestResource1",
					Attributes:         map[string]interface{}{NAMESPACE: "2", METHOD: "test"},
					Type:               common.ValueTypeInt32,
				}},
			},
			want:    make([]*sdkModel.CommandValue, 1),
			wantErr: true,
		},
		{
			name: "NOK - method call - method does not exist",
			args: args{
				deviceName: "Test",
				protocols: map[string]models.ProtocolProperties{
					config.Protocol: {config.Endpoint: test.Protocol + test.Address},
				},
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "TestResource1",
					Attributes:         map[string]interface{}{NAMESPACE: "2", METHOD: "test", OBJECT: "main"},
					Type:               common.ValueTypeInt32,
				}},
			},
			want:    make([]*sdkModel.CommandValue, 1),
			wantErr: true,
		},
		{
			name: "OK - read value from mock server",
			args: args{
				deviceName: "Test",
				protocols: map[string]models.ProtocolProperties{
					config.Protocol: {config.Endpoint: test.Protocol + test.Address},
				},
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "TestVar1",
					Attributes:         map[string]interface{}{NAMESPACE: "2", SYMBOL: "ro_int32"},
					Type:               common.ValueTypeInt32,
				}},
			},
			want: []*sdkModel.CommandValue{{
				DeviceResourceName: "TestVar1",
				Type:               common.ValueTypeInt32,
				Value:              int32(5),
				Tags:               make(map[string]string),
			}},
			wantErr: false,
		},
		{
			name: "OK - call method from mock server",
			args: args{
				deviceName: "Test",
				protocols: map[string]models.ProtocolProperties{
					config.Protocol: {config.Endpoint: test.Protocol + test.Address},
				},
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "SquareResource",
					Attributes:         map[string]interface{}{NAMESPACE: "2", METHOD: "square", OBJECT: "main", INPUTMAP: []interface{}{"2"}},
					Type:               common.ValueTypeInt64,
				}},
			},
			want: []*sdkModel.CommandValue{{
				DeviceResourceName: "SquareResource",
				Type:               common.ValueTypeInt64,
				Value:              int64(4),
				Tags:               make(map[string]string),
			}},
			wantErr: false,
		},
	}

	server := test.NewServer("../test/opcua_server.py")
	defer server.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Driver{
				Logger: &logger.MockLogger{},
			}
			got, err := d.HandleReadCommands(tt.args.deviceName, tt.args.protocols, tt.args.reqs)
			if (err != nil) != tt.wantErr {
				t.Errorf("Driver.HandleReadCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Ignore Origin for DeepEqual
			if len(got) > 0 && got[0] != nil {
				got[0].Origin = 0
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Driver.HandleReadCommands() = %v, want %v", got, tt.want)
			}
		})
	}
}
