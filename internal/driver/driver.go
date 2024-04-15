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
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/edgexfoundry/device-sdk-go/v3/pkg/interfaces"
	sdkModel "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

var once sync.Once
var driver *Driver

// Driver struct
type Driver struct {
	Logger        logger.LoggingClient
	AsyncCh       chan<- *sdkModel.AsyncValues
	sdkService    interfaces.DeviceServiceSDK
	serviceConfig *ServiceConfig
	resourceMap   map[uint32]string
	mu            sync.Mutex
	ctxCancel     context.CancelFunc
	clientMap     map[string]*opcua.Client
}

// NewProtocolDriver returns a new protocol driver object
func NewProtocolDriver() interfaces.ProtocolDriver {
	once.Do(func() {
		driver = new(Driver)
	})
	return driver
}

// Initialize performs protocol-specific initialization for the device service
func (d *Driver) Initialize(sdk interfaces.DeviceServiceSDK) error {
	d.sdkService = sdk
	d.Logger = sdk.LoggingClient()
	d.AsyncCh = sdk.AsyncValuesChannel()
	d.serviceConfig = &ServiceConfig{}
	d.mu.Lock()
	d.resourceMap = make(map[uint32]string)
	d.clientMap = make(map[string]*opcua.Client)
	d.mu.Unlock()

	if err := sdk.LoadCustomConfig(d.serviceConfig, CustomConfigSectionName); err != nil {
		return errors.NewCommonEdgeX(errors.Kind(err), fmt.Sprintf("unable to load '%s' custom configuration", CustomConfigSectionName), err)
	}

	d.Logger.Debugf("Custom config is: %v", d.serviceConfig)

	if err := d.serviceConfig.OPCUAServer.Validate(); err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}

	if err := sdk.ListenForCustomConfigChanges(&d.serviceConfig.OPCUAServer.Writable, WritableInfoSectionName, d.updateWritableConfig); err != nil {
		return errors.NewCommonEdgeX(errors.Kind(err), fmt.Sprintf("unable to listen for changes for '%s' custom configuration", WritableInfoSectionName), err)
	}

	return nil
}

// Callback function provided to ListenForCustomConfigChanges to update
// the configuration when OPCUAServer.Writable changes
func (d *Driver) updateWritableConfig(rawWritableConfig interface{}) {
	updated, ok := rawWritableConfig.(*WritableInfo)
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

func (d *Driver) Start() error {
	return nil
}

// Stop the protocol-specific DS code to shutdown gracefully, or
// if the force parameter is 'true', immediately. The driver is responsible
// for closing any in-use channels, including the channel used to send async
// readings (if supported).
func (d *Driver) Stop(force bool) error {
	d.mu.Lock()
	d.resourceMap = nil
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, cli := range d.clientMap {
		cli.Close(ctx)
	}
	d.mu.Unlock()
	d.cleanup()
	return nil
}

func (d *Driver) Discover() error {
	return fmt.Errorf("driver's Discover function isn't implemented")
}

func (d *Driver) ValidateDevice(device models.Device) error {
	_, err := FetchEndpoint(device.Protocols)
	if err != nil {
		return fmt.Errorf("invalid protocol properties, %v", err)
	}
	return nil
}

func getNodeID(attrs map[string]interface{}, id string) (string, error) {
	identifier, ok := attrs[id]
	if !ok {
		return "", fmt.Errorf("attribute %s does not exist", id)
	}

	return identifier.(string), nil
}

func (d *Driver) buildClient(ctx context.Context, endpoint string) (*opcua.Client, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if client, ok := d.clientMap[endpoint]; ok {
		return client, nil
	}

	client, _ := opcua.NewClient(endpoint, opcua.SecurityMode(ua.MessageSecurityModeNone))
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}
	d.clientMap[endpoint] = client
	return client, nil
}
