// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
// Copyright (C) 2021 Schneider Electric
// Copyright (C) 2024 YIQISOFT
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/edgexfoundry/go-mod-core-contracts/v3/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
)

// ServiceConfig configuration struct
type ServiceConfig struct {
	OPCUAServer ClientInfo
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

// ClientInfo server information defined by the device profile
type ClientInfo struct {
	CertFile string
	KeyFile  string
	// ApplicationURI is optional. but, if it is specified, it must match the URI field in "Subject Alternative Name" of the client certificate.
	ApplicationURI string
}

type ConnectionInfo struct {
	EndpointURL    string
	SecurityPolicy SecurityPolicy
	SecurityMode   SecurityMode
	AuthType       AuthType
	Username       string
	Password       string
	AutoReconnect  bool
	// ReconnectInterval is the time interval to reconnect to the server when the connection is lost when AutoReconnect is set to true.
	//default value is 5 seconds
	ReconnectInterval time.Duration
	// MaxPoolSize is the maximum number of connections that can be created in the connection pool. default value is 1
	MaxPoolSize uint32
}

func (info *ConnectionInfo) Equals(other *ConnectionInfo) bool {
	return info.EndpointURL == other.EndpointURL &&
		info.SecurityPolicy == other.SecurityPolicy &&
		info.SecurityMode == other.SecurityMode &&
		info.AuthType == other.AuthType &&
		info.Username == other.Username &&
		info.Password == other.Password &&
		info.AutoReconnect == other.AutoReconnect &&
		info.ReconnectInterval == other.ReconnectInterval &&
		info.MaxPoolSize == other.MaxPoolSize
}

type CommandInfo struct {
	watchable bool
	nodeId    string
	methodId  string
	objectId  string
	inputMap  []string
}

func (c *CommandInfo) isMethodCall() bool {
	return len(c.methodId) > 0
}

func (c *CommandInfo) IsWatchable() bool {
	return c.watchable
}

func (c *CommandInfo) HasArgs() bool {
	return len(c.inputMap) > 0
}

func createConnectionInfo(protocols map[string]models.ProtocolProperties) (*ConnectionInfo, error) {
	props, exists := protocols[Protocol]
	if !exists {
		return nil, fmt.Errorf("protocol for [%s] not exists", Protocol)
	}

	var (
		endpointUrl       any
		securityPolicy    any
		securityMode      any
		authType          any
		username          any
		password          any
		autoReconnect     any
		reconnectInterval any
		maxPoolSize       any
	)

	endpointUrl, ok := props[EndpointField]
	if !ok {
		return nil, fmt.Errorf("unable to create OPC UA connection info, protocol config [%s] not exists", EndpointField)
	}

	securityPolicy, ok = props[SecurityPolicyField]
	if !ok {
		securityPolicy = SecurityPolicyNone
	}
	securityMode, ok = props[SecurityModeField]
	if !ok {
		if securityPolicy.(SecurityPolicy) == SecurityPolicyNone {
			securityMode = SecurityModeNone
		} else {
			securityMode = SecurityModeSign
		}
	}
	authType, ok = props[AuthTypeField]
	if !ok {
		authType = AuthTypeAnonymous
	}
	switch authType.(AuthType) {
	case AuthTypeAnonymous:
		break
	case AuthTypeUsername:
		username, ok = props[UsernameField]
		if !ok {
			return nil, fmt.Errorf("unable to create OPC UA connection info, missing username while Authentication Type is AuthTypeUsername")
		}
		password, ok = props[PasswordField]
		if !ok {
			password = ""
		}
	default:
		return nil, fmt.Errorf("unable to create OPC UA connection info, because of unsupported Authentication Type [%s]", authType)
	}

	autoReconnect, ok = props[AutoReconnectField]
	if !ok {
		autoReconnect = true
	}
	reconnectInterval, ok = props[ReconnectIntervalField]
	if !ok {
		reconnectInterval = 5 * time.Second
	}
	maxPoolSize, ok = props[MaxPoolSizeField]
	if !ok {
		maxPoolSize = 1
	}

	return &ConnectionInfo{
		EndpointURL:       endpointUrl.(string),
		SecurityPolicy:    securityPolicy.(SecurityPolicy),
		SecurityMode:      securityMode.(SecurityMode),
		AuthType:          authType.(AuthType),
		Username:          username.(string),
		Password:          password.(string),
		AutoReconnect:     autoReconnect.(bool),
		ReconnectInterval: reconnectInterval.(time.Duration),
		MaxPoolSize:       maxPoolSize.(uint32),
	}, nil
}

func CreateCommandInfo(resourceName string, req map[string]interface{}) (*CommandInfo, error) {
	ret := &CommandInfo{
		watchable: false,
	}

	watchable, hasWatchable := req[WATCHABLE]
	nodeId, hasNodeId := req[NODE]
	methodId, hasMethodId := req[METHOD]
	objectId, hasObjectId := req[OBJECT]
	inputMap, hasInputMap := req[INPUTMAP]
	if hasWatchable && watchable.(bool) {
		if !hasNodeId {
			return nil, errors.NewCommonEdgeX(errors.KindStatusConflict, fmt.Sprintf("missing required attribute 'nodeId' for watchable command: [%v]", resourceName), nil)
		}
		ret.watchable = true
		ret.nodeId = nodeId.(string)
	} else if !hasNodeId && !hasMethodId {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("either '%s' or '%s' should be defined for command: [%v]", NODE, METHOD, resourceName), nil)
	} else if hasNodeId && hasMethodId {
		slog.Warn(fmt.Sprintf("[Creating CommandInfo] both '%s' and '%s' are defined for command: [%v], '%s' will be ignored", NODE, METHOD, resourceName, METHOD))
		ret.nodeId = nodeId.(string)
	} else if hasMethodId {
		if !hasObjectId {
			return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("'%s' is required when '%s' is defined for command: [%v]", OBJECT, METHOD, resourceName), nil)
		}
		ret.methodId = methodId.(string)
		ret.objectId = objectId.(string)
		if hasInputMap {
			imElements := inputMap.([]interface{})
			if len(imElements) > 0 {
				ret.inputMap = make([]string, len(imElements))
				for i := 0; i < len(imElements); i++ {
					ret.inputMap[i] = imElements[i].(string)
				}
			}
		}
	} else {
		if !hasNodeId {
			return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("all required attributes not defined for command: [%v]", resourceName), nil)
		}
		ret.nodeId = nodeId.(string)
	}
	return ret, nil
}

// FetchEndpoint returns the OPCUA endpoint defined in the configuration
func FetchEndpoint(protocols map[string]models.ProtocolProperties) (string, errors.EdgeX) {
	properties, ok := protocols[Protocol]
	if !ok {
		return "", errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("'%s' protocol properties is not defined", Protocol), nil)
	}
	endpoint, ok := properties[EndpointField]
	if !ok {
		return "", errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("'%s' not found in the '%s' protocol properties", EndpointField, Protocol), nil)
	}
	endpointString, ok := endpoint.(string)
	if !ok {
		return "", errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("cannot convert '%v' to string type", endpoint), nil)
	}
	return endpointString, nil
}
