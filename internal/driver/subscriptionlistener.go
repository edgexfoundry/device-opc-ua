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
	"time"

	sdkModels "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

type ResourceDetail struct {
	deviceName string
	command    *CommandInfo
}

func (d *Driver) startSubscriptionListener(devices []models.Device) error {

	if len(devices) == 0 {
		d.Logger.Infof("[Incoming listener] No devices available for create subscriptions")
		return nil
	}

	for _, device := range devices {
		deviceName := device.Name

		info, err := createConnectionInfo(device.Protocols)
		if err != nil {
			d.Logger.Warnf("[Incoming listener] Fail to create connectionInfo from device protocols for device '%s', %s", deviceName, err)
			continue
		}

		profileName := device.ProfileName
		deviceProfile, err := d.sdkService.GetProfileByName(profileName)
		if err != nil {
			d.Logger.Warnf("[Incoming listener] Failed to get profile by name %s, %s", profileName, err)
			continue
		}
		resources := filterResources(deviceProfile.DeviceResources)
		if len(resources) == 0 {
			d.Logger.Warnf("[Incoming listener] No resources defined in device profile '%s' for device '%s'", profileName, deviceName)
			continue
		}

		go func() {
			// Create a cancelable context for Writable configuration
			ctxBg := context.Background()
			ctx, cancel := context.WithCancel(ctxBg)
			d.ctxCancel[deviceName] = cancel

			client, err := createUaConnection(info, &d.serviceConfig.OPCUAServer)

			if err != nil {
				panic(err)
			}

			notifyCh := make(chan *opcua.PublishNotificationData)
			sub, err := client.Subscribe(ctx, &opcua.SubscriptionParameters{
				Interval: time.Duration(500) * time.Millisecond,
			}, notifyCh)
			if err != nil {
				panic(err)
			}
			defer func(sub *opcua.Subscription, ctx context.Context) {
				_ = sub.Cancel(ctx)
			}(sub, ctx)

			if err := d.configureMonitoredItems(sub, deviceName, resources); err != nil {
				panic(err)
			}

			// go sub.Run(ctx) // start Publish loop

			// read from subscription's notification channel until ctx is cancelled
			for {
				select {
				// context return
				case <-ctx.Done():
					return
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
		}()

	}

	return nil
}

func filterResources(resources []models.DeviceResource) (ret []models.DeviceResource) {
	for _, resource := range resources {
		info, err := CreateCommandInfo(resource.Name, resource.Properties.ValueType, resource.Attributes)
		if err != nil {
			driver.Logger.Warnf("[Incoming listener] Unable to create command info for resource %s, %s", resource.Name, err)
			continue
		}
		if info.IsWatchable() {
			ret = append(ret, resource)
		}
	}
	return
}

func (d *Driver) configureMonitoredItems(sub *opcua.Subscription, deviceName string, resources []models.DeviceResource) error {

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, deviceResource := range resources {
		info, err := CreateCommandInfo(deviceResource.Name, deviceResource.Properties.ValueType, deviceResource.Attributes)
		if err != nil {
			return err
		}

		opcuaNodeID := info.nodeId
		id, err := ua.ParseNodeID(opcuaNodeID)
		if err != nil {
			return err
		}

		// arbitrary client handle for the monitoring item
		// 24.04.22 - use hash of deviceName as client handle, array index is not unique
		handle := hash(deviceName)
		// map the client handle, so we know what the value returned represents
		d.resourceMap[handle] = ResourceDetail{
			deviceName: deviceName,
			command:    info,
		}
		miCreateRequest := opcua.NewMonitoredItemCreateRequestWithDefaults(id, ua.AttributeIDValue, handle)
		ctx := context.Background()
		res, err := sub.Monitor(ctx, ua.TimestampsToReturnBoth, miCreateRequest)
		if err != nil || !errors.Is(res.Results[0].StatusCode, ua.StatusOK) {
			return err
		}

		d.Logger.Infof("[Incoming listener] Start incoming data listening for %s.", deviceResource.Name)
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

func (d *Driver) onIncomingDataReceived(data interface{}, detail ResourceDetail) error {
	var (
		deviceName  = detail.deviceName
		commandInfo = detail.command
	)
	reading := data

	result, err := newResult(commandInfo, reading)
	if err != nil {
		d.Logger.Warnf("[Incoming listener] Incoming reading ignored. name=%v deviceResource=%v value=%v", deviceName, commandInfo.resourceName, data)
		return nil
	}

	asyncValues := &sdkModels.AsyncValues{
		DeviceName:    deviceName,
		CommandValues: []*sdkModels.CommandValue{result},
	}

	d.Logger.Infof("[Incoming listener] Incoming reading received: name=%v deviceResource=%v value=%v", deviceName, commandInfo.resourceName, data)

	d.AsyncCh <- asyncValues

	return nil
}
