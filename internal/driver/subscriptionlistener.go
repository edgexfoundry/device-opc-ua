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
	sdkModels "github.com/edgexfoundry/device-sdk-go/v2/pkg/models"
	"github.com/edgexfoundry/device-sdk-go/v2/pkg/service"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
	"strings"
	"time"
)

var LastClientState opcua.ConnState
var ActualClientState opcua.ConnState
var basicErrorMessage = "[Incoming listener] unable to get running device service"

func (d *Driver) StartSubscriptionListener() error {

	var (
		deviceName = d.ServiceConfig.OPCUAServer.DeviceName
		resources  = d.ServiceConfig.OPCUAServer.Writable.Resources
	)

	// No need to start a subscription if there are no resources to monitor
	if len(resources) == 0 {
		d.Logger.Info("[Incoming listener] No resources defined to generate subscriptions.")
		return nil
	}

	// Create a cancelable context for Writable configuration
	ctxBg := context.Background()
	ctx, cancel := context.WithCancel(ctxBg)
	d.CtxCancel = cancel

	ds := service.RunningService()
	if ds == nil {
		return fmt.Errorf(basicErrorMessage)
	}

	device, err := ds.GetDeviceByName(deviceName)
	if err != nil {
		return err
	}

	client, err := d.GetClient(device)
	if err != nil {
		return err
	}

	if err = client.Connect(ctx); err != nil {
		d.Logger.Warnf(basicErrorMessage, "%s", err)
		return err
	}
	defer CloseClientConnection(d, client)

	notifyCh := make(chan *opcua.PublishNotificationData)

	sub, err := client.SubscribeWithContext(ctx, &opcua.SubscriptionParameters{
		Interval: time.Duration(d.ServiceConfig.OPCUAServer.SubscriptionInterval) * time.Millisecond,
	}, notifyCh)
	if err != nil {
		return err
	}
	defer CancelSubscription(d, sub, ctx)

	// begin continuous client state check
	go InitCheckClientState(d, client)

	if err = d.configureMonitoredItems(sub, resources, deviceName); err != nil {
		return err
	}

	// read from subscription's notification channel until ctx is cancelled
	for {
		select {
		// context return
		case <-ctx.Done():
			return nil
			// receive Publish Notification Data
		case res := <-notifyCh:
			if res.Error != nil {
				d.Logger.Debug(res.Error.Error())
				continue
			}
			switch changeNotification := res.Value.(type) {
			// result type: DateChange StatusChange
			case *ua.DataChangeNotification:
				d.HandleDataChange(changeNotification)
			}
		}
	}
}

// ClientState interface gives us possibility to mock opcua client functions returning a desired state.
type ClientState interface {
	State() opcua.ConnState
}

// ClientCloser interface gives us possibility to mock opcua client functions for closing a client in tests.
type ClientCloser interface {
	Close() error
}

// SubscriptionCanceller interface gives us possibility to mock opcua client functions for cling a client in tests.
type SubscriptionCanceller interface {
	Cancel(ctx context.Context) error
}

// CloseClientConnection tries to close the client connection
func CloseClientConnection(d *Driver, client ClientCloser) error {
	err := client.Close()
	if err != nil {
		d.Logger.Warnf("[Incoming listener] Failed to close OPCUA client connection., %s", err)
	}
	return err
}

// CancelSubscription cancel the subscription
func CancelSubscription(d *Driver, cancel SubscriptionCanceller, ctx context.Context) error {
	err := cancel.Cancel(ctx)
	if err != nil {
		d.Logger.Warnf("[Incoming listener] Failed to cancel subscription., %s", err)
	}
	return err
}

// InitCheckClientState Periodically checks the client state for connection issues.
func InitCheckClientState(d *Driver, client ClientState) {

	// set to default values to avoid errors when client is nil
	LastClientState = opcua.Closed
	ActualClientState = opcua.Closed
	for {
		// separated into a function for better testability
		HandleCurrentClientState(d, client)
		// use configured interval
		time.Sleep(time.Duration(d.ServiceConfig.OPCUAServer.ConnRetryWaitTime) * time.Second)
	}
}

