# OPC-UA Device Service

## Overview

This repository is a Go-based EdgeX Foundry Device Service which uses OPC-UA protocol to interact with the devices or IoT objects.

## Features

1. Subscribe/Unsubscribe one or more variables (writable configuration)
2. Execute read command
3. Execute write command
4. Execute method (using Read command of device SDK)

## Prerequisites

- Edgex-go: core data, core command, core metadata
- OPCUA Server

## Predefined configuration

### Pre-defined Devices

Define devices for device-sdk to auto upload device profile and create device instance. Please modify `devices.toml` file found under the `./cmd/res/devices` folder

```toml
# Pre-define Devices
[[DeviceList]]
  Name = "SimulationServer"
  Profile = "OPCUA-Server"
  Description = "OPCUA device is created for test purpose"
  Labels = [ "test" ]
  [DeviceList.Protocols]
      [DeviceList.Protocols.opcua]
          Endpoint = "opc.tcp://node-red:55880/OPCUA/SimulationServer"
```

### Configuration

Modify `configuration.toml` file found under the `./cmd/res` folder if needed

```toml
# Driver configs
[OPCUAServer]
DeviceName = "SimulationServer"   # Name of Devcice exited
Policy = "None"                   # Security policy: None, Basic128Rsa15, Basic256, Basic256Sha256. Default: auto
Mode = "None"                     # Security mode: None, Sign, SignAndEncrypt. Default: auto
CertFile = ""                     # Path to cert.pem. Required for security mode/policy != None
KeyFile = ""                      # Path to private key.pem. Required for security mode/policy != None
  [OPCUAServer.Writable]
  Resources = "Counter1,Random1"    # Device resources related to Node IDs to subscribe to (comma-separated values)
```

## Device Profile

A Device Profile can be thought of as a template of a type or classification of a Device.

Write a device profile for your own devices; define `deviceResources` and `deviceCommands`. Please refer to `cmd/res/profiles/OpcuaServer.yaml`

## Installation and Execution

```bash
make build
make run
```

## Build a Container Image

```bash
make docker
```

## Testing

Running unit tests starts a mock OPCUA server on port `48408`.

The mock server defines the following attributes:

| Variable Name | Type | Default Value | Writable |
|-|-|-|-|
|`ro_bool`|`Boolean`|`True`||
|`rw_bool`|`Boolean`|`True`|✅|
|`ro_int32`|`Int32`|`5`||
|`rw_int32`|`Int32`|`5`|✅|
|`square`|`Method`|`Int64` (return value)||

All attributes are defined in `ns=2`.

```bash
# Install requirements (if necessary)
python3 -m pip install opcua
# Run tests
make test
```

## Reference

- [EdgeX Foundry Services](https://github.com/edgexfoundry/edgex-go)
- [Go OPCUA library](https://github.com/gopcua/opcua)
- [OPCUA Server](https://www.prosysopc.com/products/opc-ua-simulation-server)
