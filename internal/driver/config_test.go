// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2021 Schneider Electric
// Copyright (C) 2024 YIQISOFT
// Copyright (C) 2024 liushenglong_8597@outlook.com
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"reflect"
	"testing"
	"time"

	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
)

func Test_CreateConnectionInfo(t *testing.T) {
	var wanted = &ConnectionInfo{
		EndpointURL:       "tcp://127.0.0.1:48408",
		SecurityPolicy:    SecurityPolicyNone,
		SecurityMode:      SecurityModeNone,
		AuthType:          AuthTypeUsername,
		Username:          "user",
		Password:          "password",
		AutoReconnect:     true,
		ReconnectInterval: 1000,
		MaxPoolSize:       2,
	}

	type args struct {
		protocols map[string]models.ProtocolProperties
	}
	tests := []struct {
		name      string
		args      args
		want      any
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
			name: "NOK - Invalid security policy",
			args: args{protocols: map[string]models.ProtocolProperties{Protocol: {
				EndpointField:       wanted.EndpointURL,
				SecurityPolicyField: "invalid",
			}}},
			want:      "",
			wantError: true,
		},
		{
			name: "OK - valid security mode",
			args: args{protocols: map[string]models.ProtocolProperties{Protocol: {
				EndpointField:       wanted.EndpointURL,
				SecurityPolicyField: SecurityPolicyNone,
				SecurityModeField:   SecurityModeSign,
			}}},
			want: &ConnectionInfo{
				EndpointURL:       wanted.EndpointURL,
				SecurityPolicy:    SecurityPolicyNone,
				SecurityMode:      SecurityModeNone,
				AuthType:          AuthTypeAnonymous,
				Username:          "",
				Password:          "",
				AutoReconnect:     true,
				ReconnectInterval: 5 * time.Second,
				MaxPoolSize:       1,
			},
			wantError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createConnectionInfo(tt.args.protocols)
			if tt.wantError && err == nil {
				t.Error("createConnectionInfo() should have returned an error")
			}
			if !tt.wantError && err != nil {
				t.Errorf("createConnectionInfo() should have returned no error. Got = %v", got)
			}
			if !tt.wantError && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createConnectionInfo() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_CreateCommandInfo(t *testing.T) {
	var resourceName = "some resource"
	type args struct {
		resourceName string
		Attributes   map[string]interface{}
	}
	tests := []struct {
		name      string
		args      args
		want      any
		wantError bool
	}{
		{
			name: "NOK - watchable_missing-nodeId",
			args: args{
				resourceName: resourceName,
				Attributes: map[string]interface{}{
					WATCHABLE: true,
				},
			},
			want:      "",
			wantError: true,
		},
		{
			name: "OK - watchable",
			args: args{
				resourceName: resourceName,
				Attributes: map[string]interface{}{
					WATCHABLE: true,
					NODE:      "ns=1;i=1",
				},
			},
			want: &CommandInfo{
				resourceName: resourceName,
				watchable:    true,
				nodeId:       "ns=1;i=1",
				methodId:     "",
				objectId:     "",
				inputMap:     nil,
			},
			wantError: false,
		},
		{
			name: "OK - method&node",
			args: args{
				resourceName: resourceName,
				Attributes: map[string]interface{}{
					NODE:   "ns=1;i=1",
					METHOD: "ns=1;i=2",
				},
			},
			want: &CommandInfo{
				resourceName: resourceName,
				watchable:    false,
				nodeId:       "ns=1;i=1",
				methodId:     "",
				objectId:     "",
				inputMap:     nil,
			},
			wantError: false,
		},
		{
			name: "NOK - method without objectId",
			args: args{
				resourceName: resourceName,
				Attributes: map[string]interface{}{
					METHOD: "ns=1;i=2",
				},
			},
			want:      "",
			wantError: true,
		},
		{
			name: "OK - method&object&inputMap",
			args: args{
				resourceName: resourceName,
				Attributes: map[string]interface{}{
					METHOD:   "ns=1;i=2",
					OBJECT:   "obj",
					INPUTMAP: []any{1, "ss"},
				},
			},
			want: &CommandInfo{
				resourceName: resourceName,
				watchable:    false,
				nodeId:       "",
				methodId:     "ns=1;i=2",
				objectId:     "obj",
				inputMap:     []string{"1", "ss"},
			},
			wantError: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := CreateCommandInfo(test.args.resourceName, "", test.args.Attributes)
			if test.wantError && err == nil {
				t.Error("CreateCommandInfo() should have returned an error")
			}
			if !test.wantError && err != nil {
				t.Errorf("CreateCommandInfo() should have returned no error. got: %v", err)
			}
			if !test.wantError && !reflect.DeepEqual(got, test.want) {
				t.Errorf("CreateCommandInfo() got = %v, want %v", got, test.want)
			}
		})
	}
}
