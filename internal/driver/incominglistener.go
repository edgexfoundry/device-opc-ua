package driver

import (
	"context"
	"fmt"
	"time"

	"github.com/edgexfoundry/device-sdk-go/v2/pkg/models"
	"github.com/edgexfoundry/device-sdk-go/v2/pkg/service"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

func (d *Driver) startIncomingListening() error {

	var (
		deviceName = d.serviceConfig.OPCUAServer.DeviceName
		policy     = d.serviceConfig.OPCUAServer.Policy
		mode       = d.serviceConfig.OPCUAServer.Mode
		certFile   = d.serviceConfig.OPCUAServer.CertFile
		keyFile    = d.serviceConfig.OPCUAServer.KeyFile
		nodeID     = d.serviceConfig.OPCUAServer.NodeID
	)

	ds := service.RunningService()
	device, err := ds.GetDeviceByName(deviceName)
	if err != nil {
		return err
	}
	endpoint, err := fetchEndpoint(device.Protocols)
	if err != nil {
		return err
	}
	ctx := context.Background()

	endpoints, err := opcua.GetEndpoints(endpoint)
	if err != nil {
		return err
	}
	ep := opcua.SelectEndpoint(endpoints, policy, ua.MessageSecurityModeFromString(mode))
	ep.EndpointURL = endpoint
	if ep == nil {
		return fmt.Errorf("Failed to find suitable endpoint")
	}

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

	deviceResource, ok := ds.DeviceResource(deviceName, nodeID)
	if !ok {
		return fmt.Errorf("[Incoming listener] Unable to find device resource with name %s", nodeID)
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
	handle := uint32(42) // arbitrary client id
	miCreateRequest := opcua.NewMonitoredItemCreateRequestWithDefaults(id, ua.AttributeIDValue, handle)
	res, err := sub.Monitor(ua.TimestampsToReturnBoth, miCreateRequest)
	if err != nil || res.Results[0].StatusCode != ua.StatusOK {
		return err
	}

	d.Logger.Info("[Incoming listener] Start incoming data listening.")

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
				d.Logger.Debugf("%s", res.Error)
				continue
			}
			switch x := res.Value.(type) {
			// result type: DateChange StatusChange
			case *ua.DataChangeNotification:
				for _, item := range x.MonitoredItems {
					data := item.Value.Value.Value()
					d.onIncomingDataReceived(data)
				}
			}
		}
	}
}

func (d *Driver) onIncomingDataReceived(data interface{}) {
	deviceName := d.serviceConfig.OPCUAServer.DeviceName
	nodeResourceName := d.serviceConfig.OPCUAServer.NodeID
	reading := data

	ds := service.RunningService()

	deviceResource, ok := ds.DeviceResource(deviceName, nodeResourceName)
	if !ok {
		d.Logger.Warnf("[Incoming listener] Incoming reading ignored. No DeviceObject found: name=%v deviceResource=%v value=%v", deviceName, nodeResourceName, data)
		return
	}

	req := models.CommandRequest{
		DeviceResourceName: nodeResourceName,
		Type:               deviceResource.Properties.ValueType,
	}

	result, err := newResult(req, reading)
	if err != nil {
		d.Logger.Warnf("[Incoming listener] Incoming reading ignored. name=%v deviceResource=%v value=%v", deviceName, nodeResourceName, data)
		return
	}

	asyncValues := &models.AsyncValues{
		DeviceName:    deviceName,
		CommandValues: []*models.CommandValue{result},
	}

	d.Logger.Infof("[Incoming listener] Incoming reading received: name=%v deviceResource=%v value=%v", deviceName, nodeResourceName, data)

	d.AsyncCh <- asyncValues

}
