package syncthing

import (
	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/protocol"
)

// GetDevice takes a device ID, looks in the current configuration and returns
// that device.
func (s *Server) GetDevice(id protocol.DeviceID) *config.DeviceConfiguration {
	for _, device := range s.Config.Devices {
		if device.DeviceID == id {
			return &device
		}
	}

	return nil
}

// SetDevice takes a complete device and adds it to the local configuration.
// Server.Update() will then save this. Note that if the device already exists
// it is simply overwritten.
func (s *Server) SetDevice(device *config.DeviceConfiguration) error {
	s.RemoveDevice(device.DeviceID)

	s.Config.Devices = append(s.Config.Devices, *device)

	return nil
}

// RemoveDevice takes a device id and removes it from the local configuration.
// Server.Update() will then save this.
func (s *Server) RemoveDevice(id protocol.DeviceID) {
	for i, device := range s.Config.Devices {
		if device.DeviceID == id {
			s.Config.Devices[i] = s.Config.Devices[len(s.Config.Devices)-1]
			s.Config.Devices = s.Config.Devices[:len(s.Config.Devices)-1]
		}
	}
}
