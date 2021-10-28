package driver

import (
	"testing"

	"github.com/edgexfoundry/device-opcua-go/internal/config"
)

func Test_startSubscriptionListener(t *testing.T) {
	t.Run("create context and exit", func(t *testing.T) {
		d := NewProtocolDriver().(*Driver)
		d.serviceConfig = &config.ServiceConfig{}
		d.serviceConfig.OPCUAServer.Writable.Resources = "IntVarTest1"

		err := d.startSubscriptionListener()
		if err == nil {
			t.Error("expected err to exist in test environment")
		}

		d.ctxCancel()
	})
}

func Test_onIncomingDataListener(t *testing.T) {
	t.Run("set reading and exit", func(t *testing.T) {
		d := NewProtocolDriver().(*Driver)
		d.serviceConfig = &config.ServiceConfig{}
		d.serviceConfig.OPCUAServer.DeviceName = "Test"

		err := d.onIncomingDataReceived("42", "TestResource")
		if err == nil {
			t.Error("expected err to exist in test environment")
		}
	})
}
