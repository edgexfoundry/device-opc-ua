// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

//
package driver

import (
	"context"
	"fmt"
	"github.com/edgexfoundry/device-sdk-go"
	sdkModel "github.com/edgexfoundry/device-sdk-go/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"sync"
)

var once sync.Once
var driver *Driver

type Driver struct {
	Logger           logger.LoggingClient
	AsyncCh          chan<- *sdkModel.AsyncValues
	CommandResponses sync.Map
	Config           *SubscribeJson
}

func NewProtocolDriver() sdkModel.ProtocolDriver {
	once.Do(func() {
		driver = new(Driver)
	})
	return driver
}

// Initialize performs protocol-specific initialization for the device
// service.
func (d *Driver) Initialize(lc logger.LoggingClient, asyncCh chan<- *sdkModel.AsyncValues) error {
	d.Logger = lc
	d.AsyncCh = asyncCh
	config, err := CreateDriverConfig(device.DriverConfigs())
	if err != nil {
		d.Logger.Error(fmt.Sprintf("Driver.Initialize: Read OPCUA driver configuration failed: %v", err))
	}
	d.Config = config

	err = startIncomingListening()
	if err != nil {
		d.Logger.Error(fmt.Sprintf("Driver.Initialize: Start incoming data Listener failed: %v", err))
	}
	return err
}

func (d *Driver) DisconnectDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	d.Logger.Warn("Driver was disconnected")
	return nil
}

// HandleReadCommands triggers a protocol Read operation for the specified device.
func (d *Driver) HandleReadCommands(deviceName string, protocols map[string]models.ProtocolProperties,
	reqs []sdkModel.CommandRequest) ([]*sdkModel.CommandValue, error) {

	driver.Logger.Debug(fmt.Sprintf("Driver.HandleReadCommands: protocols: %v resource: %v attributes: %v", protocols, reqs[0].DeviceResourceName, reqs[0].Attributes))
	var responses = make([]*sdkModel.CommandValue, len(reqs))
	var err error

	// create device client and open connection
	connectionInfo, err := CreateConnectionInfo(protocols)
	ctx := context.Background()

	client := opcua.NewClient(connectionInfo.Endpoint, opcua.SecurityMode(ua.MessageSecurityModeNone))
	if err := client.Connect(ctx); err != nil {
		return responses, err
	}
	defer client.Close()

	for i, req := range reqs {
		// handle every reqs
		res, err := d.handleReadCommandRequest(client, req)
		if err != nil {
			driver.Logger.Error(fmt.Sprintf("Driver.HandleReadCommands: Handle read commands failed: %v", err))
			return responses, err
		}
		responses[i] = res
	}

	return responses, err
}

func (d *Driver) handleReadCommandRequest(deviceClient *opcua.Client,
	req sdkModel.CommandRequest) (*sdkModel.CommandValue, error) {
	var result = &sdkModel.CommandValue{}
	var err error
	nodeID := req.DeviceResourceName

	// get NewNodeID
	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		driver.Logger.Error(fmt.Sprintf("Driver.handleReadCommands: Invalid node id=%s", nodeID))
		return result, err
	}

	// make and execute ReadRequest
	request := &ua.ReadRequest{
		MaxAge: 2000,
		NodesToRead: []*ua.ReadValueID{
			&ua.ReadValueID{NodeID: id},
		},
		TimestampsToReturn: ua.TimestampsToReturnBoth,
	}
	resp, err := deviceClient.Read(request)
	if err != nil {
		driver.Logger.Error(fmt.Sprintf("Driver.handleReadCommands: Read failed: %s", err))
	}
	if resp.Results[0].Status != ua.StatusOK {
		driver.Logger.Error(fmt.Sprintf("Driver.handleReadCommands: Status not OK: %v", resp.Results[0].Status))

	}

	// make new result
	reading := resp.Results[0].Value.Value()
	result, err = newResult(req, reading)
	if err != nil {
		return result, err
	} else {
		driver.Logger.Info(fmt.Sprintf("Get command finished: %v", result))
	}

	return result, err
}

// HandleWriteCommands passes a slice of CommandRequest struct each representing
// a ResourceOperation for a specific device resource (aka DeviceObject).
// Since the commands are actuation commands, params provide parameters for the individual
// command.
func (d *Driver) HandleWriteCommands(deviceName string, protocols map[string]models.ProtocolProperties,
	reqs []sdkModel.CommandRequest, params []*sdkModel.CommandValue) error {

	driver.Logger.Debug(fmt.Sprintf("SimpleDriver.HandleWriteCommands: protocols: %v, resource: %v, parameters: %v", protocols, reqs[0].DeviceResourceName, params))
	var err error

	// create device client and open connection
	connectionInfo, err := CreateConnectionInfo(protocols)
	ctx := context.Background()
	c := opcua.NewClient(connectionInfo.Endpoint, opcua.SecurityMode(ua.MessageSecurityModeNone))
	if err := c.Connect(ctx); err != nil {
		driver.Logger.Warn(fmt.Sprintf("Driver.HandleWriteCommands: Failed to create OPCUA client, %s", err))
		return  err
	}

	for _, req := range reqs {
		// handle every reqs every params
		for _, param := range params {
			err := d.handleWriteCommandRequest(c, req, param)
			if err != nil {
				driver.Logger.Error(fmt.Sprintf("Driver.HandleWriteCommands: Handle write commands failed: %v", err))
				return  err
			}
		}

	}

	return err
}

func (d *Driver) handleWriteCommandRequest(deviceClient *opcua.Client, req sdkModel.CommandRequest,
	param *sdkModel.CommandValue) error {
	var err error
	nodeID := req.DeviceResourceName

	// get NewNodeID
	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("Driver.handleWriteCommands: Invalid node id=%s", nodeID))
	}

	value, err := newCommandValue(req.Type, param)
	if err != nil {
		return err
	}
	v, err := ua.NewVariant(value)

	if err != nil {
		return fmt.Errorf(fmt.Sprintf("Driver.handleWriteCommands: invalid value: %v", err))
	}

	request := &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{
			&ua.WriteValue{
				NodeID:      id,
				AttributeID: ua.AttributeIDValue,
				Value: &ua.DataValue{
					EncodingMask: uint8(13),  // encoding mask
					Value:        v,
				},
			},
		},
	}

	resp, err := deviceClient.Write(request)
	if err != nil {
		driver.Logger.Error(fmt.Sprintf("Driver.handleWriteCommands: Write value %v failed: %s", v, err))
		return err
	}
	driver.Logger.Info(fmt.Sprintf("Driver.handleWriteCommands:  %v", resp.Results[0]))
	return nil
}


// Stop the protocol-specific DS code to shutdown gracefully, or
// if the force parameter is 'true', immediately. The driver is responsible
// for closing any in-use channels, including the channel used to send async
// readings (if supported).
func (d *Driver) Stop(force bool) error {
	d.Logger.Warn("Driver's Stop function didn't implement")
	return nil
}

// AddDevice is a callback function that is invoked
// when a new Device associated with this Device Service is added
func (d *Driver) AddDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	d.Logger.Debug(fmt.Sprintf("Device %s is updated", deviceName))
	return nil
}

// UpdateDevice is a callback function that is invoked
// when a Device associated with this Device Service is updated
func (d *Driver) UpdateDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	d.Logger.Debug(fmt.Sprintf("Device %s is updated", deviceName))
	return nil
}

// RemoveDevice is a callback function that is invoked
// when a Device associated with this Device Service is removed
func (d *Driver) RemoveDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	d.Logger.Debug(fmt.Sprintf("Device %s is updated", deviceName))
	return nil
}