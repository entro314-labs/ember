//go:build linux

package device

import (
	"fmt"
	"os/exec"
	"strings"
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
	// Use lsblk to list block devices on Linux
	cmd := exec.Command("lsblk", "-J", "-o", "NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,MODEL,HOTPLUG,RM")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	// For now, return empty list - this would need proper JSON parsing
	// of lsblk output to implement properly
	_ = out
	return []DiskUtilDevice{}, nil
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
	parts := []string{d.GetSizeString()}

	if d.BusProtocol != "" {
		parts = append(parts, d.BusProtocol)
	}

	if d.FilesystemType != "" {
		parts = append(parts, d.FilesystemType)
	}

	if d.RemovableMedia {
		parts = append(parts, "Removable")
	}

	return strings.Join(parts, " • ")
}

// Linux-specific validation functions
func ValidateDevice(deviceID string) error {
	// Check if device exists
	if _, err := exec.Command("test", "-b", "/dev/"+deviceID).Output(); err != nil {
		return fmt.Errorf("device /dev/%s does not exist or is not a block device", deviceID)
	}
	return nil
}

func unmountDisk(deviceID string) error {
	// Try to unmount all partitions of the device
	cmd := exec.Command("umount", "/dev/"+deviceID+"*")
	cmd.Run() // Ignore errors as partitions might not be mounted
	return nil
}

func GetPartitionMountPoint(partition string) (string, error) {
	cmd := exec.Command("findmnt", "-n", "-o", "TARGET", partition)
	out, err := cmd.Output()
	if err != nil {
		return "", nil // Not mounted
	}
	return strings.TrimSpace(string(out)), nil
}
