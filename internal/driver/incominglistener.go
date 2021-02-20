//
package driver

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/edgexfoundry/device-sdk-go"
	sdkModel "github.com/edgexfoundry/device-sdk-go/pkg/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

var service *sdk.Service

func startIncomingListening() error {

	var (
		devicename = driver.Config.DeviceName
		policy     = driver.Config.Policy
		mode       = driver.Config.Mode
		certFile   = driver.Config.CertFile
		keyFile    = driver.Config.KeyFile
		nodeID     = driver.Config.NodeID
	)

	service = sdk.RunningService()
	device, err := service.GetDeviceByName(devicename)
	if err != nil {
		return err
	}
	connectionInfo, err := CreateConnectionInfo(device.Protocols)
	if err != nil {
		return err
	}
	ctx := context.Background()

	endpoints, err := opcua.GetEndpoints(connectionInfo.Endpoint)
	if err != nil {
		return err
	}
	ep := opcua.SelectEndpoint(endpoints, policy, ua.MessageSecurityModeFromString(mode))
	// replace Burning-Laptop with ip adress
	ep.EndpointURL = connectionInfo.Endpoint
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

	sub, err := client.Subscribe(&opcua.SubscriptionParameters{
		Interval: 500 * time.Millisecond,
	})
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
	deviceName := driver.Config.DeviceName
	cmd := driver.Config.NodeID
	reading := data

	deviceObject, ok := service.DeviceResource(deviceName, cmd, "get")
	if !ok {
		driver.Logger.Warn(fmt.Sprintf("[Incoming listener] Incoming reading ignored. No DeviceObject found: name=%v deviceResource=%v value=%v", deviceName, cmd, data))
		return
	}

	req := sdkModel.CommandRequest{
		DeviceResourceName: cmd,
		Type:               sdkModel.ParseValueType(deviceObject.Properties.Value.Type),
	}

	result, err := newResult(req, reading)

	if err != nil {
		driver.Logger.Warn(fmt.Sprintf("[Incoming listener] Incoming reading ignored. name=%v deviceResource=%v value=%v", deviceName, cmd, data))
		return
	}

	asyncValues := &sdkModel.AsyncValues{
		DeviceName:    deviceName,
		CommandValues: []*sdkModel.CommandValue{result},
	}

	driver.Logger.Info(fmt.Sprintf("[Incoming listener] Incoming reading received: name=%v deviceResource=%v value=%v", deviceName, cmd, data))

	driver.AsyncCh <- asyncValues

}
