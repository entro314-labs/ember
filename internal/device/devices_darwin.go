//go:build darwin

package device

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/entro314-labs/ember/internal/types"
)

type DiskUtilOutput struct {
	AllDisksAndPartitions []struct {
		DeviceIdentifier string              `json:"DeviceIdentifier"`
		Size             int64               `json:"Size"`
		Internal         bool                `json:"Internal"`
		MediaName        string              `json:"MediaName"`
		MediaType        string              `json:"MediaType"`
		RemovableMedia   bool                `json:"RemovableMedia"`
		WritableMedia    bool                `json:"WritableMedia"`
		BusProtocol      string              `json:"BusProtocol"`
		DeviceTreePath   string              `json:"DeviceTreePath"`
		IOKitSize        int64               `json:"IOKitSize"`
		DeviceNode       string              `json:"DeviceNode"`
		Partitions       []DiskUtilPartition `json:"Partitions"`
	} `json:"AllDisksAndPartitions"`
}

type DiskUtilPartition struct {
	DeviceIdentifier string `json:"DeviceIdentifier"`
	Size             int64  `json:"Size"`
	VolumeName       string `json:"VolumeName"`
	VolumeSize       int64  `json:"VolumeSize"`
	MountPoint       string `json:"MountPoint"`
	FilesystemType   string `json:"FilesystemType"`
	DeviceNode       string `json:"DeviceNode"`
}

func ListDevices() ([]types.DiskUtilDevice, error) {
	// Get all disks and partitions
	cmd := exec.Command("diskutil", "list", "-json")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	var data DiskUtilOutput
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, fmt.Errorf("failed to parse diskutil output: %w", err)
	}

	var devices []types.DiskUtilDevice
	for _, disk := range data.AllDisksAndPartitions {
		// Filter for external, removable, and writable devices
		if disk.Internal || !disk.RemovableMedia || !disk.WritableMedia {
			continue
		}

		// Additional filtering for USB devices
		if !isUSBDevice(disk.BusProtocol, disk.DeviceTreePath) {
			continue
		}

		// Skip very small devices (less than 128 MB)
		if disk.Size < 128*1024*1024 {
			continue
		}

		device := types.DiskUtilDevice{
			DeviceIdentifier: disk.DeviceIdentifier,
			Size:             disk.Size,
			Internal:         disk.Internal,
			MediaName:        disk.MediaName,
			MediaType:        disk.MediaType,
			RemovableMedia:   disk.RemovableMedia,
			WritableMedia:    disk.WritableMedia,
			BusProtocol:      disk.BusProtocol,
			DeviceTreePath:   disk.DeviceTreePath,
			IOKitSize:        disk.IOKitSize,
			DeviceNode:       disk.DeviceNode,
		}

		// Get additional device info
		if info, err := getDeviceInfo(disk.DeviceIdentifier); err == nil {
			device.VolumeName = info.VolumeName
			device.VolumeSize = info.VolumeSize
			device.MountPoint = info.MountPoint
			device.FilesystemType = info.FilesystemType
		}

		// Perform enhanced safety analysis
		device.AnalyzeDeviceSafety()

		devices = append(devices, device)
	}

	return devices, nil
}

func isUSBDevice(busProtocol, deviceTreePath string) bool {
	// Check if it's a USB device
	if strings.Contains(strings.ToLower(busProtocol), "usb") {
		return true
	}
	if strings.Contains(strings.ToLower(deviceTreePath), "usb") {
		return true
	}
	// Also accept devices with these protocols as they might be USB-based
	usbProtocols := []string{"USB", "USB 2.0", "USB 3.0", "USB 3.1", "USB-C"}
	for _, protocol := range usbProtocols {
		if strings.EqualFold(busProtocol, protocol) {
			return true
		}
	}
	return false
}

func getDeviceInfo(deviceIdentifier string) (*types.DiskUtilDevice, error) {
	cmd := exec.Command("diskutil", "info", "-json", deviceIdentifier)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var info types.DiskUtilDevice
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, err
	}

	return &info, nil
}
