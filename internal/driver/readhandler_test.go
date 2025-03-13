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

	"github.com/edgexfoundry/device-opc-ua/internal/test"
	sdkModel "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

//comment out following unittest as it requires to run a Python-based simulated OPC UA server, which is not available
//during build process
//func TestDriver_HandleReadCommands(t *testing.T) {
//	type args struct {
//		deviceName string
//		protocols  map[string]models.ProtocolProperties
//		reqs       []sdkModel.CommandRequest
//	}
//	tests := []struct {
//		name    string
//		args    args
//		want    []*sdkModel.CommandValue
//		wantErr bool
//	}{
//		{
//			name: "NOK - no endpoint defined",
//			args: args{
//				deviceName: "Test",
//				protocols:  map[string]models.ProtocolProperties{Protocol: {}},
//				reqs:       []sdkModel.CommandRequest{{DeviceResourceName: "TestVar1"}},
//			},
//			want:    nil,
//			wantErr: true,
//		},
//		{
//			name: "NOK - invalid endpoint defined",
//			args: args{
//				deviceName: "Test",
//				protocols: map[string]models.ProtocolProperties{
//					Protocol: {Endpoint: test.Protocol + "unknown"},
//				},
//				reqs: []sdkModel.CommandRequest{{DeviceResourceName: "TestVar1"}},
//			},
//			want:    nil,
//			wantErr: true,
//		},
//		{
//			name: "NOK - non-existent variable",
//			args: args{
//				deviceName: "Test",
//				protocols: map[string]models.ProtocolProperties{
//					Protocol: {Endpoint: test.Protocol + test.Address},
//				},
//				reqs: []sdkModel.CommandRequest{{
//					DeviceResourceName: "TestVar1",
//					Attributes:         map[string]interface{}{NODE: "ns=2;s=fake"},
//					Type:               common.ValueTypeInt32,
//				}},
//			},
//			want:    make([]*sdkModel.CommandValue, 1),
//			wantErr: true,
//		},
//		{
//			name: "NOK - read command - invalid node id",
//			args: args{
//				deviceName: "Test",
//				protocols: map[string]models.ProtocolProperties{
//					Protocol: {Endpoint: test.Protocol + test.Address},
//				},
//				reqs: []sdkModel.CommandRequest{{
//					DeviceResourceName: "TestResource1",
//					Attributes:         map[string]interface{}{NODE: "2"},
//					Type:               common.ValueTypeInt32,
//				}},
//			},
//			want:    make([]*sdkModel.CommandValue, 1),
//			wantErr: true,
//		},
//		{
//			name: "NOK - method call - invalid node id",
//			args: args{
//				deviceName: "Test",
//				protocols: map[string]models.ProtocolProperties{
//					Protocol: {Endpoint: test.Protocol + test.Address},
//				},
//				reqs: []sdkModel.CommandRequest{{
//					DeviceResourceName: "TestResource1",
//					Attributes:         map[string]interface{}{METHOD: "ns=2;s=test"},
//					Type:               common.ValueTypeInt32,
//				}},
//			},
//			want:    make([]*sdkModel.CommandValue, 1),
//			wantErr: true,
//		},
//		{
//			name: "NOK - method call - method does not exist",
//			args: args{
//				deviceName: "Test",
//				protocols: map[string]models.ProtocolProperties{
//					Protocol: {Endpoint: test.Protocol + test.Address},
//				},
//				reqs: []sdkModel.CommandRequest{{
//					DeviceResourceName: "TestResource1",
//					Attributes:         map[string]interface{}{METHOD: "ns=2;s=test", OBJECT: "ns=2;s=main"},
//					Type:               common.ValueTypeInt32,
//				}},
//			},
//			want:    make([]*sdkModel.CommandValue, 1),
//			wantErr: true,
//		},
//		{
//			name: "OK - read value from mock server",
//			args: args{
//				deviceName: "Test",
//				protocols: map[string]models.ProtocolProperties{
//					Protocol: {Endpoint: test.Protocol + test.Address},
//				},
//				reqs: []sdkModel.CommandRequest{{
//					DeviceResourceName: "TestVar1",
//					Attributes:         map[string]interface{}{NODE: "ns=2;s=ro_int32"},
//					Type:               common.ValueTypeInt32,
//				}},
//			},
//			want: []*sdkModel.CommandValue{{
//				DeviceResourceName: "TestVar1",
//				Type:               common.ValueTypeInt32,
//				Value:              int32(5),
//				Tags:               make(map[string]string),
//			}},
//			wantErr: false,
//		},
//		{
//			name: "OK - call method from mock server",
//			args: args{
//				deviceName: "Test",
//				protocols: map[string]models.ProtocolProperties{
//					Protocol: {Endpoint: test.Protocol + test.Address},
//				},
//				reqs: []sdkModel.CommandRequest{{
//					DeviceResourceName: "SquareResource",
//					Attributes:         map[string]interface{}{METHOD: "ns=2;s=square", OBJECT: "ns=2;s=main", INPUTMAP: []interface{}{"2"}},
//					Type:               common.ValueTypeInt64,
//				}},
//			},
//			want: []*sdkModel.CommandValue{{
//				DeviceResourceName: "SquareResource",
//				Type:               common.ValueTypeInt64,
//				Value:              int64(4),
//				Tags:               make(map[string]string),
//			}},
//			wantErr: false,
//		},
//	}
//
//	server := test.NewServer("../test/opcua_server.py")
//	defer server.Close()
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			d := &Driver{
//				Logger:    &logger.MockLogger{},
//				clientMap: map[string]*opcua.Client{},
//			}
//			got, err := d.HandleReadCommands(tt.args.deviceName, tt.args.protocols, tt.args.reqs)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("Driver.HandleReadCommands() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			// Ignore Origin for DeepEqual
//			if len(got) > 0 && got[0] != nil {
//				got[0].Origin = 0
//			}
//			if !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("Driver.HandleReadCommands() = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}

func Benchmark_HandleReadCommands_ReuseClient(b *testing.B) {
	server := test.NewServer("../test/opcua_server.py")
	defer server.Close()

	d := &Driver{
		Logger:    &logger.MockLogger{},
		clientMap: map[string]*opcua.Client{},
	}
	deviceName := "Test"
	protocols := map[string]models.ProtocolProperties{
		Protocol: {Endpoint: test.Protocol + test.Address},
	}
	reqs := []sdkModel.CommandRequest{{
		DeviceResourceName: "TestVar1",
		Attributes:         map[string]interface{}{NODE: "ns=2;s=ro_int32"},
		Type:               common.ValueTypeInt32,
	}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := d.HandleReadCommands(deviceName, protocols, reqs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_HandleReadCommands_WithoutReuseClient(b *testing.B) {
	server := test.NewServer("../test/opcua_server.py")
	defer server.Close()

	d := &Driver{
		Logger:    &logger.MockLogger{},
		clientMap: map[string]*opcua.Client{},
	}
	deviceName := "Test"
	protocols := map[string]models.ProtocolProperties{
		Protocol: {Endpoint: test.Protocol + test.Address},
	}
	reqs := []sdkModel.CommandRequest{{
		DeviceResourceName: "TestVar1",
		Attributes:         map[string]interface{}{NODE: "ns=2;s=ro_int32"},
		Type:               common.ValueTypeInt32,
	}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = handleReadCommandsWithoutReuseClient(d, deviceName, protocols, reqs)
	}
}

func handleReadCommandsWithoutReuseClient(
	d *Driver,
	deviceName string,
	protocols map[string]models.ProtocolProperties,
	reqs []sdkModel.CommandRequest) ([]*sdkModel.CommandValue, error) {

	d.Logger.Debugf("Driver.HandleReadCommands: protocols: %v resource: %v attributes: %v", protocols, reqs[0].DeviceResourceName, reqs[0].Attributes)

	// create device client and open connection
	endpoint, err := FetchEndpoint(protocols)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	client, _ := opcua.NewClient(endpoint, opcua.SecurityMode(ua.MessageSecurityModeNone))
	if err := client.Connect(ctx); err != nil {
		d.Logger.Warnf("Driver.HandleReadCommands: Failed to connect OPCUA client, %s", err)
		return nil, err
	}
	defer client.Close(ctx)

	return d.processReadCommands(client, reqs)
}
