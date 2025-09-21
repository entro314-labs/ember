//go:build !darwin && !linux

package device

import (
	"errors"
	"fmt"
)

type DiskUtilDevice struct {
	DeviceIdentifier string
	Size             int64
	Internal         bool
	MediaName        string
	MediaType        string
	RemovableMedia   bool
	WritableMedia    bool
	BusProtocol      string
	DeviceTreePath   string
	IOKitSize        int64
	VolumeName       string
	VolumeSize       int64
	MountPoint       string
	FilesystemType   string
	DeviceNode       string
}

func ListDevices() ([]DiskUtilDevice, error) {
	return nil, errors.New("not implemented on this platform")
}

// GetDeviceDisplayName returns a human-readable name for the device
func (d *DiskUtilDevice) GetDisplayName() string {
	if d.VolumeName != "" {
		return fmt.Sprintf("%s (%s)", d.VolumeName, d.DeviceIdentifier)
	}
	return d.DeviceIdentifier
}

// GetSizeString returns a human-readable size string
func (d *DiskUtilDevice) GetSizeString() string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	size := float64(d.Size)
	switch {
	case size >= TB:
		return fmt.Sprintf("%.1f TB", size/TB)
	case size >= GB:
		return fmt.Sprintf("%.1f GB", size/GB)
	case size >= MB:
		return fmt.Sprintf("%.1f MB", size/MB)
	case size >= KB:
		return fmt.Sprintf("%.1f KB", size/KB)
	default:
		return fmt.Sprintf("%.0f B", size)
	}
}

// GetDeviceDescription returns a detailed description of the device
func (d *DiskUtilDevice) GetDeviceDescription() string {
	return d.GetSizeString()
}
