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
- OPCUA Server (Prosys Simulation Server, for example)

## Predefined configuration

### Simulation Server

Download the Prosys OPC UA Simulation Server from [here](https://www.prosysopc.com/products/opc-ua-simulation-server/). Install and run it to have access to the default configured resources.

### Pre-defined Devices

Define devices for device-sdk to auto upload device profile and create device instance. Please modify `devices.toml` file found under the `./cmd/res/devices` folder.

> This device service is currently limited to a single device instance.

```toml
# Pre-define Devices
[[DeviceList]]
  Name = "SimulationServer"
  Profile = "OPCUA-Server"
  Description = "OPCUA device is created for test purpose"
  Labels = [ "test" ]
  [DeviceList.Protocols]
      [DeviceList.Protocols.opcua]
          Endpoint = "opc.tcp://127.0.0.1:53530/OPCUA/SimulationServer"
```

### Configuration

Modify `configuration.toml` file found under the `./cmd/res` folder if needed

```toml
# Driver configs
[OPCUAServer]
DeviceName = "SimulationServer"   # Name of existing Device
Policy = "None"                   # Security policy: None, Basic128Rsa15, Basic256, Basic256Sha256. Default: None
Mode = "None"                     # Security mode: None, Sign, SignAndEncrypt. Default: None
CertFile = ""                     # Path to cert.pem. Required for security mode/policy != None
KeyFile = ""                      # Path to private key.pem. Required for security mode/policy != None
  [OPCUAServer.Writable]
  Resources = "Counter,Random"    # Device resources related to Node IDs to subscribe to (comma-separated values)
```

## Device Profile

A Device Profile can be thought of as a template of a type or classification of a Device.

Write a device profile for your own devices; define `deviceResources` and `deviceCommands`. Please refer to `cmd/res/profiles/OpcuaServer.yaml`.

### Using Methods

OPC UA methods can be referenced in the device profile and called with a read command. An example of a method instance might look something like this:

```yaml
deviceResources:
  -
    name: "SetDefaultsMethod"
    description: "Set all variables to their default values"
    properties:
      # Specifies the response value type
      valueType: "String"
      readWrite: "R"
    attributes:
      { methodId: "ns=5;s=Defaults", objectId: "ns=5;i=1111" }

deviceCommands:
  -
    name: "SetDefaults"
    isHidden: false
    readWrite: "R"
    resourceOperations:
      - { deviceResource: "SetDefaultsMethod" }
```

Notice that method calls require specifying the NodeId of both the method and its parent object.

The `attributes` field may also contain an `inputMap: []` that passes parameters to the method, if applicable.

## Build and Run

```bash
make build
cd cmd
EDGEX_SECURITY_SECRET_STORE=false ./device-opcua -cp -d -o
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
