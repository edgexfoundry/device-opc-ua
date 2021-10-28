package driver

const (
	// Protocol is the supported device protocol
	Protocol = "opcua"
	// Endpoint is a constant string
	Endpoint = "Endpoint"
)

const (
	// CustomConfigSectionName is the name of the configuration options
	// section in /cmd/res/configuration.toml
	CustomConfigSectionName = "OPCUAServer"
	// WritableInfoSectionName is the Writable section key
	WritableInfoSectionName = CustomConfigSectionName + "/Writable"
)

const (
	// NAMESPACE attribute
	NAMESPACE = "namespace"
	// SYMBOL attribute
	SYMBOL = "symbol"
	// OBJECT attribute
	OBJECT = "object"
	// METHOD attribute
	METHOD = "method"
	// INPUTMAP attribute
	INPUTMAP = "inputMap"
)
