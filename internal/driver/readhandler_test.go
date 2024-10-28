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
	"sync"
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

func Benchmark_HandleReadCommands_ReuseClientWithEncryptionAsync(b *testing.B) {
	certs, err := test.CreateCerts()
	if err != nil {
		b.Fatal(err)
	}
	defer test.Clean(certs)

	remoteCertBytes, err := os.ReadFile(certs.ServerPEMCertPath)
	if err != nil {
		b.Fatal(err)
	}
	remoteCert := string(remoteCertBytes)

	deviceName := "Test"
	protocols := map[string]models.ProtocolProperties{
		Protocol: {
			EndpointField:       test.Protocol + test.Address,
			SecurityPolicyField: SecurityPolicyBasic256Sha256,
			SecurityModeField:   SecurityModeSignAndEncrypt,
			RemotePemCertField:  remoteCert,
			MaxPoolSizeField:    4,
		},
	}
	reqs := []sdkModel.CommandRequest{{
		DeviceResourceName: "TestVar1",
		Attributes:         map[string]interface{}{NODE: "ns=2;s=ro_int32"},
		Type:               common.ValueTypeInt32,
	}}

	server := test.NewServer("../test/opcua_server.py", certs.ServerPKPath, certs.ServerDERCertPath)
	defer server.Close()
	d := initDriver(certs.ClientPKPath, certs.ClientPEMCertPath)
	b.ResetTimer()
	n := b.N
	wg := sync.WaitGroup{}
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			_, err = d.HandleReadCommands(deviceName, protocols, reqs)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func Benchmark_HandleReadCommands_ReuseClientWithEncryption(b *testing.B) {
	certs, err := test.CreateCerts()
	if err != nil {
		b.Fatal(err)
	}
	defer test.Clean(certs)

	remoteCertBytes, err := os.ReadFile(certs.ServerPEMCertPath)
	if err != nil {
		b.Fatal(err)
	}
	remoteCert := string(remoteCertBytes)

	server := test.NewServer("../test/opcua_server.py", certs.ServerPKPath, certs.ServerDERCertPath)
	defer server.Close()

	d := initDriver(certs.ClientPKPath, certs.ClientPEMCertPath)
	deviceName := "Test"
	protocols := map[string]models.ProtocolProperties{
		Protocol: {
			EndpointField:       test.Protocol + test.Address,
			SecurityPolicyField: SecurityPolicyBasic256Sha256,
			SecurityModeField:   SecurityModeSignAndEncrypt,
			RemotePemCertField:  remoteCert,
		},
	}
	reqs := []sdkModel.CommandRequest{{
		DeviceResourceName: "TestVar1",
		Attributes:         map[string]interface{}{NODE: "ns=2;s=ro_int32"},
		Type:               common.ValueTypeInt32,
	}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = d.HandleReadCommands(deviceName, protocols, reqs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_HandleReadCommands_ReuseClientAsync(b *testing.B) {
	server := test.NewServer("../test/opcua_server.py")
	defer server.Close()

	d := initSimpleDriver()
	deviceName := "Test"
	protocols := map[string]models.ProtocolProperties{
		Protocol: {
			EndpointField:       test.Protocol + test.Address,
			SecurityPolicyField: SecurityPolicyNone,
			MaxPoolSizeField:    4,
		},
	}
	reqs := []sdkModel.CommandRequest{{
		DeviceResourceName: "TestVar1",
		Attributes:         map[string]interface{}{NODE: "ns=2;s=ro_int32"},
		Type:               common.ValueTypeInt32,
	}}

	b.ResetTimer()

	n := b.N
	wg := sync.WaitGroup{}
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			_, err := d.HandleReadCommands(deviceName, protocols, reqs)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func Benchmark_HandleReadCommands_ReuseClient(b *testing.B) {
	server := test.NewServer("../test/opcua_server.py")
	defer server.Close()

	d := initSimpleDriver()
	deviceName := "Test"
	protocols := map[string]models.ProtocolProperties{
		Protocol: {
			EndpointField:       test.Protocol + test.Address,
			SecurityPolicyField: SecurityPolicyNone,
		},
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

	d := initSimpleDriver()
	deviceName := "Test"
	protocols := map[string]models.ProtocolProperties{
		Protocol: {
			EndpointField:       test.Protocol + test.Address,
			SecurityPolicyField: SecurityPolicyNone,
		},
	}
	reqs := []*CommandInfo{{
		resourceName: "TestVar1",
		nodeId:       "ns=2;s=ro_int32",
		valueType:    common.ValueTypeInt32,
	}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := handleReadCommandsWithoutReuseClient(d, deviceName, protocols, reqs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func handleReadCommandsWithoutReuseClient(
	d *Driver,
	deviceName string,
	protocols map[string]models.ProtocolProperties,
	reqs []*CommandInfo) ([]*sdkModel.CommandValue, error) {

	d.Logger.Debugf("Driver.HandleReadCommands: protocols: %v resource: %v attributes: %v", protocols, reqs[0].resourceName, reqs[0])
	info, err := createConnectionInfo(protocols)
	if err != nil {
		return nil, err
	}
	// create device client and open connection
	wrapper, err := d.uaConnectionPool.getConnectionUnsafe(info)
	if err != nil {
		return nil, err
	}
	clientWrapper := wrapper.(ClientWrapper)
	defer clientWrapper.Close()

	return d.processReadCommands(clientWrapper.GetClient(), reqs)
}