func HandleCurrentClientState(d *Driver, client ClientState) {
	if client != nil {
		ActualClientState = client.State()
		if (LastClientState == opcua.Connected || ActualClientState == opcua.Disconnected) && LastClientState != ActualClientState {
			// if you are coming from connected (last state) then log warning
			d.Logger.Warnf("opc ua client is in connection state: Disconnected")
		} else if ActualClientState == opcua.Connected && LastClientState != ActualClientState {
			// if you are in disconnected (actual state) then log info
			d.Logger.Infof("opc ua client is in connection state: Connected")
		} else if ActualClientState == opcua.Reconnecting && ActualClientState == LastClientState {
			// if you actual and last state are reconnecting, inform the user that the reconnect is still being tried.
			d.Logger.Infof("opc ua client is in connection state: Reconnecting")
		}
		LastClientState = ActualClientState
	} else {
		d.Logger.Warnf("opc ua client is null. Attempting to reconnect.")
	}
}

func (d *Driver) GetClient(device models.Device) (*opcua.Client, error) {
	opts, err := d.CreateClientOptions()
	if err != nil {
		d.Logger.Warnf("Driver.getClient: Failed to create OPCUA client options, %s", err)
		return nil, err
	}

	return opcua.NewClient(d.ServiceConfig.OPCUAServer.Endpoint, opts...), nil
}

func (d *Driver) configureMonitoredItems(sub *opcua.Subscription, resources, deviceName string) error {
	d.Logger.Infof("[Incoming listener] Start configuring for resources.", resources)
	ds := service.RunningService()
	if ds == nil {
		return fmt.Errorf(basicErrorMessage)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for i, node := range strings.Split(resources, ",") {
		deviceResource, ok := ds.DeviceResource(deviceName, node)
		if !ok {
			return fmt.Errorf("[Incoming listener] Unable to find device resource with name %s", node)
		}

		opcuaNodeID, err := GetNodeID(deviceResource.Attributes, NODE)
		if err != nil {
			return err
		}

		id, err := ua.ParseNodeID(opcuaNodeID)
		if err != nil {
			return err
		}

		// arbitrary client handle for the monitoring item
		handle := uint32(i + 42)
		// map the client handle so we know what the value returned represents
		d.resourceMap[handle] = node
		miCreateRequest := opcua.NewMonitoredItemCreateRequestWithDefaults(id, ua.AttributeIDValue, handle)
		res, err := sub.Monitor(ua.TimestampsToReturnBoth, miCreateRequest)
		if err != nil || res.Results[0].StatusCode != ua.StatusOK {
			return err
		}

		d.Logger.Infof("[Incoming listener] Start incoming data listening for %s.", node)
	}

	return nil
}

func (d *Driver) HandleDataChange(dcn *ua.DataChangeNotification) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, item := range dcn.MonitoredItems {
		data := item.Value.Value.Value()
		value := item.Value.Value
		nodeName := d.resourceMap[item.ClientHandle]
		if err := d.OnIncomingDataReceived(data, nodeName, value); err != nil {
			d.Logger.Errorf("%v", err)
		}
	}
}

func (d *Driver) OnIncomingDataReceived(data interface{}, nodeResourceName string, value *ua.Variant) error {
	deviceName := d.ServiceConfig.OPCUAServer.DeviceName
	reading := data

	ds := service.RunningService()
	if ds == nil {
		return fmt.Errorf(basicErrorMessage)
	}

	deviceResource, ok := ds.DeviceResource(deviceName, nodeResourceName)
	if !ok {
		d.Logger.Warnf("[Incoming listener] Incoming reading ignored. No DeviceObject found: name=%v deviceResource=%v value=%v", deviceName, nodeResourceName, data)
		return nil
	}

	req := sdkModels.CommandRequest{
		DeviceResourceName: nodeResourceName,
		Type:               deviceResource.Properties.ValueType,
	}

	result, err := NewResult(req, reading)

	sourceTimestamp := ExtractSourceTimestamp(value.DataValue())
	result.Tags["source timestamp"] = sourceTimestamp.String()

	if err != nil {
		d.Logger.Warnf("[Incoming listener] Incoming reading ignored. name=%v deviceResource=%v value=%v", deviceName, nodeResourceName, data)
		return nil
	}

	asyncValues := &sdkModels.AsyncValues{
		DeviceName:    deviceName,
		CommandValues: []*sdkModels.CommandValue{result},
	}

	d.Logger.Infof("[Incoming listener] Incoming reading received: name=%v deviceResource=%v value=%v", deviceName, nodeResourceName, data)

	d.AsyncCh <- asyncValues

	return nil
}
