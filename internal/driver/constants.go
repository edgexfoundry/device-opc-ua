// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2021 Schneider Electric
//
// SPDX-License-Identifier: Apache-2.0

package driver

const (
	// InsecureSecretsConfigSectionName is the name of the configuration options
	// section for username and password in /cmd/res/configuration.toml
	InsecureSecretsConfigSectionName = "/Writable/InsecureSecrets/OPCUA/Secrets"
	// CustomConfigSectionName is the name of the opcua configuration options
	// section in /cmd/res/configuration.toml
	CustomConfigSectionName = "/OPCUAServer/Writable"
)

const (
	// NODE id attribute
	NODE = "nodeId"
	// OBJECT node id attribute
	OBJECT = "objectId"
	// METHOD node id attribute
	METHOD = "methodId"
	// INPUTMAP attribute
	INPUTMAP = "inputMap"
)
