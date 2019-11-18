// set OPCUA Server Nodes configuration
package driver

import (
	"encoding/json"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/models"
	"reflect"
	"strconv"
)

type ConnectionInfo struct {
	Endpoint     	string
}
type SubscribeJson struct {
	Devices []DevicesInfo
}
type DevicesInfo struct {
	DeviceName		string    `json:"deviceName"`
	NodeIds			[]string  `json:"nodeIds"`
	Policy 			string	  `json:"policy"`
	Mode  			string	  `json:"mode"`
	CertFile	 	string    `json:"certFile"`
	KeyFile 		string	  `json:"keyFile"`
}

// CreateDriverConfig use to load driver config for incoming listener and response listener
func CreateDriverConfig(configMap map[string]string) (*SubscribeJson, error) {
	config := new(SubscribeJson)
	data := []byte(configMap["SubscribeJson"])

	err := json.Unmarshal(data, config)
	if err != nil {
		return config, err
	}
	return config, nil
}

// CreateConnectionInfo use to load connectionInfo for read and write command
func CreateConnectionInfo(protocols map[string]models.ProtocolProperties) (*ConnectionInfo, error) {
	info := new(ConnectionInfo)
	protocol, ok := protocols["opcua"]
	if !ok {
		return info, fmt.Errorf("unable to load config, 'opcua' not exist")
	}

	err := load(protocol, info)
	if err != nil {
		return info, err
	}
	return info, nil
}

// load by reflect to check map key and then fetch the value
func load(config map[string]string, des interface{}) error {
	errorMessage := "unable to load config, '%s' not exist"
	val := reflect.ValueOf(des).Elem()
	for i := 0; i < val.NumField(); i++ {
		typeField := val.Type().Field(i)
		valueField := val.Field(i)

		val, ok := config[typeField.Name]
		if !ok {
			return fmt.Errorf(errorMessage, typeField.Name)
		}

		switch valueField.Kind() {
		case reflect.Int:
			intVal, err := strconv.Atoi(val)
			if err != nil {
				return err
			}
			valueField.SetInt(int64(intVal))
		case reflect.String:
			valueField.SetString(val)
		default:
			return fmt.Errorf("none supported value type %v ,%v", valueField.Kind(), typeField.Name)
		}
	}
	return nil
}