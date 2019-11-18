//
package driver

import (
	"context"
	"fmt"
	sdk "github.com/edgexfoundry/device-sdk-go"
	sdkModel "github.com/edgexfoundry/device-sdk-go/pkg/models"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/monitor"
	"github.com/gopcua/opcua/ua"
	"time"
)

var service *sdk.Service

func startIncomingListening() error {
	driver.Logger.Info("[Incoming listener] Start incoming data listening. ")
	service = sdk.RunningService()
	errChan := make(chan error)
	for _, device := range driver.Config.Devices {
		go func() {
			errChan <- subscribeEachDevice(device)
		}()
	}
	return <- errChan
}

func subscribeEachDevice(deviceInfo DevicesInfo) error {
	device, err := service.GetDeviceByName(deviceInfo.DeviceName)
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
	ep := opcua.SelectEndpoint(endpoints, deviceInfo.Policy, ua.MessageSecurityModeFromString(deviceInfo.Mode))
	// replace
	ep.EndpointURL = connectionInfo.Endpoint
	if ep == nil {
		return fmt.Errorf("failed to find suitable endpoint")
	}

	opts := []opcua.Option{
		opcua.SecurityPolicy(deviceInfo.Policy),
		opcua.SecurityModeString(deviceInfo.Mode),
		opcua.CertificateFile(deviceInfo.CertFile),
		opcua.PrivateKeyFile(deviceInfo.KeyFile),
		opcua.AuthAnonymous(),
		opcua.SecurityFromEndpoint(ep, ua.UserTokenTypeAnonymous),
	}

	client := opcua.NewClient(ep.EndpointURL, opts...)
	if err := client.Connect(ctx); err != nil {
		return err
	}
	defer client.Close()


	m, err := monitor.NewNodeMonitor(client)
	if err != nil {
		return err
	}

	m.SetErrorHandler(func(_ *opcua.Client, sub *monitor.Subscription, err error) {
		driver.Logger.Error(fmt.Sprintf("error: sub=%d err=%s", sub.SubscriptionID(), err.Error()))
	})

	// start channel-based subscription
	errchan := make(chan error)
	go startChanSub(ctx, m, deviceInfo.DeviceName, errchan,0, deviceInfo.NodeIds...)
	<-ctx.Done()
	return <- errchan
}

func startChanSub(ctx context.Context, m *monitor.NodeMonitor, deviceName string, errchan chan error, lag time.Duration, nodes ...string) {
	ch := make(chan *monitor.DataChangeMessage, 16)
	sub, err := m.ChanSubscribe(ctx, ch, nodes...)

	if err != nil {
		errchan <- err
	}

	defer cleanup(sub)

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			if msg.Error != nil {
				errchan <- fmt.Errorf(fmt.Sprintf("[channel ] sub=%d error=%s", sub.SubscriptionID(), msg.Error))
			} else {
				onIncomingDataReceived(msg.Value.Value(), msg.NodeID.String(), deviceName)
				driver.Logger.Debug(fmt.Sprintf("[channel ] sub=%d node=%s value=%v", sub.SubscriptionID(), msg.NodeID, msg.Value.Value()))
			}
			time.Sleep(lag)
		}
	}
}

func cleanup(sub *monitor.Subscription) {
	driver.Logger.Debug(fmt.Sprintf("stats: sub=%d delivered=%d dropped=%d", sub.SubscriptionID(), sub.Delivered(), sub.Dropped()))
	sub.Unsubscribe()
}
func onIncomingDataReceived(data interface{}, deviceResource string, deviceName string) {
	deviceObject, ok := service.DeviceResource(deviceName, deviceResource, "get")
	if !ok {
		driver.Logger.Warn(fmt.Sprintf("[Incoming listener] Incoming reading ignored. No DeviceObject found: name=%v deviceResource=%v value=%v", deviceName, deviceResource, data))
		return
	}

	req := sdkModel.CommandRequest{
		DeviceResourceName: deviceResource,
		Type:               sdkModel.ParseValueType(deviceObject.Properties.Value.Type),
	}

	result, err := newResult(req, data)
	if err != nil {
		driver.Logger.Warn(fmt.Sprintf("[Incoming listener] Incoming reading ignored. name=%v deviceResource=%v value=%v", deviceName, deviceResource, data))
		return
	}

	asyncValues := &sdkModel.AsyncValues{
		DeviceName:    deviceName,
		CommandValues: []*sdkModel.CommandValue{result},
	}
	driver.Logger.Info(fmt.Sprintf("[Incoming listener] Incoming reading received: name=%v deviceResource=%v value=%v", deviceName, deviceResource, data))
	driver.AsyncCh <- asyncValues
}
