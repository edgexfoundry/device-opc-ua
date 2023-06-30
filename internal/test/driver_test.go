// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2021 Schneider Electric
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"crypto/tls"
	"github.com/edgexfoundry/device-opcua-go/internal/config"
	"github.com/edgexfoundry/device-opcua-go/internal/driver"
	sdkModel "github.com/edgexfoundry/device-sdk-go/v2/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"github.com/pkg/errors"
	"testing"
)

// helper function to be used in several test classes
func closeServer(server *Server) {
	err := server.Close()
	if err != nil {
		// do nothing
	}
}
func TestDriverUpdateWritableConfig(t *testing.T) {
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
			args: args{rawWritableConfig: &config.WritableInfo{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &driver.Driver{
				Logger:        &logger.MockLogger{},
				ServiceConfig: &config.ServiceConfig{},
			}
			d.UpdateWritableConfig(tt.args.rawWritableConfig)
		})
	}
}

func TestDriverAddDevice(t *testing.T) {
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
			d := driver.NewProtocolDriver().(*driver.Driver)
			d.Logger = &logger.MockLogger{}
			d.ServiceConfig = &config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{DeviceName: tt.args.deviceName}}
			if err := d.AddDevice(tt.args.deviceName, tt.args.protocols, tt.args.adminState); (err != nil) != tt.wantErr {
				t.Errorf("Driver.AddDevice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDriverUpdateDevice(t *testing.T) {
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
			d := &driver.Driver{
				Logger: &logger.MockLogger{},
			}
			if err := d.UpdateDevice(tt.args.deviceName, tt.args.protocols, tt.args.adminState); (err != nil) != tt.wantErr {
				t.Errorf("Driver.UpdateDevice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
func TestDriverCreateClientOptions(t *testing.T) {
	tests := []struct {
		name                 string
		getEndpointsMock     func(ctx context.Context, endpoint string, opt ...opcua.Option) ([]*ua.EndpointDescription, error)
		certAndKeyReaderMock func(clientCertFileName, clientKeyFileName string) ([]byte, []byte, error)
		certKeyPairMock      func(certPEMBlock []byte, keyPEMBlock []byte) (tls.Certificate, error)
		serviceConfig        config.ServiceConfig
		expectedResultLength int
		wantErr              bool
	}{
		{
			name: "OK - options created successfully",
			getEndpointsMock: func(ctx context.Context, endpoint string, opt ...opcua.Option) ([]*ua.EndpointDescription, error) {
				var endpoints []*ua.EndpointDescription
				ep := &ua.EndpointDescription{}
				endpoints = append(endpoints, ep)
				return endpoints, nil
			},
			certAndKeyReaderMock: func(clientCertFileName, clientKeyFileName string) ([]byte, []byte, error) {
				var cert []byte
				var key []byte
				return cert, key, nil
			},
			certKeyPairMock: func(certPEMBlock []byte, keyPEMBlock []byte) (tls.Certificate, error) {
				a := [][]byte{
					{0, 1, 2, 3},
					{4, 5, 6, 7},
				}
				var cert = tls.Certificate{Certificate: a, PrivateKey: nil}
				return cert, nil
			},
			serviceConfig:        config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{Endpoint: "127.0.0.1", Policy: "Ba256Sha256", Mode: "SignAndEncrypt"}},
			expectedResultLength: 10,
			wantErr:              false,
		},
		{
			name: "NOK - options not created when endpoints cannot be fetched",
			getEndpointsMock: func(ctx context.Context, endpoint string, opt ...opcua.Option) ([]*ua.EndpointDescription, error) {
				return nil, errors.New("random endpoint error")
			},
			expectedResultLength: 0,
			wantErr:              true,
		},
		{
			name: "OK - options created correctly with no security policy",
			getEndpointsMock: func(ctx context.Context, endpoint string, opt ...opcua.Option) ([]*ua.EndpointDescription, error) {
				var endpoints []*ua.EndpointDescription
				ep := &ua.EndpointDescription{}
				endpoints = append(endpoints, ep)
				return endpoints, nil
			},
			serviceConfig:        config.ServiceConfig{OPCUAServer: config.OPCUAServerConfig{Endpoint: "127.0.0.1", Policy: "None", Mode: "None"}},
			expectedResultLength: 0,
			wantErr:              false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &driver.Driver{
				Logger: &logger.MockLogger{},
			}
			d.ServiceConfig = &tt.serviceConfig
			driver.GetEndpoints = tt.getEndpointsMock
			driver.ReadCertAndKey = tt.certAndKeyReaderMock
			driver.CertKeyPair = tt.certKeyPairMock

			// can be mocked here since it is the same for every test
			driver.SelectEndPoint = func(endpoints []*ua.EndpointDescription, policy string, mode ua.MessageSecurityMode) *ua.EndpointDescription {
				description := &ua.EndpointDescription{}
				return description
			}
			opts, err := d.CreateClientOptions()
			if (err != nil) != tt.wantErr {
				t.Errorf("Driver.CreateClientOptions()  = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(opts) != tt.expectedResultLength {
				t.Errorf("Driver.CreateClientOptions() = %v, want array len %v", len(opts), tt.expectedResultLength)
			}
		})
	}
}
func TestDriverStop(t *testing.T) {
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
			d := &driver.Driver{
				Logger:    &logger.MockLogger{},
				CtxCancel: cancel,
			}
			if err := d.Stop(tt.args.force); (err != nil) != tt.wantErr {
				t.Errorf("Driver.Stop() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetNodeID(t *testing.T) {
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
			args:    args{attrs: map[string]interface{}{driver.NODE: "ns=2"}, id: "fail"},
			want:    "",
			wantErr: true,
		},
		{
			name:    "OK - node id returned",
			args:    args{attrs: map[string]interface{}{driver.NODE: "ns=2;s=edgex/int32/var0"}, id: driver.NODE},
			want:    "ns=2;s=edgex/int32/var0",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := driver.GetNodeID(tt.args.attrs, tt.args.id)
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

func TestDriverInitialize(t *testing.T) {
	t.Run("initialize", func(t *testing.T) {
		d := driver.NewProtocolDriver()
		err := d.Initialize(&logger.MockLogger{}, make(chan<- *sdkModel.AsyncValues), make(chan<- []sdkModel.DiscoveredDevice))
		if err == nil {
			t.Errorf("expected error to be returned in test environment")
		}
	})
}
