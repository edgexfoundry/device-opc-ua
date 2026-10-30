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

	"github.com/edgexfoundry/device-sdk-go/v4/pkg/interfaces"
	sdkModel "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
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
	clients       map[string]*clientSlot
	deviceKeys    map[string]string
	stopped       bool
	// dial and secrets are test seams. Production leaves them nil.
	dial    func(ctx context.Context, info sessionInfo) (uaSession, error)
	secrets secretGetter
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
	d.clients = make(map[string]*clientSlot)
	d.deviceKeys = make(map[string]string)
	d.stopped = false
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

// Start or restart the subscription listener.
// A transport failure drops the shared session and dials again. A configuration
// error stops the listener. cleanup cancels the context and ends the retry.
func (d *Driver) startSubscriber() {
	resources := ""
	if d.serviceConfig != nil {
		resources = d.serviceConfig.OPCUAServer.Writable.Resources
	}
	if len(resources) == 0 {
		d.Logger.Info("[Incoming listener] No resources defined to generate subscriptions.")
		return
	}

	ctx, ok := d.installListenerContext()
	if !ok {
		return
	}

	backoff := time.Second
	for {
		err := d.startSubscriptionListener(ctx)
		if err == nil || ctx.Err() != nil {
			return
		}
		d.Logger.Errorf("Driver.Initialize: Start incoming data Listener failed: %v", err)
		if isPermanentListenerError(err) {
			return
		}
		d.mu.Lock()
		stopped := d.stopped
		d.mu.Unlock()
		if stopped {
			return
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// installListenerContext replaces the listener context.
// A service that is already stopping does not keep the new context.
// The previous cancel function runs after the lock is released.
func (d *Driver) installListenerContext() (context.Context, bool) {
	ctx, cancel := context.WithCancel(context.Background())
	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		cancel()
		return nil, false
	}
	prev := d.ctxCancel
	d.ctxCancel = cancel
	d.mu.Unlock()
	if prev != nil {
		prev()
	}
	return ctx, true
}

// Close the existing context.
// This, in turn, cancels the existing subscription if it exists.
func (d *Driver) cleanup() {
	d.mu.Lock()
	cancel := d.ctxCancel
	d.ctxCancel = nil
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// AddDevice is a callback function that is invoked
// when a new Device associated with this Device Service is added
func (d *Driver) AddDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	if err := d.trackDevice(deviceName, protocols); err != nil {
		return err
	}
	// Start subscription listener when device is added.
	// This does not happen automatically like it does when the device is updated
	go d.startSubscriber()
	d.Logger.Debugf("Device %s is added", deviceName)
	return nil
}

// UpdateDevice is a callback function that is invoked
// when a Device associated with this Device Service is updated
func (d *Driver) UpdateDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	info, err := d.sessionInfo(protocols)
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.ensureMapsLocked()
	prev, had := d.deviceKeys[deviceName]
	d.mu.Unlock()
	restart := had && prev != info.Key() && d.subscriptionDevice(deviceName)
	if restart {
		// Let the listener drop its in-flight use before the old session can close.
		d.cleanup()
		d.waitKeyIdle(prev)
	}
	if err := d.trackDevice(deviceName, protocols); err != nil {
		return err
	}
	if restart {
		go d.startSubscriber()
	}
	d.Logger.Debugf("Device %s is updated", deviceName)
	return nil
}

// RemoveDevice is a callback function that is invoked
// when a Device associated with this Device Service is removed
func (d *Driver) RemoveDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	if d.subscriptionDevice(deviceName) {
		d.mu.Lock()
		d.ensureMapsLocked()
		key := d.deviceKeys[deviceName]
		d.mu.Unlock()
		// Cancel the subscription only. Client.Close belongs to the refcount.
		d.cleanup()
		if key != "" {
			d.waitKeyIdle(key)
		}
	}
	d.releaseDevice(deviceName)
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
	d.stopped = true
	d.resourceMap = nil
	d.mu.Unlock()
	// The listener ends its subscription and its in-flight count. It does not Close.
	d.cleanup()
	d.waitIdle()
	d.mu.Lock()
	d.ensureMapsLocked()
	var closing []uaSession
	for key, slot := range d.clients {
		closing = append(closing, detachLocked(slot)...)
		delete(d.clients, key)
	}
	d.deviceKeys = make(map[string]string)
	d.mu.Unlock()
	closeSessions(closing)
	return nil
}

func (d *Driver) Discover() error {
	return fmt.Errorf("driver's Discover function isn't implemented")
}

func (d *Driver) ValidateDevice(device models.Device) error {
	_, err := d.sessionInfo(device.Protocols)
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
