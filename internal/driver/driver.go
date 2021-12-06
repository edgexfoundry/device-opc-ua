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
	"fmt"
	"sync"

	"github.com/edgexfoundry/device-opcua-go/internal/config"
	sdkModel "github.com/edgexfoundry/device-sdk-go/v2/pkg/models"
	"github.com/edgexfoundry/device-sdk-go/v2/pkg/service"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/models"
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

// Build a NodeID string in the form of ns=_;s=_
// based on attributes of a device resource
func buildNodeID(attrs map[string]interface{}, sKey string) (string, error) {
	if _, ok := attrs[NAMESPACE]; !ok {
		return "", fmt.Errorf("attribute %s does not exist", NAMESPACE)
	}
	if _, ok := attrs[sKey]; !ok {
		return "", fmt.Errorf("attribute %s does not exist", sKey)
	}

	return fmt.Sprintf("ns=%s;s=%s", attrs[NAMESPACE].(string), attrs[sKey].(string)), nil
}
