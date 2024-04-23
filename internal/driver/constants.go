// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2021 Schneider Electric
// Copyright (C) 2024 YIQISOFT
// Copyright (C) 2024 liushenglong_8597@outlook.com
//
// SPDX-License-Identifier: Apache-2.0

package driver

import "fmt"

const (
	// CustomConfigSectionName is the name of the configuration options
	// section in /cmd/res/configuration.toml
	CustomConfigSectionName = "OPCUAServer"
	// WritableInfoSectionName is the Writable section key
	WritableInfoSectionName = CustomConfigSectionName + "/Writable"
)

const (
	// NODE id attribute
	NODE = "nodeId"
	// OBJECT node id attribute
	OBJECT = "objectId"
	// METHOD node id attribute
	METHOD = "methodId"
	// INPUTMAP attribute
	INPUTMAP   = "inputMap"
	WATCHABLE  = "watchable"
	VALUE_TYPE = "valueType"
)

const (
	// Protocol is the supported device protocol
	Protocol = "opcua"
	// EndpointField is a constant string
	EndpointField          = "Endpoint"
	SecurityPolicyField    = "SecurityPolicy"
	SecurityModeField      = "SecurityMode"
	RemotePemCertField     = "RemotePemCert"
	AuthTypeField          = "AuthType"
	UsernameField          = "Username"
	PasswordField          = "Password"
	AutoReconnectField     = "AutoReconnect"
	ReconnectIntervalField = "ReconnectInterval"
	MaxPoolSizeField       = "MaxPoolSize"
)

type SecurityPolicy string

const (
	SecurityPolicyNone                SecurityPolicy = "None"
	SecurityPolicyBasic128Rsa15       SecurityPolicy = "Basic128Rsa15"
	SecurityPolicyBasic256            SecurityPolicy = "Basic256"
	SecurityPolicyBasic256Sha256      SecurityPolicy = "Basic256Sha256"
	SecurityPolicyAes128Sha256RsaOaep SecurityPolicy = "Aes128Sha256RsaOaep"
	SecurityPolicyAes256Sha256RsaPss  SecurityPolicy = "Aes256Sha256RsaPss"
)

func (sp *SecurityPolicy) String() string {
	return fmt.Sprintf("%v", *sp)
}

type SecurityMode string

const (
	SecurityModeNone           SecurityMode = "None"
	SecurityModeSign           SecurityMode = "Sign"
	SecurityModeSignAndEncrypt SecurityMode = "SignAndEncrypt"
)

func (sm *SecurityMode) String() string {
	return fmt.Sprintf("%v", *sm)
}

type AuthType string

const (
	AuthTypeAnonymous AuthType = "Anonymous"
	AuthTypeUsername  AuthType = "Username"
	// AuthTypeCertificate and AuthTypeIssuedToken are not supported yet
	// AuthTypeCertificate AuthType = "Certificate"
	// AuthTypeIssuedToken AuthType = "IssuedToken"
)

func (ap *AuthType) String() string {
	return fmt.Sprintf("%v", *ap)
}
