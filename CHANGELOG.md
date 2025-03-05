<a name="EdgeX OPC-UA Device Service (found in device-opc-ua) Changelog"></a>
## EdgeX OPC-UA Device Service
[Github repository](https://github.com/edgexfoundry/device-opc-ua)

### Change Logs for EdgeX Dependencies
- [go-mod-bootstrap](https://github.com/edgexfoundry/go-mod-bootstrap/blob/main/CHANGELOG.md)
- [go-mod-core-contracts](https://github.com/edgexfoundry/go-mod-core-contracts/blob/main/CHANGELOG.md)
- [go-mod-messaging](https://github.com/edgexfoundry/go-mod-messaging/blob/main/CHANGELOG.md)
- [go-mod-registry](https://github.com/edgexfoundry/go-mod-registry/blob/main/CHANGELOG.md) 
- [go-mod-secrets](https://github.com/edgexfoundry/go-mod-secrets/blob/main/CHANGELOG.md) (indirect dependency)
- [go-mod-configuration](https://github.com/edgexfoundry/go-mod-configuration/blob/main/CHANGELOG.md) (indirect dependency)

## [4.0.0] Odessa - 2025-03-12 (Only compatible with the 4.x releases)

### ✨ Features

- Enable PIE support for ASLR and full RELRO ([f4576cb…](https://github.com/edgexfoundry/device-opc-ua/commit/f4576cb6e730d5cb56728cd5301c3469e6700745))
- Support reuse of the opcua.Client ([#48](https://github.com/edgexfoundry/device-opc-ua/issues/48)) ([0652287…](https://github.com/edgexfoundry/device-opc-ua/commit/0652287e7ffe2b31c8f997123275b50b3773b654))

### ♻ Code Refactoring

- Update module to v4 ([78189af…](https://github.com/edgexfoundry/device-opc-ua/commit/78189afc06e4f293a751a0d27007109e12aff490))
```text

BREAKING CHANGE: update go module to v4

```

### 🐛 Bug Fixes

- Make variable usage consistent in Makefile ([2d2e1ba…](https://github.com/edgexfoundry/device-opc-ua/commit/2d2e1ba9e6b6bf1cb27eb31121842de0592b21f6))
- Only one ldflags flag is allowed ([fe1b7d4…](https://github.com/edgexfoundry/device-opc-ua/commit/fe1b7d460f373c3184ce3951159384d1eb3ec948))
- Retention failure ([#54](https://github.com/edgexfoundry/device-opc-ua/issues/54)) ([ebc1033…](https://github.com/edgexfoundry/device-opc-ua/commit/ebc1033c4e999d7351c7388ead58caee164ca851))
- Change go mod name to device-opc-ua ([11f2cd1…](https://github.com/edgexfoundry/device-opc-ua/commit/11f2cd1c89942d99ef8344c9c8430f1263f63267))

### 👷 Build

- Upgrade to go-1.23 and Alpine 3.20 ([59a52fd…](https://github.com/edgexfoundry/device-opc-ua/commit/59a52fd5833a6e42fe3b7b3e47a5549daf0b54c2))

### 🤖 Continuous Integration

- Add Device OPC-UA to Jenkinsfile ([4b71756…](https://github.com/edgexfoundry/device-opc-ua/commit/4b717569e2b63042dcbc4f40566b8e46e339337e))


