# OPC-UA Device Service

## Overview
This repository is a Go-based EdgeX Foundry Device Service which uses OPC-UA protocol to interact with the devices or IoT objects.

## Feature

1. Subscribe data from OPCUA endpoint
2. Execute read command
2. Execute write command

## Prerequisite
* MongoDB / Redis
* Edgex-go: core data, core command, core metadata
* OPCUA Server

## Predefined configuration

### Pre-define Devices
Define devices for device-sdk to auto upload device profile and create device instance. Please modify `configuration.toml` file which under `./cmd/res` folder
```toml
# Pre-define Devices
[[DeviceList]]
  Name = "SimulationServer"
  Profile = "OPCUA-Server"
  Description = "OPCUA device is created for test purpose"
  Labels = [ "test" ]
  [DeviceList.Protocols]
      [DeviceList.Protocols.opcua]
          Endpoint = "opc.tcp://Burning-Laptop:53530/OPCUA/SimulationServer"
```

Endpoint field provide the endpoint(protocol + host + port + server dir) config

### Subscribe configuration
Modify `configuration.toml` file which under `./cmd/res` folder if needed
```toml
# Driver configs
[Driver]
  SubscribeJson = " {\"devices\":[{\"deviceName\":\"SimulationServer\",\"nodeIds\":[\"ns=5;s=Counter1\",\"ns=5;s=Random1\"],\"policy\":\"None\",\"mode\":\"None\",\"certFile\":\"\",\"keyFile\":\"\"}]} "
```
SubscribeJson field provide the subscription info for driver to subscribe the specific devices and its nodes, and some policy, security mode, certification file and key file also included.
This info must be set as JSON format and need to escape especially because the driver struct(map[string]string) defined in [go-mod-core-contracts](https://github.com/edgexfoundry/go-mod-core-contracts)

## Devic Profile

A Device Profile can be thought of as a template of a type or classification of Device. 

Write device profile for your own devices, difine deviceResources, deviceCommands and coreCommands. Please refer to `cmd/res/OpcuaServer.yaml`

Tips: name in deviceResources should consistent with OPCUA nodeid and make sure the type match with each other


## Installation and Execution
```bash
make build
make run
```

## Reference
* EdgeX Foundry Services: https://github.com/edgexfoundry/edgex-go
* Go OPCUA library: https://github.com/gopcua/opcua
* OPCUA Server: https://www.prosysopc.com/products/opc-ua-simulation-server

## Buy me a cup of coffee
If you like this project, please star it to make encouragements.
