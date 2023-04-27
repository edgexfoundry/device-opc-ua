// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
// Copyright (C) 2021 Schneider Electric
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"github.com/edgexfoundry/device-opcua-go/internal/config"
	sdkModel "github.com/edgexfoundry/device-sdk-go/v2/pkg/models"
	"github.com/edgexfoundry/device-sdk-go/v2/pkg/service"
	"github.com/edgexfoundry/go-mod-bootstrap/v2/bootstrap/secret"
	"github.com/edgexfoundry/go-mod-bootstrap/v2/bootstrap/startup"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"math/big"
	"net"
	"net/url"
	"sync"
	"time"
)

var once sync.Once
var driver *Driver
var cert []byte

// Driver struct
type Driver struct {
	Logger        logger.LoggingClient
	AsyncCh       chan<- *sdkModel.AsyncValues
	serviceConfig *config.ServiceConfig
	resourceMap   map[uint32]string
	mu            sync.Mutex
	ctxCancel     context.CancelFunc
}

// NewProtocolDriver returns a new protocol driver object
func NewProtocolDriver() sdkModel.ProtocolDriver {
	once.Do(func() {
		driver = new(Driver)
	})
	return driver
}

// Initialize performs protocol-specific initialization for the device service
func (d *Driver) Initialize(lc logger.LoggingClient, asyncCh chan<- *sdkModel.AsyncValues, deviceCh chan<- []sdkModel.DiscoveredDevice) error {
	d.Logger = lc
	d.AsyncCh = asyncCh
	d.serviceConfig = &config.ServiceConfig{}
	d.mu.Lock()
	d.resourceMap = make(map[uint32]string)
	d.mu.Unlock()

	ds := service.RunningService()
	if ds == nil {
		return errors.NewCommonEdgeXWrapper(fmt.Errorf("unable to get running device service"))
	}

	if err := ds.LoadCustomConfig(d.serviceConfig, CustomConfigSectionName); err != nil {
		return errors.NewCommonEdgeX(errors.Kind(err), fmt.Sprintf("unable to load '%s' custom configuration", CustomConfigSectionName), err)
	}

	lc.Debugf("Custom config is: %v", d.serviceConfig)

	if err := d.serviceConfig.OPCUAServer.Validate(); err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}

	if err := ds.ListenForCustomConfigChanges(&d.serviceConfig.OPCUAServer.Writable, WritableInfoSectionName, d.updateWritableConfig); err != nil {
		return errors.NewCommonEdgeX(errors.Kind(err), fmt.Sprintf("unable to listen for changes for '%s' custom configuration", WritableInfoSectionName), err)
	}

	return nil
}

var (
	GetEndpoints = opcua.GetEndpoints
)

var (
	SelectEndPoint = opcua.SelectEndpoint
)

// creates the options to connect with a opcua Client based on the configured options.
func (d *Driver) createClientOptions() ([]opcua.Option, error) {
	availableServerEndpoints, err := GetEndpoints(d.serviceConfig.OPCUAServer.Endpoint)
	if err != nil {
		d.Logger.Error("OPC GetEndpoints: %w", err)
		return nil, err
	}
	credentials, err := d.getCredentials(d.serviceConfig.OPCUAServer.CredentialsPath)
	if err != nil {
		d.Logger.Error("getCredentials: %w", err)
		return nil, err
	}

	username := credentials.Username
	password := credentials.Password
	policy := ua.FormatSecurityPolicyURI(d.serviceConfig.OPCUAServer.Policy)
	mode := ua.MessageSecurityModeFromString(d.serviceConfig.OPCUAServer.Mode)

	ep := SelectEndPoint(availableServerEndpoints, policy, mode)
	c, err := generateCert() // This is where you generate the certificate
	if err != nil {
		d.Logger.Error("generateCert: %w", err)
		return nil, err
	}

	pk, ok := c.PrivateKey.(*rsa.PrivateKey) // This is where you set the private key
	if !ok {
		d.Logger.Error("invalid private key")
	}

	cert = c.Certificate[0]

	opts := []opcua.Option{
		opcua.SecurityPolicy(policy),
		opcua.SecurityMode(mode),
		opcua.PrivateKey(pk),
		opcua.Certificate(cert),                // Set the certificate for the OPC UA Client
		opcua.AuthUsername(username, password), // Use this if you are using username and password
		opcua.SecurityFromEndpoint(ep, ua.UserTokenTypeUserName),
		opcua.SessionTimeout(30 * time.Minute),
	}
	return opts, nil
}

