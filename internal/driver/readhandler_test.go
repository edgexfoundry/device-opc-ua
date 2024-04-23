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
	"os"
	"reflect"
	"testing"

	"github.com/edgexfoundry/device-opc-ua/internal/test"
	sdkModel "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
)

func TestDriver_HandleReadCommands(t *testing.T) {
	certs, err := test.CreateCerts()
	if err != nil {
		t.Errorf("Failed to create certificates: %v", err)
	}
	defer test.Clean(certs)
	bytes, err := os.ReadFile(certs.ServerPEMCertPath)
	if err != nil {
		t.Errorf("Failed to read server certificate: %v", err)
	}
	servercert := string(bytes)

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
				protocols:  map[string]models.ProtocolProperties{Protocol: {}},
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
					Protocol: {EndpointField: test.Protocol + "unknown"},
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
					Protocol: {
						EndpointField:       test.Protocol + test.Address,
						SecurityPolicyField: SecurityPolicyNone,
					},
				},
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "TestVar1",
					Attributes:         map[string]interface{}{NODE: "ns=2;s=fake"},
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
					Protocol: {
						EndpointField:       test.Protocol + test.Address,
						SecurityPolicyField: SecurityPolicyNone,
					},
				},
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "TestResource1",
					Attributes:         map[string]interface{}{NODE: "2"},
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
					Protocol: {
						EndpointField:       test.Protocol + test.Address,
						SecurityPolicyField: SecurityPolicyNone,
					},
				},
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "TestResource1",
					Attributes: map[string]interface{}{
						METHOD: "ns=2;s=test",
						OBJECT: "2",
					},
					Type: common.ValueTypeInt32,
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
					Protocol: {
						EndpointField:       test.Protocol + test.Address,
						SecurityPolicyField: SecurityPolicyNone,
					},
				},
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "TestResource1",
					Attributes:         map[string]interface{}{METHOD: "ns=2;s=test", OBJECT: "ns=2;s=main"},
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
					Protocol: {
						EndpointField:       test.Protocol + test.Address,
						SecurityPolicyField: SecurityPolicyNone,
					},
				},
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "TestVar1",
					Attributes:         map[string]interface{}{NODE: "ns=2;s=ro_int32"},
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
					Protocol: {
						EndpointField:       test.Protocol + test.Address,
						SecurityPolicyField: SecurityPolicyBasic256Sha256,
						SecurityModeField:   SecurityModeSignAndEncrypt,
						RemotePemCertField:  servercert,
					},
				},
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "SquareResource",
					Attributes:         map[string]interface{}{METHOD: "ns=2;s=square", OBJECT: "ns=2;s=main", INPUTMAP: []interface{}{"2"}},
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

	server := test.NewServer("../test/opcua_server.py", certs.ServerPKPath, certs.ServerDERCertPath)
	defer server.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := initDriver(certs.ClientPKPath, certs.ClientPEMCertPath)
			got, err := d.HandleReadCommands(tt.args.deviceName, tt.args.protocols, tt.args.reqs)
			if (err != nil) != tt.wantErr {
				t.Errorf("Driver.HandleReadCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Ignore Origin for DeepEqual
			if len(got) > 0 && got[0] != nil {
				got[0].Origin = 0
			}
			fmt.Printf("readings: %d\n", len(got))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Driver.HandleReadCommands() = %v, want %v", got, tt.want)
			}
		})
	}
}
