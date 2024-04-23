/*
 * Copyright (c) 2024.  liushenglong_8597@outlook.com.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mock

import (
	"fmt"
	"github.com/edgexfoundry/device-sdk-go/v3/pkg/interfaces"
	sdkModels "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	interfaces2 "github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/labstack/echo/v4"
	"net/http"
)

type DeviceServiceSdk struct {
	devices              map[string]models.Device
	profiles             map[string]models.DeviceProfile
	driverConfigs        map[string]string
	asyncValues          chan *sdkModels.AsyncValues
	LoadCustomConfigImpl func(customConfig interfaces.UpdatableConfig, sectionName string) error
}

func NewDeviceSdk() interfaces.DeviceServiceSDK {
	dss := &DeviceServiceSdk{
		devices:     make(map[string]models.Device),
		profiles:    make(map[string]models.DeviceProfile),
		asyncValues: make(chan *sdkModels.AsyncValues),
	}
	return interfaces.DeviceServiceSDK(dss)
}

func (dss *DeviceServiceSdk) AddDevice(device models.Device) (string, error) {
	if len(device.Name) == 0 {
		return "", fmt.Errorf("DeviceName should not be null")
	}
	dss.devices[device.Name] = device
	return device.Name, nil
}
func (dss *DeviceServiceSdk) Devices() []models.Device {
	ret := make([]models.Device, 0, len(dss.devices))
	for _, device := range dss.devices {
		ret = append(ret, device)
	}
	return ret
}
func (dss *DeviceServiceSdk) GetDeviceByName(name string) (models.Device, error) {
	device, ok := dss.devices[name]
	if !ok {
		return models.Device{}, fmt.Errorf("device not found")
	}
	return device, nil
}
func (dss *DeviceServiceSdk) UpdateDevice(device models.Device) error {
	device, ok := dss.devices[device.Name]
	if !ok {
		return fmt.Errorf("device not found")
	}
	dss.devices[device.Name] = device
	return nil
}
func (dss *DeviceServiceSdk) RemoveDeviceByName(name string) error {
	_, ok := dss.devices[name]
	if !ok {
		return fmt.Errorf("device not found")
	}
	delete(dss.devices, name)
	return nil
}
func (dss *DeviceServiceSdk) AddDeviceProfile(profile models.DeviceProfile) (string, error) {
	dss.profiles[profile.Name] = profile
	return profile.Name, nil
}
func (dss *DeviceServiceSdk) DeviceProfiles() []models.DeviceProfile {
	ret := make([]models.DeviceProfile, 0, len(dss.profiles))
	for _, profile := range dss.profiles {
		ret = append(ret, profile)
	}
	return ret
}
func (dss *DeviceServiceSdk) GetProfileByName(name string) (models.DeviceProfile, error) {
	profile, ok := dss.profiles[name]
	if !ok {
		return models.DeviceProfile{}, fmt.Errorf("device profile with name %s not found", name)
	}
	return profile, nil
}
func (dss *DeviceServiceSdk) UpdateDeviceProfile(profile models.DeviceProfile) error {
	_, ok := dss.profiles[profile.Name]
	if !ok {
		return fmt.Errorf("device profile with name %s not found", profile.Name)
	}
	dss.profiles[profile.Name] = profile
	return nil
}
func (dss *DeviceServiceSdk) RemoveDeviceProfileByName(name string) error {
	delete(dss.profiles, name)
	return nil
}
func (dss *DeviceServiceSdk) AddProvisionWatcher(watcher models.ProvisionWatcher) (string, error) {
	return "", nil
}
func (dss *DeviceServiceSdk) ProvisionWatchers() []models.ProvisionWatcher {
	return nil
}
func (dss *DeviceServiceSdk) GetProvisionWatcherByName(name string) (models.ProvisionWatcher, error) {
	return models.ProvisionWatcher{}, nil
}
func (dss *DeviceServiceSdk) UpdateProvisionWatcher(watcher models.ProvisionWatcher) error {
	return nil
}
func (dss *DeviceServiceSdk) RemoveProvisionWatcher(name string) error {
	return nil
}
func (dss *DeviceServiceSdk) DeviceResource(deviceName string, deviceResource string) (models.DeviceResource, bool) {
	device, ok := dss.devices[deviceName]
	if !ok {
		return models.DeviceResource{}, false
	}
	profile, ok := dss.profiles[device.ProfileName]
	if !ok {
		return models.DeviceResource{}, false
	}
	for _, resource := range profile.DeviceResources {
		if deviceResource == resource.Name {
			return resource, true
		}
	}
	return models.DeviceResource{}, false
}
func (dss *DeviceServiceSdk) DeviceCommand(deviceName string, commandName string) (models.DeviceCommand, bool) {
	device, ok := dss.devices[deviceName]
	if !ok {
		return models.DeviceCommand{}, false
	}
	profile, ok := dss.profiles[device.ProfileName]
	if !ok {
		return models.DeviceCommand{}, false
	}
	for _, command := range profile.DeviceCommands {
		if commandName == command.Name {
			return command, true
		}
	}
	return models.DeviceCommand{}, false
}
func (dss *DeviceServiceSdk) AddDeviceAutoEvent(deviceName string, event models.AutoEvent) error {
	return nil
}
func (dss *DeviceServiceSdk) RemoveDeviceAutoEvent(deviceName string, event models.AutoEvent) error {
	return nil
}
func (dss *DeviceServiceSdk) UpdateDeviceOperatingState(name string, state models.OperatingState) error {
	device, ok := dss.devices[name]
	if !ok {
		return fmt.Errorf("device not found")
	}
	device.OperatingState = state
	return nil
}
func (dss *DeviceServiceSdk) DeviceExistsForName(name string) bool {
	_, ok := dss.devices[name]
	return ok
}
func (dss *DeviceServiceSdk) PatchDevice(updateDevice dtos.UpdateDevice) error {
	return nil
}
func (dss *DeviceServiceSdk) Run() error {
	return nil
}
func (dss *DeviceServiceSdk) Name() string {
	return ""
}
func (dss *DeviceServiceSdk) Version() string {
	return ""
}
func (dss *DeviceServiceSdk) AsyncReadingsEnabled() bool {
	return false
}
func (dss *DeviceServiceSdk) AsyncValuesChannel() chan *sdkModels.AsyncValues {
	return dss.asyncValues
}
func (dss *DeviceServiceSdk) DiscoveredDeviceChannel() chan []sdkModels.DiscoveredDevice {
	return nil
}
func (dss *DeviceServiceSdk) DeviceDiscoveryEnabled() bool {
	return false
}
func (dss *DeviceServiceSdk) DriverConfigs() map[string]string {
	return dss.driverConfigs
}
func (dss *DeviceServiceSdk) AddRoute(route string, handler func(http.ResponseWriter, *http.Request), methods ...string) error {
	return nil
}
func (dss *DeviceServiceSdk) AddCustomRoute(route string, authentication interfaces.Authentication, handler func(e echo.Context) error, methods ...string) error {
	return nil
}

func (dss *DeviceServiceSdk) LoadCustomConfig(customConfig interfaces.UpdatableConfig, sectionName string) error {
	if dss.LoadCustomConfigImpl == nil {
		return fmt.Errorf("LoadCustomConfigImpl not set")
	}
	return dss.LoadCustomConfigImpl(customConfig, sectionName)
}
func (dss *DeviceServiceSdk) ListenForCustomConfigChanges(configToWatch interface{}, sectionName string, changedCallback func(interface{})) error {
	return nil
}
func (dss *DeviceServiceSdk) LoggingClient() logger.LoggingClient {
	return logger.MockLogger{}
}
func (dss *DeviceServiceSdk) SecretProvider() interfaces2.SecretProvider {
	return nil
}
func (dss *DeviceServiceSdk) MetricsManager() interfaces2.MetricsManager {
	return nil
}
