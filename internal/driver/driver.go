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
	"context"
	"fmt"
	"github.com/edgexfoundry/device-sdk-go/v3/pkg/interfaces"
	sdkModel "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"sync"
)

var once sync.Once
var driver *Driver

// Driver struct
type Driver struct {
	Logger           logger.LoggingClient
	AsyncCh          chan<- *sdkModel.AsyncValues
	sdkService       interfaces.DeviceServiceSDK
	serviceConfig    *ServiceConfig
	resourceMap      map[uint32]ResourceDetail
	mu               sync.Mutex
	ctxCancel        map[string]context.CancelFunc
	uaConnectionPool *NamedConnectionPool
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
	d.resourceMap = make(map[uint32]ResourceDetail)
	d.ctxCancel = make(map[string]context.CancelFunc)
	d.mu.Unlock()

	if err := sdk.LoadCustomConfig(d.serviceConfig, CustomConfigSectionName); err != nil {
		return errors.NewCommonEdgeX(errors.Kind(err), fmt.Sprintf("unable to load '%s' custom configuration", CustomConfigSectionName), err)
	}

	d.Logger.Debugf("Custom config is: %v", d.serviceConfig)

	if err := sdk.ListenForCustomConfigChanges(&d.serviceConfig.OPCUAServer, CustomConfigSectionName, d.updateClientInfo); err != nil {
		return errors.NewCommonEdgeX(errors.Kind(err), fmt.Sprintf("unable to listen for changes for '%s' custom configuration", CustomConfigSectionName), err)
	}

	// Initialize ua connection pool
	d.uaConnectionPool = New(WaitForConnection, &d.serviceConfig.OPCUAServer)
	// create subscriptions for existing watchable resources
	d.startExistingWatchableResourcesListener()
	return nil
}

// Callback function provided to ListenForCustomConfigChanges to update
// the configuration when OPCUAServer.Writable changes
func (d *Driver) updateClientInfo(iClientInfo interface{}) {
	clientInfo, ok := iClientInfo.(*ClientInfo)
	if !ok {
		d.Logger.Error("updateClientInfo: type assertion failed")
		return
	}
	d.uaConnectionPool.Reset(clientInfo)

	// recreate subscriptions
	d.cleanupListeners()
	d.startExistingWatchableResourcesListener()
}

func (d *Driver) removeSubscriber(deviceName string) {
	cancelFunc, exists := d.ctxCancel[deviceName]
	if exists {
		cancelFunc()
		delete(d.ctxCancel, deviceName)
	}
}

// Start or restart the subscription listener
func (d *Driver) startSubscriber(devices []models.Device) {
	err := d.startSubscriptionListener(devices)
	if err != nil {
		d.Logger.Errorf("Driver.Initialize: Start incoming data Listener failed: %v", err)
	}
}

func (d *Driver) startSubscriberByDeviceName(deviceName string) {
	device, err := d.sdkService.GetDeviceByName(deviceName)
	if err != nil {
		d.Logger.Errorf("Driver.Initialize: Start incoming data Listener failed: %v", err)
		return
	}

	d.removeSubscriber(deviceName)

	go d.startSubscriber([]models.Device{device})
}

func (d *Driver) startExistingWatchableResourcesListener() {
	devices := d.sdkService.Devices()
	go d.startSubscriber(devices)
}

// Close the existing context.
// This, in turn, cancels the existing subscription if it exists
func (d *Driver) cleanupListeners() {
	if len(d.ctxCancel) > 0 {
		for deviceName, cancel := range d.ctxCancel {
			cancel()
			delete(d.ctxCancel, deviceName)
		}
		//d.ctxCancel = nil
	}
}

// AddDevice is a callback function that is invoked
// when a new Device associated with this Device Service is added
func (d *Driver) AddDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	// if device is locked, do nothing
	if adminState == models.Locked {
		return nil
	}

	// validate protocol properties
	_, err := createConnectionInfo(protocols)
	if err != nil {
		return err
	}

	// Start subscription listener when device is added.
	// This does not happen automatically like it does when the device is updated
	d.startSubscriberByDeviceName(deviceName)
	d.Logger.Debugf("Device %s is added\n", deviceName)
	return nil
}

// UpdateDevice is a callback function that is invoked
// when a Device associated with this Device Service is updated
func (d *Driver) UpdateDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	// if device is locked, try to terminate the connection pool
	if adminState == models.Locked {
		d.uaConnectionPool.TerminateNamedPool(deviceName)
	}
	info, err := createConnectionInfo(protocols)
	if err != nil {
		return err
	}
	// recreate connection pool
	d.uaConnectionPool.CheckUpdatesAndDoUpdate(deviceName, info)

	d.startSubscriberByDeviceName(deviceName)

	d.Logger.Debugf("Device %s is updated", deviceName)
	return nil
}

// RemoveDevice is a callback function that is invoked
// when a Device associated with this Device Service is removed
func (d *Driver) RemoveDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	d.uaConnectionPool.TerminateNamedPool(deviceName)
	d.removeSubscriber(deviceName)
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
	d.cleanupListeners()
	d.ctxCancel = nil
	d.uaConnectionPool.Reset(&ClientInfo{})
	d.mu.Unlock()
	d.cleanupListeners()
	return nil
}

func (d *Driver) Discover() error {
	return fmt.Errorf("driver's Discover function isn't implemented")
}

func (d *Driver) ValidateDevice(device models.Device) error {
	_, err := createConnectionInfo(device.Protocols)
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
