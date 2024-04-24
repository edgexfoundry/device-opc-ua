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
	"errors"
	"fmt"
	sdkModel "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

// HandleReadCommands triggers a protocol Read operation for the specified device.
func (d *Driver) HandleReadCommands(deviceName string, protocols map[string]models.ProtocolProperties,
	reqs []sdkModel.CommandRequest) ([]*sdkModel.CommandValue, error) {

	var commandInfos = make([]*CommandInfo, len(reqs))
	// create and validate command infos before
	for i, req := range reqs {
		commandInfo, err := CreateCommandInfo(req.DeviceResourceName, req.Type, req.Attributes)
		if err != nil {
			fmt.Printf("Driver.HandleReadCommands: Create command info failed: %v\n", err)
			return nil, err
		}
		commandInfos[i] = commandInfo
	}
	connectionInfo, err := createConnectionInfo(protocols)
	if err != nil {
		return nil, err
	}
	connection, err := d.uaConnectionPool.GetConnection(deviceName, connectionInfo)
	if err != nil {
		return nil, err
	}
	defer connection.Close()
	return d.processReadCommands(connection.GetClient(), commandInfos)
}

func (d *Driver) processReadCommands(client *opcua.Client, reqs []*CommandInfo) ([]*sdkModel.CommandValue, error) {
	var responses = make([]*sdkModel.CommandValue, len(reqs))
	for i, req := range reqs {
		// handle every reqs
		res, err := d.handleReadCommandRequest(client, req)
		if err != nil {
			d.Logger.Errorf("Driver.HandleReadCommands: Handle read commands failed: %v", err)
			return responses, err
		}
		responses[i] = res
	}

	return responses, nil
}

func (d *Driver) handleReadCommandRequest(deviceClient *opcua.Client, info *CommandInfo) (*sdkModel.CommandValue, error) {
	var result = &sdkModel.CommandValue{}
	var err error

	isMethod := info.isMethodCall()

	if isMethod {
		result, err = makeMethodCall(deviceClient, info)
		d.Logger.Infof("Method command finished: %v", result)
	} else {
		result, err = makeReadRequest(deviceClient, info)
		d.Logger.Infof("Read command finished: %v", result)
	}

	return result, err
}

func makeReadRequest(deviceClient *opcua.Client, req *CommandInfo) (*sdkModel.CommandValue, error) {
	nodeID := req.nodeId

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

	ctx := context.Background()
	resp, err := deviceClient.Read(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("Driver.handleReadCommands: Read failed: %s", err)
	}
	if !errors.Is(resp.Results[0].Status, ua.StatusOK) {
		return nil, fmt.Errorf("Driver.handleReadCommands: Status not OK: %v", resp.Results[0].Status)
	}

	// make new result
	reading := resp.Results[0].Value.Value()
	return newResult(req, reading)
}

func makeMethodCall(deviceClient *opcua.Client, req *CommandInfo) (*sdkModel.CommandValue, error) {
	var inputs []*ua.Variant

	objectID := req.objectId
	oid, err := ua.ParseNodeID(objectID)
	if err != nil {
		return nil, fmt.Errorf("Driver.handleReadCommands: %v", err)
	}

	methodID := req.methodId
	mid, err := ua.ParseNodeID(methodID)
	if err != nil {
		return nil, fmt.Errorf("Driver.handleReadCommands: %v", err)
	}

	inputMap := req.inputMap
	if len(inputMap) > 0 {
		inputs = make([]*ua.Variant, len(inputMap))
		for i, el := range inputMap {
			inputs[i] = ua.MustVariant(el)
		}
	}

	request := &ua.CallMethodRequest{
		ObjectID:       oid,
		MethodID:       mid,
		InputArguments: inputs,
	}

	ctx := context.Background()
	resp, err := deviceClient.Call(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("Driver.handleReadCommands: Method call failed: %s", err)
	}
	if !errors.Is(resp.StatusCode, ua.StatusOK) {
		return nil, fmt.Errorf("Driver.handleReadCommands: Method status not OK: %v", resp.StatusCode)
	}

	return newResult(req, resp.OutputArguments[0].Value())
}
