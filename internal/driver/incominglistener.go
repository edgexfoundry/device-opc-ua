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

func startIncomingListening() error {

	var (
		devicename = driver.serviceConfig.OPCUAServer.DeviceName
		policy     = driver.serviceConfig.OPCUAServer.Policy
		mode       = driver.serviceConfig.OPCUAServer.Mode
		certFile   = driver.serviceConfig.OPCUAServer.CertFile
		keyFile    = driver.serviceConfig.OPCUAServer.KeyFile
		nodeID     = driver.serviceConfig.OPCUAServer.NodeID
	)

	service := service.RunningService()
	device, err := service.GetDeviceByName(devicename)
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
			Interval: 500 * time.Millisecond,
		}, nil)
	if err != nil {
		return err
	}
	defer sub.Cancel()

	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return err
	}

	// arbitrary client handle for the monitoring item
	handle := uint32(1) // arbitrary client id
	miCreateRequest := opcua.NewMonitoredItemCreateRequestWithDefaults(id, ua.AttributeIDValue, handle)
	res, err := sub.Monitor(ua.TimestampsToReturnBoth, miCreateRequest)
	if err != nil || res.Results[0].StatusCode != ua.StatusOK {
		return err
	}

	driver.Logger.Info("[Incoming listener] Start incoming data listening. ")

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
				driver.Logger.Debug(fmt.Sprintf("%s", res.Error))
				continue
			}
			switch x := res.Value.(type) {
			// result type: DateChange StatusChange
			case *ua.DataChangeNotification:
				for _, item := range x.MonitoredItems {
					data := item.Value.Value.Value
					onIncomingDataReceived(data)
				}
			}
		}
	}
}

func onIncomingDataReceived(data interface{}) {
	deviceName := driver.serviceConfig.OPCUAServer.DeviceName
	cmd := driver.serviceConfig.OPCUAServer.NodeID
	reading := data

	service := service.RunningService()

	deviceObject, ok := service.DeviceResource(deviceName, cmd)
	if !ok {
		driver.Logger.Warn(fmt.Sprintf("[Incoming listener] Incoming reading ignored. No DeviceObject found: name=%v deviceResource=%v value=%v", deviceName, cmd, data))
		return
	}

	req := models.CommandRequest{
		DeviceResourceName: cmd,
		Type:               deviceObject.Properties.ValueType,
	}

	result, err := newResult(req, reading)

	if err != nil {
		driver.Logger.Warn(fmt.Sprintf("[Incoming listener] Incoming reading ignored. name=%v deviceResource=%v value=%v", deviceName, cmd, data))
		return
	}

	asyncValues := &models.AsyncValues{
		DeviceName:    deviceName,
		CommandValues: []*models.CommandValue{result},
	}

	driver.Logger.Info(fmt.Sprintf("[Incoming listener] Incoming reading received: name=%v deviceResource=%v value=%v", deviceName, cmd, data))

	driver.AsyncCh <- asyncValues

}