// Gets the username and password credentials from the configuration.
func (d *Driver) getCredentials(secretPath string) (config.Credentials, error) {
	credentials := config.Credentials{}
	timer := startup.NewTimer(d.serviceConfig.OPCUAServer.CredentialsRetryTime, d.serviceConfig.OPCUAServer.CredentialsRetryWait)
	service := service.RunningService()
	var secretData map[string]string
	var err error
	for timer.HasNotElapsed() {
		secretData, err = service.SecretProvider.GetSecret(secretPath, secret.UsernameKey, secret.PasswordKey)
		if err == nil {
			break
		}

		d.Logger.Warnf(
			"Unable to retrieve OPCUA credentials from SecretProvider at path '%s': %s. Retrying for %s",
			secretPath,
			err.Error(),
			timer.RemainingAsString())
		timer.SleepForInterval()
	}

	if err != nil {
		return credentials, err
	}

	credentials.Username = secretData[secret.UsernameKey]
	credentials.Password = secretData[secret.PasswordKey]

	return credentials, nil
}

func generateCert() (*tls.Certificate, error) {

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %s", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // 1 year

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Client"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageContentCommitment | x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	host := "urn:testing:client"
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host)
	}
	if uri, err := url.Parse(host); err == nil {
		template.URIs = append(template.URIs, uri)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKeys(priv), priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %s", err)
	}

	certBuf := bytes.NewBuffer(nil)
	if err := pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, fmt.Errorf("failed to encode certificate: %s", err)
	}

	keyBuf := bytes.NewBuffer(nil)
	if err := pem.Encode(keyBuf, pemBlockForKeys(priv)); err != nil {
		return nil, fmt.Errorf("failed to encode key: %s", err)
	}

	cert, err := tls.X509KeyPair(certBuf.Bytes(), keyBuf.Bytes())
	return &cert, err
}

// Callback function provided to ListenForCustomConfigChanges to update
// the configuration when OPCUAServer.Writable changes
func (d *Driver) updateWritableConfig(rawWritableConfig interface{}) {
	updated, ok := rawWritableConfig.(*config.WritableInfo)
	if !ok {
		d.Logger.Error("unable to update writable config: Cannot cast raw config to type 'WritableInfo'")
		return
	}

	d.cleanup()

	d.serviceConfig.OPCUAServer.Writable = *updated

	go d.startSubscriber()
}

// Start or restart the subscription listener
func (d *Driver) startSubscriber() {
	err := d.startSubscriptionListener()
	if err != nil {
		d.Logger.Errorf("Driver.Initialize: Start incoming data Listener failed: %v", err)
	}
}

// Close the existing context.
// This, in turn, cancels the existing subscription if it exists
func (d *Driver) cleanup() {
	if d.ctxCancel != nil {
		d.ctxCancel()
		d.ctxCancel = nil
	}
}

// AddDevice is a callback function that is invoked
// when a new Device associated with this Device Service is added
func (d *Driver) AddDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	// Start subscription listener when device is added.
	// This does not happen automatically like it does when the device is updated
	go d.startSubscriber()
	d.Logger.Debugf("Device %s is added", deviceName)
	return nil
}

// UpdateDevice is a callback function that is invoked
// when a Device associated with this Device Service is updated
func (d *Driver) UpdateDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	d.Logger.Debugf("Device %s is updated", deviceName)
	return nil
}

// RemoveDevice is a callback function that is invoked
// when a Device associated with this Device Service is removed
func (d *Driver) RemoveDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	d.Logger.Debugf("Device %s is removed", deviceName)
	return nil
}

// Stop the protocol-specific DS code to shutdown gracefully, or
// if the force parameter is 'true', immediately. The driver is responsible
// for closing any in-use channels, including the channel used to send async
// readings (if supported).
func (d *Driver) Stop(force bool) error {
	d.mu.Lock()
	d.resourceMap = nil
	d.mu.Unlock()
	d.cleanup()
	return nil
}

func getNodeID(attrs map[string]interface{}, id string) (string, error) {
	identifier, ok := attrs[id]
	if !ok {
		return "", fmt.Errorf("attribute %s does not exist", id)
	}

	return identifier.(string), nil
}
