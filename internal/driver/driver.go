// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	sdkModel "github.com/edgexfoundry/device-sdk-go/v2/pkg/models"
	"github.com/edgexfoundry/device-sdk-go/v2/pkg/service"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"github.com/spf13/cast"
)

var once sync.Once
var driver *Driver

// Driver struct
type Driver struct {
	Logger           logger.LoggingClient
	AsyncCh          chan<- *sdkModel.AsyncValues
	CommandResponses sync.Map
	serviceConfig    *ServiceConfig
}

// NewProtocolDriver returns a new protocol driver object
func NewProtocolDriver() sdkModel.ProtocolDriver {
	once.Do(func() {
		driver = new(Driver)
	})
	return driver
}

// Initialize performs protocol-specific initialization for the device
// service.
func (d *Driver) Initialize(lc logger.LoggingClient, asyncCh chan<- *sdkModel.AsyncValues, deviceCh chan<- []sdkModel.DiscoveredDevice) error {
	d.Logger = lc
	d.AsyncCh = asyncCh
	d.serviceConfig = &ServiceConfig{}

	ds := service.RunningService()

	if err := ds.LoadCustomConfig(d.serviceConfig, CustomConfigSectionName); err != nil {
		return fmt.Errorf("unable to load '%s' custom configuration: %s", CustomConfigSectionName, err.Error())
	}

	lc.Debugf("Custom config is: %v", d.serviceConfig)

	if err := d.serviceConfig.OPCUAServer.Validate(); err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}

	// go func() {
	// 	err := startIncomingListening()
	// 	if err != nil {
	// 		panic(fmt.Errorf("Driver.Initialize: Start incoming data Listener failed: %v", err))
	// 	}
	// }()
	return nil
}

// AddDevice is a callback function that is invoked
// when a new Device associated with this Device Service is added
func (d *Driver) AddDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
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

// HandleReadCommands triggers a protocol Read operation for the specified device.
func (d *Driver) HandleReadCommands(deviceName string, protocols map[string]models.ProtocolProperties,
	reqs []sdkModel.CommandRequest) ([]*sdkModel.CommandValue, error) {

	d.Logger.Debugf("Driver.HandleReadCommands: protocols: %v resource: %v attributes: %v", protocols, reqs[0].DeviceResourceName, reqs[0].Attributes)
	var responses = make([]*sdkModel.CommandValue, len(reqs))

	// create device client and open connection
	endpoint, err := fetchEndpoint(protocols)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()

	client := opcua.NewClient(endpoint, opcua.SecurityMode(ua.MessageSecurityModeNone))
	if err := client.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	for i, req := range reqs {
		// handle every reqs
		res, err := d.handleReadCommandRequest(client, req)
		if err != nil {
			d.Logger.Errorf("Driver.HandleReadCommands: Handle read commands failed: %v", err)
			return responses, err
		}
		responses[i] = res
	}

	return responses, err
}

func (d *Driver) handleReadCommandRequest(deviceClient *opcua.Client, req sdkModel.CommandRequest) (*sdkModel.CommandValue, error) {
	var result = &sdkModel.CommandValue{}
	var err error

	_, isMethod := req.Attributes[METHOD]

	if isMethod {
		result, err = makeMethodCall(deviceClient, req)
		d.Logger.Infof("Method command finished: %v", result)
	} else {
		result, err = makeReadRequest(deviceClient, req)
		d.Logger.Infof("Read command finished: %v", result)
	}

	return result, err
}

func makeReadRequest(deviceClient *opcua.Client, req sdkModel.CommandRequest) (*sdkModel.CommandValue, error) {
	nodeID, err := buildNodeID(req.Attributes, SYMBOL)
	if err != nil {
		return nil, fmt.Errorf("Driver.handleReadCommands: %v", err)
	}

	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("Driver.handleReadCommands: Invalid node id=%s; %v", nodeID, err)
	}

	request := &ua.ReadRequest{
		MaxAge: 2000,
		NodesToRead: []*ua.ReadValueID{
			{NodeID: id},
		},
		TimestampsToReturn: ua.TimestampsToReturnBoth,
	}
	resp, err := deviceClient.Read(request)
	if err != nil {
		return nil, fmt.Errorf("Driver.handleReadCommands: Read failed: %s", err)
	}
	if resp.Results[0].Status != ua.StatusOK {
		return nil, fmt.Errorf("Driver.handleReadCommands: Status not OK: %v", resp.Results[0].Status)
	}

	// make new result
	reading := resp.Results[0].Value.Value()
	return newResult(req, reading)
}

