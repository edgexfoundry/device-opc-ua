# OPC-UA Device Service

## Overview
This repository is a Go-based EdgeX Foundry Device Service which uses OPC-UA protocol to interact with the devices or IoT objects.

## Feature

1. Subscribe data from OPCUA endpoint
2. Execute read command
2. Execute write command

## Prerequisite
* MongoDB
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

### Subscribe configuration
Modify `configuration.toml` file which under `./cmd/res` folder if needed
```toml
# Driver configs
[Driver]
  DeviceName = "SimulationServer"   # Name of Devcice exited
  Policy = "None"                   # Security policy: None, Basic128Rsa15, Basic256, Basic256Sha256. Default: auto
  Mode = "None"                     # Security mode: None, Sign, SignAndEncrypt. Default: auto
  CertFile = ""                     # Path to cert.pem. Required for security mode/policy != None
  KeyFile = ""                      # Path to private key.pem. Required for security mode/policy != None
  NodeID = "ns=5;s=Counter1"        # Node id to subscribe to
```
## Devic Profile

A Device Profile can be thought of as a template of a type or classification of Device. 

Write device profile for your own devices, difine deviceResources, deviceCommands and coreCommands. Please refer to `cmd/res/OpcuaServer.yaml`

Tips: name in deviceResources should consistent with OPCUA nodeid


## Installation and Execution
```bash
make build
make run
```

## Reference
* EdgeX Foundry Services: https://github.com/edgexfoundry/edgex-go
* Go OPCUA library: https://github.com/gopcua/opcua
* OPCUA Server: https://www.prosysopc.com/products/opc-ua-simulation-server
