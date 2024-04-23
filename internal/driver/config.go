// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
// Copyright (C) 2021 Schneider Electric
// Copyright (C) 2024 YIQISOFT
// Copyright (C) 2024 liushenglong_8597@outlook.com
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"fmt"
	"github.com/spf13/cast"
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
	RemotePemCert  string
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

type CommandInfo struct {
	resourceName string
	watchable    bool
	nodeId       string
	methodId     string
	objectId     string
	inputMap     []string
	valueType    string
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
		err               error
		endpointUrl       string
		securityPolicy    SecurityPolicy
		securityMode      SecurityMode
		remotePemCert     string
		authType          AuthType
		username          string
		password          string
		autoReconnect     bool
		reconnectInterval time.Duration
		maxPoolSize       uint32
	)

	ep, ok := props[EndpointField]
	if !ok {
		return nil, fmt.Errorf("unable to create OPC UA connection info, protocol config [%s] not exists", EndpointField)
	}
	endpointUrl = cast.ToString(ep)
	sp, ok := props[SecurityPolicyField]
	if !ok {
		return nil, fmt.Errorf("unable to create OPC UA connection info, missing SecurityPolicy")
	} else {
		securityPolicy, err = toSecurityPolicy(sp)
		if err != nil {
			return nil, err
		}
	}

	// if security policy is SecurityPolicyNone, then security mode can only be SecurityModeNone.
	// meanwhile, if security policy is not SecurityPolicyNone, then security mode is required.
	if securityPolicy == SecurityPolicyNone {
		securityMode = SecurityModeNone
	} else {
		sm, ok := props[SecurityModeField]
		if !ok {
			return nil, fmt.Errorf("unable to create OPC UA connection info, missing SecurityMode while SecurityPolicy is not SecurityPolicyNone")
		} else {
			securityMode, err = toSecurityMode(sm)
			if err != nil {
				return nil, err
			}
			if securityMode == SecurityModeNone {
				return nil, fmt.Errorf("unable to create OPC UA connection info, SecurityMode cannot be SecurityModeNone while SecurityPolicy is not SecurityPolicyNone")
			}
		}
	}

	// if security mode is not SecurityModeNone, then server certificate is required to apply the asymmetric encryption
	if securityMode != SecurityModeNone {
		rpc, ok := props[RemotePemCertField]
		if !ok {
			return nil, fmt.Errorf("unable to create OPC UA connection info, missing RemotePemCert while SecurityMode is not SecurityModeNone")
		}
		remotePemCert = fmt.Sprintf("%v", rpc)
	}

	at, ok := props[AuthTypeField]
	if !ok {
		authType = AuthTypeAnonymous
	} else {
		authType, err = toAuthType(at)
		if err != nil {
			return nil, err
		}
	}
	if authType == AuthTypeUsername {
		uname, ok := props[UsernameField]
		if !ok {
			return nil, fmt.Errorf("unable to create OPC UA connection info, missing username while Authentication Type is AuthTypeUsername")
		}
		username = cast.ToString(uname)
		passwd, ok := props[PasswordField]
		if !ok {
			password = ""
		} else {
			password = cast.ToString(passwd)
		}
	}

	arc, ok := props[AutoReconnectField]
	if !ok {
		autoReconnect = true
	} else {
		autoReconnect = cast.ToBool(arc)
	}

	ri, ok := props[ReconnectIntervalField]
	if !ok {
		reconnectInterval = 5 * time.Second
	} else {
		// this will handle pure digital characters and expressions like "5s", "5m" etc.
		reconnectInterval = cast.ToDuration(ri)
	}
	mps, ok := props[MaxPoolSizeField]
	if !ok {
		maxPoolSize = 1
	} else {
		maxPoolSize = cast.ToUint32(mps)
	}

	return &ConnectionInfo{
		EndpointURL:       endpointUrl,
		SecurityPolicy:    securityPolicy,
		SecurityMode:      securityMode,
		RemotePemCert:     remotePemCert,
		AuthType:          authType,
		Username:          username,
		Password:          password,
		AutoReconnect:     autoReconnect,
		ReconnectInterval: reconnectInterval,
		MaxPoolSize:       maxPoolSize,
	}, nil
}

func toSecurityPolicy(sp any) (SecurityPolicy, error) {
	spStr := fmt.Sprintf("%v", sp)
	if len(spStr) == 0 {
		return SecurityPolicyNone, nil
	}

	policy := SecurityPolicy(spStr)
	switch policy {
	case SecurityPolicyNone:
	case SecurityPolicyBasic256:
	case SecurityPolicyBasic128Rsa15:
	case SecurityPolicyBasic256Sha256:
	case SecurityPolicyAes128Sha256RsaOaep:
	case SecurityPolicyAes256Sha256RsaPss:
	default:
		return "", fmt.Errorf("unsupported SecurityPolicy [%s]", spStr)
	}
	return policy, nil
}

func toSecurityMode(mode interface{}) (SecurityMode, error) {
	switch mode {
	case SecurityModeNone:
		return SecurityModeNone, nil
	case SecurityModeSign:
		return SecurityModeSign, nil
	case SecurityModeSignAndEncrypt:
		return SecurityModeSignAndEncrypt, nil
	default:
		return "", fmt.Errorf("unsupported SecurityMode [%s]", mode)
	}
}

func toAuthType(at interface{}) (AuthType, error) {
	switch at {
	case AuthTypeAnonymous:
		return AuthTypeAnonymous, nil
	case AuthTypeUsername:
		return AuthTypeUsername, nil
	default:
		return "", fmt.Errorf("unable to create OPC UA connection info, because of unsupported Authentication Type [%s]", at)
	}
}

func CreateCommandInfo(resourceName string, valueType string, req map[string]interface{}) (*CommandInfo, error) {
	ret := &CommandInfo{
		resourceName: resourceName,
		watchable:    false,
		valueType:    valueType,
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
		ret.methodId = cast.ToString(methodId)
		ret.objectId = cast.ToString(objectId)
		if hasInputMap {
			imElements := inputMap.([]interface{})
			if len(imElements) > 0 {
				ret.inputMap = make([]string, len(imElements))
				for i := 0; i < len(imElements); i++ {
					ret.inputMap[i] = cast.ToString(imElements[i])
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