func makeMethodCall(deviceClient *opcua.Client, req sdkModel.CommandRequest) (*sdkModel.CommandValue, error) {
	var inputs []*ua.Variant

	objectID, err := buildNodeID(req.Attributes, OBJECT)
	if err != nil {
		return nil, fmt.Errorf("Driver.handleReadCommands: %v", err)
	}
	oid, err := ua.ParseNodeID(objectID)
	if err != nil {
		return nil, fmt.Errorf("Driver.handleReadCommands: %v", err)
	}

	methodID, err := buildNodeID(req.Attributes, METHOD)
	if err != nil {
		return nil, fmt.Errorf("Driver.handleReadCommands: %v", err)
	}
	mid, err := ua.ParseNodeID(methodID)
	if err != nil {
		return nil, fmt.Errorf("Driver.handleReadCommands: %v", err)
	}

	inputMap, ok := req.Attributes[INPUTMAP]
	if ok {
		imElements := inputMap.([]interface{})
		if len(imElements) > 0 {
			inputs = make([]*ua.Variant, len(imElements))
			for i := 0; i < len(imElements); i++ {
				inputs[i] = ua.MustVariant(imElements[i].(string))
			}
		}
	}

	request := &ua.CallMethodRequest{
		ObjectID:       oid,
		MethodID:       mid,
		InputArguments: inputs,
	}

	resp, err := deviceClient.Call(request)
	if err != nil {
		return nil, fmt.Errorf("Driver.handleReadCommands: Method call failed: %s", err)
	}
	if resp.StatusCode != ua.StatusOK {
		return nil, fmt.Errorf("Driver.handleReadCommands: Method status not OK: %v", resp.StatusCode)
	}

	return newResult(req, resp.OutputArguments[0].Value())
}

// HandleWriteCommands passes a slice of CommandRequest struct each representing
// a ResourceOperation for a specific device resource (aka DeviceObject).
// Since the commands are actuation commands, params provide parameters for the individual
// command.
func (d *Driver) HandleWriteCommands(deviceName string, protocols map[string]models.ProtocolProperties,
	reqs []sdkModel.CommandRequest, params []*sdkModel.CommandValue) error {

	d.Logger.Debugf("Driver.HandleWriteCommands: protocols: %v, resource: %v, parameters: %v", protocols, reqs[0].DeviceResourceName, params)
	var err error

	// create device client and open connection
	endpoint, err := fetchEndpoint(protocols)
	if err != nil {
		return err
	}

	ctx := context.Background()
	client := opcua.NewClient(endpoint, opcua.SecurityMode(ua.MessageSecurityModeNone))
	if err := client.Connect(ctx); err != nil {
		d.Logger.Warnf("Driver.HandleWriteCommands: Failed to create OPCUA client, %s", err)
		return err
	}
	defer client.Close()

	for _, req := range reqs {
		// handle every reqs every params
		for _, param := range params {
			err := d.handleWriteCommandRequest(client, req, param)
			if err != nil {
				d.Logger.Errorf("Driver.HandleWriteCommands: Handle write commands failed: %v", err)
				return err
			}
		}

	}

	return err
}

func (d *Driver) handleWriteCommandRequest(deviceClient *opcua.Client, req sdkModel.CommandRequest,
	param *sdkModel.CommandValue) error {
	nodeID, err := buildNodeID(req.Attributes, SYMBOL)
	if err != nil {
		return fmt.Errorf("Driver.handleWriteCommands: %v", err)
	}

	// get NewNodeID
	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return fmt.Errorf("Driver.handleWriteCommands: Invalid node id=%s", nodeID)
	}

	value, err := newCommandValue(req.Type, param)
	if err != nil {
		return err
	}
	v, err := ua.NewVariant(value)

	if err != nil {
		return fmt.Errorf("Driver.handleWriteCommands: invalid value: %v", err)
	}

	request := &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{
			{
				NodeID:      id,
				AttributeID: ua.AttributeIDValue,
				Value: &ua.DataValue{
					EncodingMask: ua.DataValueValue, // encoding mask
					Value:        v,
				},
			},
		},
	}

	resp, err := deviceClient.Write(request)
	if err != nil {
		d.Logger.Errorf("Driver.handleWriteCommands: Write value %v failed: %s", v, err)
		return err
	}
	d.Logger.Infof("Driver.handleWriteCommands: write sucessfully, %v", resp.Results[0])
	return nil
}

