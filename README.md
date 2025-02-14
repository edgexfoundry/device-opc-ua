# OPC-UA Device Service


> **Warning**  
> The **main** branch of this repository contains work-in-progress development code for the upcoming release, and is **not guaranteed to be stable or working**.
> It is only compatible with the [main branch of edgex-compose](https://github.com/edgexfoundry/edgex-compose) which uses the Docker images built from the **main** branch of this repo and other repos.
>
> **The source for the latest release can be found at [Releases](https://github.com/edgexfoundry/device-opc-ua/releases).**

## Documentation

For latest documentation please visit https://docs.edgexfoundry.org/latest/microservices/device/services/device-opc-ua/Purpose

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

### Configuration

Modify [configuration.yaml](./cmd/res/configuration.yaml) file found under the `./cmd/res` folder if needed

### Pre-defined Devices

Define devices for device-sdk to auto upload device profile and create device instance. Please modify [Simple_Devices.yaml](./cmd/res/devices/Simple-Devices.yaml) file found under the `./cmd/res/devices` folder.

### Device Profile

A Device Profile can be thought of as a template of a type or classification of a Device.

Write a device profile for your own devices; define `deviceResources` and `deviceCommands`. Please refer to [OpcuaServer.yaml](cmd/res/profiles/OpcuaServer.yaml).

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


## Build Instructions

1.  Clone the device-rest-go repo with the following command:

        git clone https://github.com/edgexfoundry/device-opc-ua.git

2.  Build a docker image by using the following command:

        make docker

3.  Alternatively the device service can be built natively:

        make build

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

## Packaging

This component is packaged as docker images.

Please refer to the [Dockerfile](./Dockerfile) and [Docker Compose Builder](https://github.com/edgexfoundry/edgex-compose/tree/main/compose-builder) scripts.

## Reference

- [EdgeX Foundry Services](https://github.com/edgexfoundry/edgex-go)
- [Go OPCUA library](https://github.com/gopcua/opcua)
- [OPCUA Server](https://www.prosysopc.com/products/opc-ua-simulation-server)
