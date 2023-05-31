// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
// Copyright (C) 2021 Schneider Electric
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/models"
)

// ServiceConfig configuration struct
type ServiceConfig struct {
	OPCUAServer OPCUAServerConfig
}

// UpdateFromRaw updates the service's full configuration from raw data received from
// the Service Provider.
func (sw *ServiceConfig) UpdateFromRaw(rawConfig interface{}) bool {
	configuration, ok := rawConfig.(*ServiceConfig)
	if !ok {
		return false
	}

	*sw = *configuration

	return true
}

// OPCUAServerConfig server information defined by the device profile
type OPCUAServerConfig struct {
	DeviceName            string
	Policy                string
	Mode                  string
	Endpoint              string
	CredentialsPath       string
	CertificateConfig     CertificateConfiguration
	CredentialsRetryTime  int
	CredentialsRetryWait  int
	ConnEstablishingRetry int
	ConnRetryWaitTime     int
	Writable              WritableInfo
}

// CertificateConfiguration config information regarding the certificate
type CertificateConfiguration struct {
	CertFile            string
	CertOrganization    string
	CertCountry         string
	CertProvince        string
	CertLocality        string
	CertBits            int
	CertFilePermissions string
	KeyFile             string
}

// WritableInfo configuration data that can be written without restarting the service
type WritableInfo struct {
	Resources string
}

var policies map[string]int = map[string]int{
	"None":           1,
	"Basic128Rsa15":  2,
	"Basic256":       3,
	"Basic256Sha256": 4,
}

type Credentials struct {
	Username string
	Password string
}

var modes map[string]int = map[string]int{
	"None":           1,
	"Sign":           2,
	"SignAndEncrypt": 3,
}

// Validate ensures your custom configuration has proper values.
func (info *OPCUAServerConfig) Validate() errors.EdgeX {
	if info.DeviceName == "" {
		return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.DeviceName configuration setting cannot be blank", nil)
	}

	if _, ok := policies[info.Policy]; !ok {
		return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.Policy configuration setting mismatch", nil)
	}
	if _, ok := modes[info.Mode]; !ok {
		return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.Mode configuration setting mismatch", nil)
	}
	if info.Mode != "None" || info.Policy != "None" {
		if info.CertificateConfig.CertFile == "" {
			return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.CertFile configuration setting cannot be blank when a security mode or policy is set", nil)
		}
		if info.CertificateConfig.KeyFile == "" {
			return errors.NewCommonEdgeX(errors.KindContractInvalid, "OPCUAServerInfo.KeyFile configuration setting cannot be blank when a security mode or policy is set", nil)
		}
	}

	return nil
}

// FetchEndpoint returns the OPCUA endpoint defined in the configuration
func FetchEndpoint(protocols map[string]models.ProtocolProperties) (string, errors.EdgeX) {
	properties, ok := protocols[Protocol]
	if !ok {
		return "", errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("'%s' protocol properties is not defined", Protocol), nil)
	}
	endpoint, ok := properties[Endpoint]
	if !ok {
		return "", errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("'%s' not found in the '%s' protocol properties", Endpoint, Protocol), nil)
	}
	return endpoint, nil
}