// Stop the protocol-specific DS code to shutdown gracefully, or
// if the force parameter is 'true', immediately. The driver is responsible
// for closing any in-use channels, including the channel used to send async
// readings (if supported).
func (d *Driver) Stop(force bool) error {
	d.Logger.Warn("Driver's Stop function not implemented")
	return nil
}

func buildNodeID(attrs map[string]interface{}, sKey string) (string, error) {
	if _, ok := attrs[NAMESPACE]; !ok {
		return "", fmt.Errorf("Attribute %s does not exist", NAMESPACE)
	}
	if _, ok := attrs[sKey]; !ok {
		return "", fmt.Errorf("Attribute %s does not exist", sKey)
	}

	return fmt.Sprintf("ns=%s;s=%s", attrs[NAMESPACE].(string), attrs[sKey].(string)), nil
}

func newResult(req sdkModel.CommandRequest, reading interface{}) (*sdkModel.CommandValue, error) {
	var result = &sdkModel.CommandValue{}
	var err error
	castError := "fail to parse %v reading, %v"

	if !checkValueInRange(req.Type, reading) {
		err = fmt.Errorf("parse reading fail. Reading %v is out of the value type(%v)'s range", reading, req.Type)
		return result, err
	}

	var val interface{}

	switch req.Type {
	case common.ValueTypeBool:
		val, err = cast.ToBoolE(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeString:
		val, err = cast.ToStringE(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeUint8:
		val, err = cast.ToUint8E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeUint16:
		val, err = cast.ToUint16E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeUint32:
		val, err = cast.ToUint32E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeUint64:
		val, err = cast.ToUint64E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeInt8:
		val, err = cast.ToInt8E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeInt16:
		val, err = cast.ToInt16E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeInt32:
		val, err = cast.ToInt32E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeInt64:
		val, err = cast.ToInt64E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeFloat32:
		val, err = cast.ToFloat32E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeFloat64:
		val, err = cast.ToFloat64E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	default:
		err = fmt.Errorf("return result fail, none supported value type: %v", req.Type)
	}

	result, err = sdkModel.NewCommandValue(req.DeviceResourceName, req.Type, val)
	if err != nil {
		return nil, err
	}
	result.Origin = time.Now().UnixNano() / int64(time.Millisecond)

	return result, err
}

func newCommandValue(valueType string, param *sdkModel.CommandValue) (interface{}, error) {
	var commandValue interface{}
	var err error
	switch valueType {
	case common.ValueTypeBool:
		commandValue, err = param.BoolValue()
	case common.ValueTypeString:
		commandValue, err = param.StringValue()
	case common.ValueTypeUint8:
		commandValue, err = param.Uint8Value()
	case common.ValueTypeUint16:
		commandValue, err = param.Uint16Value()
	case common.ValueTypeUint32:
		commandValue, err = param.Uint32Value()
	case common.ValueTypeUint64:
		commandValue, err = param.Uint64Value()
	case common.ValueTypeInt8:
		commandValue, err = param.Int8Value()
	case common.ValueTypeInt16:
		commandValue, err = param.Int16Value()
	case common.ValueTypeInt32:
		commandValue, err = param.Int32Value()
	case common.ValueTypeInt64:
		commandValue, err = param.Int64Value()
	case common.ValueTypeFloat32:
		commandValue, err = param.Float32Value()
	case common.ValueTypeFloat64:
		commandValue, err = param.Float64Value()
	default:
		err = fmt.Errorf("fail to convert param, none supported value type: %v", valueType)
	}

	return commandValue, err
}
