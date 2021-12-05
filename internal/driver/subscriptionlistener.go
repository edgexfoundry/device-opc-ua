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
	"strings"
	"time"

	"github.com/edgexfoundry/device-opcua-go/internal/config"
	"github.com/edgexfoundry/device-sdk-go/v2/pkg/models"
	"github.com/edgexfoundry/device-sdk-go/v2/pkg/service"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

func (d *Driver) startSubscriptionListener() error {

	var (
		deviceName = d.serviceConfig.OPCUAServer.DeviceName
		policy     = d.serviceConfig.OPCUAServer.Policy
		mode       = d.serviceConfig.OPCUAServer.Mode
		certFile   = d.serviceConfig.OPCUAServer.CertFile
		keyFile    = d.serviceConfig.OPCUAServer.KeyFile
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

	ds := service.RunningService()
	if ds == nil {
		return fmt.Errorf("[Incoming listener] unable to get running device service")
	}

	device, err := ds.GetDeviceByName(deviceName)
	if err != nil {
		return err
	}
	endpoint, err := config.FetchEndpoint(device.Protocols)
	if err != nil {
		return err
	}

	endpoints, err := opcua.GetEndpoints(endpoint)
	if err != nil {
		return err
	}
	ep := opcua.SelectEndpoint(endpoints, policy, ua.MessageSecurityModeFromString(mode))
	if ep == nil {
		return fmt.Errorf("[Incoming listener] Failed to find suitable endpoint")
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

	client := opcua.NewClient(ep.EndpointURL, opts...)
	if err := client.Connect(ctx); err != nil {
		d.Logger.Warnf("[Incoming listener] Failed to connect OPCUA client, %s", err)
		return err
	}
	defer client.Close()

	sub, err := client.Subscribe(
		&opcua.SubscriptionParameters{
			Interval: time.Duration(500) * time.Millisecond,
		}, make(chan *opcua.PublishNotificationData))
	if err != nil {
		return err
	}
	defer sub.Cancel()

	for i, node := range strings.Split(resources, ",") {
		deviceResource, ok := ds.DeviceResource(deviceName, node)
		if !ok {
			return fmt.Errorf("[Incoming listener] Unable to find device resource with name %s", node)
		}

		opcuaNodeID, err := buildNodeID(deviceResource.Attributes, SYMBOL)
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

	go sub.Run(ctx) // start Publish loop

	// read from subscription's notification channel until ctx is cancelled
	for {
		select {
		// context return
		case <-ctx.Done():
			return nil
			// receive Publish Notification Data
		case res := <-sub.Notifs:
			if res.Error != nil {
				d.Logger.Debug(res.Error.Error())
				continue
			}
			switch x := res.Value.(type) {
			// result type: DateChange StatusChange
			case *ua.DataChangeNotification:
				for _, item := range x.MonitoredItems {
					data := item.Value.Value.Value()
					if err := d.onIncomingDataReceived(data, d.resourceMap[item.ClientHandle]); err != nil {
						d.Logger.Errorf("%v", err)
					}
				}
			}
		}
	}
}

func (d *Driver) onIncomingDataReceived(data interface{}, nodeResourceName string) error {
	deviceName := d.serviceConfig.OPCUAServer.DeviceName
	reading := data

	ds := service.RunningService()
	if ds == nil {
		return fmt.Errorf("[Incoming listener] unable to get running device service")
	}

	deviceResource, ok := ds.DeviceResource(deviceName, nodeResourceName)
	if !ok {
		d.Logger.Warnf("[Incoming listener] Incoming reading ignored. No DeviceObject found: name=%v deviceResource=%v value=%v", deviceName, nodeResourceName, data)
		return nil
	}

	req := models.CommandRequest{
		DeviceResourceName: nodeResourceName,
		Type:               deviceResource.Properties.ValueType,
	}

	result, err := newResult(req, reading)
	if err != nil {
		d.Logger.Warnf("[Incoming listener] Incoming reading ignored. name=%v deviceResource=%v value=%v", deviceName, nodeResourceName, data)
		return nil
	}

	asyncValues := &models.AsyncValues{
		DeviceName:    deviceName,
		CommandValues: []*models.CommandValue{result},
	}

	d.Logger.Infof("[Incoming listener] Incoming reading received: name=%v deviceResource=%v value=%v", deviceName, nodeResourceName, data)

	d.AsyncCh <- asyncValues

	return nil
}
