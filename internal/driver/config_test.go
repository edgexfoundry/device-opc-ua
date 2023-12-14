// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2021 Schneider Electric
// Copyright (C) 2023 YIQISOFT
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"testing"

	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
)

func TestOPCUAServerConfig_Validate(t *testing.T) {
	type fields struct {
		DeviceName string
		Policy     string
		Mode       string
		CertFile   string
		KeyFile    string
		Writable   WritableInfo
	}
	tests := []struct {
		name      string
		fields    fields
		wantError bool
	}{
		{
			name:      "NOK - no device name specified",
			fields:    fields{},
			wantError: true,
		},
		{
			name:      "NOK - policy mismatch",
			fields:    fields{DeviceName: "Test"},
			wantError: true,
		},
		{
			name:      "NOK - mode mismatch",
			fields:    fields{DeviceName: "Test", Policy: "None"},
			wantError: true,
		},
		{
			name:      "NOK - missing certfile",
			fields:    fields{DeviceName: "Test", Policy: "Basic256", Mode: "Sign"},
			wantError: true,
		},
		{
			name:      "NOK - missing keyfile",
			fields:    fields{DeviceName: "Test", Policy: "Basic256", Mode: "Sign", CertFile: "path/to/cert"},
			wantError: true,
		},
		{
			name:      "OK - valid configuration with policy and mode",
			fields:    fields{DeviceName: "Test", Policy: "Basic256", Mode: "Sign", CertFile: "path/to/cert", KeyFile: "path/to/key"},
			wantError: false,
		},
		{
			name:      "OK - valid configuration without policy and mode",
			fields:    fields{DeviceName: "Test", Policy: "None", Mode: "None"},
			wantError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &OPCUAServerConfig{
				DeviceName: tt.fields.DeviceName,
				Policy:     tt.fields.Policy,
				Mode:       tt.fields.Mode,
				CertFile:   tt.fields.CertFile,
				KeyFile:    tt.fields.KeyFile,
				Writable:   tt.fields.Writable,
			}
			if got := info.Validate(); got != nil && !tt.wantError || got == nil && tt.wantError {
				t.Errorf("OPCUAServerConfig.Validate() = %v, wantError %v", got, tt.wantError)
			}
		})
	}
}

func Test_FetchEndpoint(t *testing.T) {
	const testEndpoint string = "opc://test-endpoint"

	type args struct {
		protocols map[string]models.ProtocolProperties
	}
	tests := []struct {
		name      string
		args      args
		want      string
		wantError bool
	}{
		{
			name:      "NOK - missing protocol",
			args:      args{protocols: map[string]models.ProtocolProperties{}},
			want:      "",
			wantError: true,
		},
		{
			name:      "NOK - missing endpoint",
			args:      args{protocols: map[string]models.ProtocolProperties{Protocol: {}}},
			want:      "",
			wantError: true,
		},
		{
			name:      "OK - valid properties",
			args:      args{protocols: map[string]models.ProtocolProperties{Protocol: {Endpoint: testEndpoint}}},
			want:      testEndpoint,
			wantError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FetchEndpoint(tt.args.protocols)
			if got != tt.want {
				t.Errorf("fetchEndpoint() got = %v, want %v", got, tt.want)
			}
			if tt.wantError && err == nil {
				t.Error("fetchEndpoint() should have returned an error")
			}
			if !tt.wantError && err != nil {
				t.Errorf("fetchEndpoint() should have returned an error. Got = %v", got)
			}
		})
	}
}

func TestServiceConfig_UpdateFromRaw(t *testing.T) {
	type fields struct {
		OPCUAServer OPCUAServerConfig
	}
	type args struct {
		rawConfig interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "NOK - bad configuration",
			fields: fields{OPCUAServer: OPCUAServerConfig{}},
			args:   args{rawConfig: nil},
			want:   false,
		},
		{
			name:   "OK - good configuration",
			fields: fields{OPCUAServer: OPCUAServerConfig{}},
			args:   args{rawConfig: &ServiceConfig{OPCUAServer: OPCUAServerConfig{DeviceName: "Test"}}},
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sw := &ServiceConfig{
				OPCUAServer: tt.fields.OPCUAServer,
			}
			if got := sw.UpdateFromRaw(tt.args.rawConfig); got != tt.want {
				t.Errorf("ServiceConfig.UpdateFromRaw() = %v, want %v", got, tt.want)
			}
		})
	}
}
