// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
// Copyright (C) 2021 Schneider Electric
// Copyright (C) 2023 YIQISOFT
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	device_opcua "github.com/edgexfoundry/device-opcua-go"
	"github.com/edgexfoundry/device-opcua-go/internal/driver"
	"github.com/edgexfoundry/device-sdk-go/v3/pkg/startup"
)

const (
	serviceName string = "device-opcua"
)

func main() {
	sd := driver.NewProtocolDriver()
	startup.Bootstrap(serviceName, device_opcua.Version, sd)
}
