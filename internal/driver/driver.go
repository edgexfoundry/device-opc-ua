// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
// Copyright (C) 2021 Schneider Electric
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
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
	"log"
	"math/big"
	"os"
	"strconv"
	"sync"
	"time"
)

var once sync.Once
var driver *Driver

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
func createX509Template() x509.Certificate {
	template := x509.Certificate{
		SerialNumber: big.NewInt(2023),
		Subject: pkix.Name{
			Organization: []string{driver.serviceConfig.OPCUAServer.CertificateConfig.CertOrganization},
			Country:      []string{driver.serviceConfig.OPCUAServer.CertificateConfig.CertCountry},
			Province:     []string{driver.serviceConfig.OPCUAServer.CertificateConfig.CertProvince},
			Locality:     []string{driver.serviceConfig.OPCUAServer.CertificateConfig.CertLocality},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		BasicConstraintsValid: true,
	}
	return template
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

	// Add listener for username and password changes in insecure secrets config section
	if err := ds.ListenForCustomConfigChanges(&d.serviceConfig.OPCUAServer.Writable, InsecureSecretsConfigSectionName, d.updateWritableConfig); err != nil {
		return errors.NewCommonEdgeX(errors.Kind(err), fmt.Sprintf("unable to listen for changes for '%s' custom configuration", InsecureSecretsConfigSectionName), err)
	}
	// Add listener for changes in opcua custom config section
	if err := ds.ListenForCustomConfigChanges(&d.serviceConfig.OPCUAServer.Writable, CustomConfigSectionName, d.updateWritableConfig); err != nil {
		return errors.NewCommonEdgeX(errors.Kind(err), fmt.Sprintf("unable to listen for changes for '%s' custom configuration", CustomConfigSectionName), err)
	}
	return nil
}

// GetEndpoints capsules the containing method for easy mocking in unit tests
var (
	GetEndpoints = opcua.GetEndpoints
)

// SelectEndPoint capsules the containing method for easy mocking in unit tests
var (
	SelectEndPoint = opcua.SelectEndpoint
)

// ReadCertAndKey capsules the containing method for easy mocking in unit tests
var (
	ReadCertAndKey = ReadClientCertAndPrivateKey
)

// CertKeyPair capsules the containing method for easy mocking in unit tests
var (
	CertKeyPair = tls.X509KeyPair
)

func ReadClientCertAndPrivateKey(clientCertFileName, clientKeyFileName string) ([]byte, []byte, error) {
	clientCertificate, err := os.ReadFile(clientCertFileName)
	var privateKey []byte = nil

	if err != nil {
		log.Println("Client certificate not existing, creating new one")

		clientCert, clientKey, err := CreateSelfSignedClientCertificates("localhost")
		if err != nil {
			return nil, nil, err
		}

		var perm int
		perm, err = strconv.Atoi(driver.serviceConfig.OPCUAServer.CertificateConfig.CertFilePermissions)
		if err != nil {
			log.Println("Could not convert permission string to uint:", err)
			return nil, nil, err
		}

		err = os.WriteFile(clientCertFileName, clientCert, os.FileMode(perm))
		if err != nil {
			return nil, nil, err
		}
		err = os.WriteFile(clientKeyFileName, clientKey, os.FileMode(perm))
		if err != nil {
			return nil, nil, err
		}

		clientCertificate = clientCert
		privateKey = clientKey
	} else {
		privateKey, err = os.ReadFile(clientKeyFileName)
		if err != nil {
			return nil, nil, err
		}
	}
	println("Successfully load certificates from file")
	return clientCertificate, privateKey, nil
}
func CreateSelfSignedClientCertificates(clientName string) ([]byte, []byte, error) {
	template := createX509Template()
	clientPrivateKey, err := rsa.GenerateKey(rand.Reader, driver.serviceConfig.OPCUAServer.CertificateConfig.CertBits)
	if err != nil {
		log.Fatal(err)
	}
	clientPrivateKeyBytes := x509.MarshalPKCS1PrivateKey(clientPrivateKey)
	clientPrivateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: clientPrivateKeyBytes})

	clientTemplate := setClientTemplateOptions(template, clientName)

	serverBytes, err := x509.CreateCertificate(rand.Reader, &clientTemplate, &clientTemplate, &clientPrivateKey.PublicKey, clientPrivateKey)
	if err != nil {
		return nil, nil, err
	}
	clientPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverBytes})
	return clientPem, clientPrivateKeyPEM, nil
}

// setClientTemplateOptions Prepare template for generating client certificate
func setClientTemplateOptions(template x509.Certificate, commonName string) x509.Certificate {
	template.Subject.CommonName = commonName
	template.IsCA = false
	template.KeyUsage = x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
	template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}

	return template
}

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

	var opts []opcua.Option

	ep := SelectEndPoint(availableServerEndpoints, policy, mode)

	// no need to set options if no security policy is set
	if mode != ua.MessageSecurityModeNone {
		clientCertFileName := d.serviceConfig.OPCUAServer.CertificateConfig.CertFile
		clientKeyFileName := d.serviceConfig.OPCUAServer.CertificateConfig.KeyFile

		cert, key, err := ReadCertAndKey(clientCertFileName, clientKeyFileName)

		if err != nil {
			return nil, err
		}
		clientCertificate, err := CertKeyPair(cert, key)

		pk, ok := clientCertificate.PrivateKey.(*rsa.PrivateKey) // This is where you set the private key
		if !ok {
			d.Logger.Error("invalid private key")
		}

		cert = clientCertificate.Certificate[0]

		opts = []opcua.Option{
			opcua.SecurityPolicy(policy),
			opcua.SecurityMode(mode),
			opcua.PrivateKey(pk),
			opcua.Certificate(cert),                // Set the certificate for the OPC UA Client
			opcua.AuthUsername(username, password), // Use this if you are using username and password
			opcua.SecurityFromEndpoint(ep, ua.UserTokenTypeUserName),
			opcua.SessionTimeout(30 * time.Minute),
		}
	}
	return opts, nil
}

// Gets the username and password credentials from the configuration.
func (d *Driver) getCredentials(secretPath string) (config.Credentials, error) {
	credentials := config.Credentials{}
	timer := startup.NewTimer(d.serviceConfig.OPCUAServer.CredentialsRetryTime, d.serviceConfig.OPCUAServer.CredentialsRetryWait)
	runningService := service.RunningService()
	var secretData map[string]string
	var err error
	for timer.HasNotElapsed() {
		secretData, err = runningService.SecretProvider.GetSecret(secretPath, secret.UsernameKey, secret.PasswordKey)
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

// Callback function provided to ListenForCustomConfigChanges to update
// the configuration a config section changes, for example via consul
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
