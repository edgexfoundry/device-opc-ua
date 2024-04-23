/*
 * Copyright (c) 2024.  liushenglong_8597@outlook.com.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package driver

import (
	"context"
	"fmt"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"time"
)

func checkConnection(client *opcua.Client) bool {
	return client.State() != opcua.Closed
}

func closeConnection(client *opcua.Client) {
	if checkConnection(client) {
		_ = client.Close(context.Background())
	}
}

func createUaConnection(connectionInfo *ConnectionInfo, clientConfig *ClientInfo) (*opcua.Client, error) {
	var (
		endpoint          = connectionInfo.EndpointURL
		securityPolicy    = connectionInfo.SecurityPolicy
		securityMode      = connectionInfo.SecurityMode
		remoteCert        = connectionInfo.RemotePemCert
		authType          = connectionInfo.AuthType
		authUsername      = connectionInfo.Username
		authPassword      = connectionInfo.Password
		autoReconnect     = connectionInfo.AutoReconnect
		reconnectInterval = connectionInfo.ReconnectInterval
		applicationUri    = clientConfig.ApplicationURI
		certificateFile   = clientConfig.CertFile
		privateKeyFile    = clientConfig.KeyFile
	)
	opts := []opcua.Option{
		opcua.SecurityPolicy(securityPolicy.String()),
		opcua.SecurityModeString(securityMode.String()),
		opcua.AutoReconnect(autoReconnect),
		opcua.ReconnectInterval(reconnectInterval),
		opcua.Lifetime(3600 * time.Second),
		opcua.SessionTimeout(3600 * time.Second),
	}
	if securityMode != SecurityModeNone {
		if len(remoteCert) == 0 {
			return nil, fmt.Errorf("remote certificate is required for security mode %s", securityMode)
		}
		remoteCertData, err := loadCertificate([]byte(remoteCert))
		if err != nil {
			return nil, err
		}
		opts = append(opts, opcua.RemoteCertificate(remoteCertData))
	}

	// check client certificate and apply
	if len(certificateFile) > 0 && len(privateKeyFile) > 0 {
		opts = append(opts,
			opcua.CertificateFile(certificateFile),
			opcua.PrivateKeyFile(privateKeyFile),
			opcua.ApplicationURI(applicationUri),
			opcua.ProductURI(applicationUri))
	}

	// authentication type
	switch authType {
	case AuthTypeAnonymous:
		opts = append(opts, opcua.AuthAnonymous())
	case AuthTypeUsername:
		opts = append(opts, opcua.AuthUsername(authUsername, authPassword))
	default:
		return nil, fmt.Errorf("auth type %s not supported yet", authType)
	}
	ctx := context.Background()
	endpoints, err := opcua.GetEndpoints(ctx, endpoint, opts...)
	if err != nil {
		return nil, err
	}
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints found for %s", endpoint)
	}
	ep := opcua.SelectEndpoint(endpoints, securityPolicy.String(), ua.MessageSecurityModeFromString(securityMode.String()))
	if ep == nil {
		return nil, fmt.Errorf("No exact security configuration match is found, \nendpoint: %s \nsecurity policy: %s\nsecurity mode: %s",
			endpoint, securityPolicy, securityMode)
	}
	opts = append(opts, opcua.SecurityFromEndpoint(ep, ua.UserTokenTypeFromString(authType.String())))

	client, err := opcua.NewClient(endpoint, opts...)
	if err != nil {
		return nil, err
	}
	if err = client.Connect(ctx); err != nil {
		return nil, err
	}
	return client, nil
}
