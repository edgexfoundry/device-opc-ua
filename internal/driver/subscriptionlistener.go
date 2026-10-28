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
	"errors"
	"fmt"
	"strings"
	"time"

	sdkModels "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

// listenerStop is a subscription setup error that a new session will not fix.
type listenerStop struct {
	err error
}

func (e *listenerStop) Error() string { return e.err.Error() }

func (e *listenerStop) Unwrap() error { return e.err }

func isPermanentListenerError(err error) bool {
	var target *listenerStop
	return errors.As(err, &target)
}

func (d *Driver) startSubscriptionListener(ctx context.Context) error {

	var (
		deviceName = d.serviceConfig.OPCUAServer.DeviceName
		resources  = d.serviceConfig.OPCUAServer.Writable.Resources
	)

	device, err := d.sdkService.GetDeviceByName(deviceName)
	if err != nil {
		return &listenerStop{err: err}
	}

	// The listener shares the read/write session. endUse releases the in-flight
	// count. Client.Close stays with the device refcount and with Stop.
	client, key, err := d.acquireClient(ctx, deviceName, device.Protocols)
	if err != nil {
		d.Logger.Warnf("[Incoming listener] Failed to connect OPCUA client, %s", err)
		return err
	}
	defer d.endUse(key)

	notifyCh := make(chan *opcua.PublishNotificationData)
	sub, err := client.Subscribe(ctx, &opcua.SubscriptionParameters{
		Interval: time.Duration(500) * time.Millisecond,
	}, notifyCh)
	if err != nil {
		d.dropSession(key)
		return transportFailure(err)
	}
	defer func(sub *opcua.Subscription, ctx context.Context) {
		_ = sub.Cancel(ctx)
	}(sub, ctx)

	if err := d.configureMonitoredItems(sub, resources, deviceName); err != nil {
		if isTransportError(err) {
			d.dropSession(key)
		}
		return err
	}

	// go sub.Run(ctx) // start Publish loop

	// read from subscription's notification channel until ctx is cancelled
	for {
		select {
		// context return
		case <-ctx.Done():
			return nil
			// receive Publish Notification Data
		case res := <-notifyCh:
			if res == nil || res.Error != nil {
				err := errors.New("OPC UA subscription closed")
				if res != nil && res.Error != nil {
					err = res.Error
				}
				d.Logger.Errorf("[Incoming listener] Subscription publish failed: %s", err)
				d.dropSession(key)
				return transportFailure(err)
			}

			switch x := res.Value.(type) {
			// result type: DateChange StatusChange
			case *ua.DataChangeNotification:
				d.handleDataChange(x)

			// case *ua.EventNotificationList:
			// 	for _, item := range x.Events {
			// 		d.Logger.Debug("Event for client handle: %v\n", item.ClientHandle)
			// 		for i, field := range item.EventFields {
			// 			d.Logger.Debug("%v: %v of Type: %T", eventFieldNames[i], field.Value(), field.Value())
			// 		}
			// 	}

			default:
				d.Logger.Debug("what's this publish result? %T", res.Value)
			}
		}
	}
}

func (d *Driver) configureMonitoredItems(sub *opcua.Subscription, resources, deviceName string) error {

	d.mu.Lock()
	defer d.mu.Unlock()

	for i, node := range strings.Split(resources, ",") {
		deviceResource, ok := d.sdkService.DeviceResource(deviceName, node)
		if !ok {
			return &listenerStop{err: fmt.Errorf("[Incoming listener] Unable to find device resource with name %s", node)}
		}

		opcuaNodeID, err := getNodeID(deviceResource.Attributes, NODE)
		if err != nil {
			return &listenerStop{err: err}
		}

		id, err := ua.ParseNodeID(opcuaNodeID)
		if err != nil {
			return &listenerStop{err: err}
		}

		// arbitrary client handle for the monitoring item
		handle := uint32(i + 42) // #nosec G115
		// map the client handle so we know what the value returned represents
		d.resourceMap[handle] = node
		miCreateRequest := opcua.NewMonitoredItemCreateRequestWithDefaults(id, ua.AttributeIDValue, handle)
		ctx := context.Background()
		res, err := sub.Monitor(ctx, ua.TimestampsToReturnBoth, miCreateRequest)
		if err != nil {
			return transportFailure(fmt.Errorf("[Incoming listener] monitor %s: %w", node, err))
		}
		if res == nil || len(res.Results) == 0 || res.Results[0].StatusCode != ua.StatusOK {
			return &listenerStop{err: fmt.Errorf("[Incoming listener] monitor status for %s is not OK", node)}
		}

		d.Logger.Infof("[Incoming listener] Start incoming data listening for %s.", node)
	}

	return nil
}

func (d *Driver) handleDataChange(dcn *ua.DataChangeNotification) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, item := range dcn.MonitoredItems {
		data := item.Value.Value.Value()
		nodeName := d.resourceMap[item.ClientHandle]
		if err := d.onIncomingDataReceived(data, nodeName); err != nil {
			d.Logger.Errorf("%v", err)
		}
	}
}

func (d *Driver) onIncomingDataReceived(data interface{}, nodeResourceName string) error {
	deviceName := d.serviceConfig.OPCUAServer.DeviceName
	reading := data

	deviceResource, ok := d.sdkService.DeviceResource(deviceName, nodeResourceName)
	if !ok {
		d.Logger.Warnf("[Incoming listener] Incoming reading ignored. No DeviceObject found: name=%v deviceResource=%v value=%v", deviceName, nodeResourceName, data)
		return nil
	}

	req := sdkModels.CommandRequest{
		DeviceResourceName: nodeResourceName,
		Type:               deviceResource.Properties.ValueType,
	}

	result, err := newResult(req, reading)
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
