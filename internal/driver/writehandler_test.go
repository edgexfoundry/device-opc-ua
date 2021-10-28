package driver

import (
	"reflect"
	"testing"

	"github.com/edgexfoundry/device-opcua-go/internal/config"
	sdkModel "github.com/edgexfoundry/device-sdk-go/v2/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/models"
	"github.com/gopcua/opcua"
)

func TestDriver_HandleWriteCommands(t *testing.T) {
	type args struct {
		deviceName string
		protocols  map[string]models.ProtocolProperties
		reqs       []sdkModel.CommandRequest
		params     []*sdkModel.CommandValue
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "NOK - no endpoint defined",
			args: args{
				deviceName: "Test",
				protocols:  map[string]models.ProtocolProperties{config.Protocol: {}},
				reqs:       []sdkModel.CommandRequest{{DeviceResourceName: "TestVar1"}},
			},
			wantErr: true,
		},
		{
			name: "OK - no connection to client",
			args: args{
				deviceName: "Test",
				protocols:  map[string]models.ProtocolProperties{config.Protocol: {config.Endpoint: "opc.tcp://test"}},
				reqs:       []sdkModel.CommandRequest{{DeviceResourceName: "TestVar1"}},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Driver{
				Logger: &logger.MockLogger{},
			}
			if err := d.HandleWriteCommands(tt.args.deviceName, tt.args.protocols, tt.args.reqs, tt.args.params); (err != nil) != tt.wantErr {
				t.Errorf("Driver.HandleWriteCommands() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDriver_processCommands(t *testing.T) {
	type args struct {
		reqs   []sdkModel.CommandRequest
		params []*sdkModel.CommandValue
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "NOK - invalid node id",
			args: args{
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "TestResource1",
					Attributes:         map[string]interface{}{NAMESPACE: "2"},
					Type:               common.ValueTypeInt32,
				}},
				params: []*sdkModel.CommandValue{{
					DeviceResourceName: "TestResource1",
					Type:               common.ValueTypeInt32,
					Value:              int32(42),
				}},
			},
			wantErr: true,
		},
		{
			name: "NOK - invalid value",
			args: args{
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "TestResource1",
					Attributes:         map[string]interface{}{NAMESPACE: "2", SYMBOL: "edgex/int32/test1"},
					Type:               common.ValueTypeInt32,
				}},
				params: []*sdkModel.CommandValue{{
					DeviceResourceName: "TestResource1",
					Type:               common.ValueTypeString,
					Value:              "foobar",
				}},
			},
			wantErr: true,
		},
		{
			name: "OK - command request with one parameter (no mock client)",
			args: args{
				reqs: []sdkModel.CommandRequest{{
					DeviceResourceName: "TestResource1",
					Attributes:         map[string]interface{}{NAMESPACE: "2", SYMBOL: "edgex/int32/test1"},
					Type:               common.ValueTypeInt32,
				}},
				params: []*sdkModel.CommandValue{{
					DeviceResourceName: "TestResource1",
					Type:               common.ValueTypeInt32,
					Value:              int32(42),
				}},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Driver{
				Logger: &logger.MockLogger{},
			}
			if err := d.processWriteCommands(opcua.NewClient("opc.tcp//test"), tt.args.reqs, tt.args.params); (err != nil) != tt.wantErr {
				t.Errorf("Driver.processCommands() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_newCommandValue(t *testing.T) {
	type args struct {
		valueType string
		param     *sdkModel.CommandValue
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name:    "NOK - unknown type",
			args:    args{valueType: "uknown"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "NOK - bool value - mismatching types",
			args:    args{valueType: common.ValueTypeBool, param: &sdkModel.CommandValue{Value: "42", Type: common.ValueTypeString}},
			want:    false,
			wantErr: true,
		},
		{
			name:    "OK - bool value - matching types",
			args:    args{valueType: common.ValueTypeBool, param: &sdkModel.CommandValue{Value: true, Type: common.ValueTypeBool}},
			want:    true,
			wantErr: false,
		},
		{
			name:    "OK - string value",
			args:    args{valueType: common.ValueTypeString, param: &sdkModel.CommandValue{Value: "test", Type: common.ValueTypeString}},
			want:    "test",
			wantErr: false,
		},
		{
			name:    "OK - uint8 value",
			args:    args{valueType: common.ValueTypeUint8, param: &sdkModel.CommandValue{Value: uint8(5), Type: common.ValueTypeUint8}},
			want:    uint8(5),
			wantErr: false,
		},
		{
			name:    "OK - uint16 value",
			args:    args{valueType: common.ValueTypeUint16, param: &sdkModel.CommandValue{Value: uint16(5), Type: common.ValueTypeUint16}},
			want:    uint16(5),
			wantErr: false,
		},
		{
			name:    "OK - uint32 value",
			args:    args{valueType: common.ValueTypeUint32, param: &sdkModel.CommandValue{Value: uint32(5), Type: common.ValueTypeUint32}},
			want:    uint32(5),
			wantErr: false,
		},
		{
			name:    "OK - uint64 value",
			args:    args{valueType: common.ValueTypeUint64, param: &sdkModel.CommandValue{Value: uint64(5), Type: common.ValueTypeUint64}},
			want:    uint64(5),
			wantErr: false,
		},
		{
			name:    "OK - int8 value",
			args:    args{valueType: common.ValueTypeInt8, param: &sdkModel.CommandValue{Value: int8(5), Type: common.ValueTypeInt8}},
			want:    int8(5),
			wantErr: false,
		},
		{
			name:    "OK - int16 value",
			args:    args{valueType: common.ValueTypeInt16, param: &sdkModel.CommandValue{Value: int16(5), Type: common.ValueTypeInt16}},
			want:    int16(5),
			wantErr: false,
		},
		{
			name:    "OK - int32 value",
			args:    args{valueType: common.ValueTypeInt32, param: &sdkModel.CommandValue{Value: int32(5), Type: common.ValueTypeInt32}},
			want:    int32(5),
			wantErr: false,
		},
		{
			name:    "OK - int64 value",
			args:    args{valueType: common.ValueTypeInt64, param: &sdkModel.CommandValue{Value: int64(5), Type: common.ValueTypeInt64}},
			want:    int64(5),
			wantErr: false,
		},
		{
			name:    "OK - float32 value",
			args:    args{valueType: common.ValueTypeFloat32, param: &sdkModel.CommandValue{Value: float32(5), Type: common.ValueTypeFloat32}},
			want:    float32(5),
			wantErr: false,
		},
		{
			name:    "OK - float64 value",
			args:    args{valueType: common.ValueTypeFloat64, param: &sdkModel.CommandValue{Value: float64(5), Type: common.ValueTypeFloat64}},
			want:    float64(5),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newCommandValue(tt.args.valueType, tt.args.param)
			if (err != nil) != tt.wantErr {
				t.Errorf("newCommandValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newCommandValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
