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
	"strings"
	"time"

	sdkModels "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

func (d *Driver) startSubscriptionListener() error {

	var (
		deviceName = d.serviceConfig.OPCUAServer.DeviceName
		resources  = d.serviceConfig.OPCUAServer.Writable.Resources
	)

	// No need to start a subscription if there are no resources to monitor
	if len(resources) == 0 {
		d.Logger.Info("[Incoming listener] No resources defined to generate subscriptions.")
		return nil
	}

	// Create a cancelable context for Writable configuration
	ctxBg := context.Background()
	ctx, cancel := context.WithCancel(ctxBg)
	d.ctxCancel = cancel

	device, err := d.sdkService.GetDeviceByName(deviceName)
	if err != nil {
		return err
	}

	client, err := d.getClient(device)
	if err != nil {
		return err
	}

	if err := client.Connect(ctx); err != nil {
		d.Logger.Warnf("[Incoming listener] Failed to connect OPCUA client, %s", err)
		return err
	}
	defer func(client *opcua.Client, ctx context.Context) {
		_ = client.Close(ctx)
	}(client, ctx)

	notifyCh := make(chan *opcua.PublishNotificationData)
	sub, err := client.Subscribe(ctx, &opcua.SubscriptionParameters{
		Interval: time.Duration(500) * time.Millisecond,
	}, notifyCh)
	if err != nil {
		return err
	}
	defer func(sub *opcua.Subscription, ctx context.Context) {
		_ = sub.Cancel(ctx)
	}(sub, ctx)

	if err := d.configureMonitoredItems(sub, resources, deviceName); err != nil {
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
			if res.Error != nil {
				d.Logger.Debug(res.Error.Error())
				continue
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

func (d *Driver) getClient(device models.Device) (*opcua.Client, error) {
	var (
		policy   = d.serviceConfig.OPCUAServer.Policy
		mode     = d.serviceConfig.OPCUAServer.Mode
		certFile = d.serviceConfig.OPCUAServer.CertFile
		keyFile  = d.serviceConfig.OPCUAServer.KeyFile
	)

	endpoint, xerr := FetchEndpoint(device.Protocols)
	if xerr != nil {
		return nil, xerr
	}

	ctx := context.Background()
	endpoints, err := opcua.GetEndpoints(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	ep, err := opcua.SelectEndpoint(endpoints, policy, ua.MessageSecurityModeFromString(mode))
	if err != nil {
		return nil, err
	}
	if ep == nil {
		return nil, fmt.Errorf("[Incoming listener] Failed to find suitable endpoint")
	}
	ep.EndpointURL = endpoint

	opts := []opcua.Option{
		opcua.SecurityPolicy(policy),
		opcua.SecurityModeString(mode),
		opcua.CertificateFile(certFile),
		opcua.PrivateKeyFile(keyFile),
		opcua.AuthAnonymous(),
		opcua.SecurityFromEndpoint(ep, ua.UserTokenTypeAnonymous),
	}

	return opcua.NewClient(ep.EndpointURL, opts...)
}

func (d *Driver) configureMonitoredItems(sub *opcua.Subscription, resources, deviceName string) error {

	d.mu.Lock()
	defer d.mu.Unlock()

	for i, node := range strings.Split(resources, ",") {
		deviceResource, ok := d.sdkService.DeviceResource(deviceName, node)
		if !ok {
			return fmt.Errorf("[Incoming listener] Unable to find device resource with name %s", node)
		}

		opcuaNodeID, err := getNodeID(deviceResource.Attributes, NODE)
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
		ctx := context.Background()
		res, err := sub.Monitor(ctx, ua.TimestampsToReturnBoth, miCreateRequest)
		if err != nil || res.Results[0].StatusCode != ua.StatusOK {
			return err
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
