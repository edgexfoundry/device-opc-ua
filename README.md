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
5. Username/password authentication per device, or anonymous when no secret is set
6. Browse the direct children of one node (`GET /api/v3/opcua/device/{deviceName}/browse`)

## Username and password

Each device may set optional `opcua` protocol properties `Policy`, `Mode`, and `SecretName`.

- Missing `Policy` or `Mode` uses `OPCUAServer.Policy` and `OPCUAServer.Mode`.
- Missing `SecretName` keeps an anonymous session.
- `SecretName` is passed to `SecretProvider.GetSecret`. The secret keys are `username` and `password`. Do not put the password in the device definition.

Devices that share the same endpoint, policy, mode, and secret name share one OPC UA session. Read, write, and subscribe use that session. The subscription listener cancels its subscription and does not close the client. A transport error on that subscription drops the session, and the listener dials again. The session is closed when the last device releases it, when the service stops, or after a transport error. A bad status on one node does not close the session. Removing one device does not close a session that another device still holds.

`Policy` or `Mode` other than `None` requires `OPCUAServer.CertFile` and `OPCUAServer.KeyFile`. A server may reject a username token on security policy `None`.

```yaml
protocols:
  opcua:
    Endpoint: "opc.tcp://192.0.2.20:4840"
    Policy: "Basic256Sha256"
    Mode: "SignAndEncrypt"
    SecretName: "opcua/plc-1"
```

Store the secret through the device service. In secure mode only this service can read it back:

```
POST /api/v3/secret
```

```json
{
  "apiVersion": "v3",
  "secretName": "opcua/plc-1",
  "secretData": [
    {"key": "username", "value": "alice"},
    {"key": "password", "value": "secret"}
  ]
}
```

## Browse one level

```text
GET /api/v3/opcua/device/{deviceName}/browse?nodeId=&maxNodes=200
```

`deviceName` is the EdgeX device name. `nodeId` is a node on that server. Without `nodeId` the call returns the children of the `Objects` folder (`i=85`). The response does not descend. The caller sends a child's `nodeId` to open the next level. A `depth` parameter is rejected.

`maxNodes` defaults to 200 and cannot exceed 1000. `includeProperties` defaults to false and drops `HasProperty` references. `nodeClass` is an optional comma-separated list of `Object`, `Variable`, `Method`, and `View`. When a sibling list does not fit, the response sets `truncated` and `continuationToken`. The token is standard base64 of the OPC UA continuation point for the current session. URL-encode it and repeat the same `nodeId`. An unknown token returns `409`. The call times out after 15 seconds.

Variable nodes include `dataType`, `dataTypeId`, `valueRank`, and `accessLevel`. `edgexValueType` is set only for a scalar built-in type with a direct EdgeX match (`Float` to `Float32`, `Double` to `Float64`, integers, `Boolean` to `Bool`, `String` to `String`, `ByteString` to `Binary`). Structures, enumerations, and arrays (`valueRank` >= 0) omit it.

The route is registered as `Unauthenticated`. Device command GET and PUT on the same port still require the EdgeX JWT. Browse uses the same session as read and write, including `SecretName`. A transport error drops that session. A `continuationToken` from the dropped session returns `409`, and the same `nodeId` is requested again without the token.

```bash
curl -fsS "http://localhost:59997/api/v3/opcua/device/SimulationServer/browse"
curl -fsS --get "http://localhost:59997/api/v3/opcua/device/SimulationServer/browse" \
  --data-urlencode "nodeId=ns=3;i=1001"
```

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
