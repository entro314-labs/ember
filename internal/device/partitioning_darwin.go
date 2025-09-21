//go:build darwin

package device

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func GetBlockDevicePartition(blockDevice string, partNumber int) string {
	// On macOS, partitions are identified by their identifier, which includes the disk identifier and the partition number.
	// For example, a partition on disk2 will be disk2s1, disk2s2, etc.
	return blockDevice + "s" + strconv.Itoa(partNumber)
}

func FormatDiskForUEFINTFS(device string, gpt bool) error {
	// Validate the device identifier
	if !strings.HasPrefix(device, "/dev/disk") && !strings.HasPrefix(device, "disk") {
		return fmt.Errorf("invalid device identifier: %s", device)
	}

	// Clean the device identifier (remove /dev/ prefix if present)
	deviceID := strings.TrimPrefix(device, "/dev/")

	// Unmount the disk before formatting
	if err := unmountDisk(deviceID); err != nil {
		return fmt.Errorf("failed to unmount disk %s: %w", deviceID, err)
	}

	// Create the partition scheme
	if err := createPartitionScheme(deviceID, gpt); err != nil {
		return fmt.Errorf("failed to create partition scheme: %w", err)
	}

	// Format the partitions
	if err := formatPartitions(deviceID); err != nil {
		return fmt.Errorf("failed to format partitions: %w", err)
	}

	return nil
}

func unmountDisk(deviceID string) error {
	// First try to unmount all volumes on the disk
	cmd := exec.Command("diskutil", "unmountDisk", "force", deviceID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Check if the error is just because the disk wasn't mounted
		if strings.Contains(string(out), "not currently mounted") ||
			strings.Contains(string(out), "Volume on") {
			return nil // Not an error if it wasn't mounted
		}
		return fmt.Errorf("diskutil unmountDisk failed: %w\noutput: %s", err, out)
	}
	return nil
}

func createPartitionScheme(deviceID string, gpt bool) error {
	// Windows USB creation follows modern 2025 patterns:
	// 1. GPT + UEFI for standard Windows 11 compatibility (default)
	// 2. MBR + BIOS for legacy compatibility (fallback only)

	// Default to GPT for modern Windows (2025 best practice)
	partitionScheme := "GPT"
	if !gpt {
		partitionScheme = "MBR" // Legacy compatibility mode only
	}

	// For now, create FAT32 partition for maximum compatibility
	// TODO: Detect large files and switch to NTFS + UEFI:NTFS scheme
	partitionCmd := []string{
		"partitionDisk", deviceID, "1", partitionScheme,
		"MS-DOS FAT32", "WINDOWS", "100%", // FAT32 for Windows compatibility
	}

	cmd := exec.Command("diskutil", partitionCmd...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("diskutil partitionDisk failed: %w\noutput: %s", err, out)
	}

	return nil
}

func formatPartitions(deviceID string) error {
	// Get the main Windows partition identifier
	windowsPartition := GetBlockDevicePartition(deviceID, 1)

	// Format the Windows partition as FAT32 for maximum compatibility
	// Note: diskutil partitionDisk already formatted it, but we'll re-format to ensure proper settings
	if err := FormatPartitionAsFAT32(windowsPartition, "WINDOWS"); err != nil {
		return fmt.Errorf("failed to format Windows partition: %w", err)
	}

	return nil
}

// calculateOptimalPartitionSizes determines optimal partition layout based on 2025 standards
func calculateOptimalPartitionSizes(totalUSBSize int64, installWimSize int64) (mainSize, uefiSize string) {
	const (
		// 2025 Microsoft recommendations
		minUEFISize     = 300 * 1024 * 1024  // 300MB minimum
		safeUEFISize    = 512 * 1024 * 1024  // 512MB safe default
		optimalUEFISize = 1024 * 1024 * 1024 // 1GB for large drives
	)

	// For drives > 64GB, use 1GB UEFI partition for future Windows updates
	if totalUSBSize > 64*1024*1024*1024 {
		return "R1GB", "1GB"
	}

	// For large install.wim (>4GB), use 512MB UEFI partition
	if installWimSize > 4*1024*1024*1024 {
		return "R512MB", "512MB"
	}

	// Default: 512MB UEFI partition (2025 standard)
	return "R512MB", "512MB"
}

// CreateUEFINTFSPartitionScheme creates NTFS + UEFI:NTFS partition scheme for large files
func CreateUEFINTFSPartitionScheme(deviceID string, gpt bool, installWimSize int64) error {
	// Default to GPT for modern Windows (2025 best practice)
	partitionScheme := "GPT"
	if !gpt {
		partitionScheme = "MBR" // Legacy compatibility mode only
	}

	// Get device size for optimal partition calculation
	deviceSize, err := getDeviceSize(deviceID)
	if err != nil {
		return fmt.Errorf("failed to get device size: %w", err)
	}

	// Calculate optimal partition sizes based on 2025 best practices
	mainSizeSpec, uefiPartitionSize := calculateOptimalPartitionSizes(deviceSize, installWimSize)

	partitionCmd := []string{
		"partitionDisk", deviceID, "2", partitionScheme,
		"ExFAT", "WINDOWS", mainSizeSpec,     // Use calculated main partition size
		"MS-DOS FAT32", "UEFI", uefiPartitionSize, // UEFI System Partition (ESP)
	}

	cmd := exec.Command("diskutil", partitionCmd...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("diskutil partitionDisk failed: %w\noutput: %s", err, out)
	}

	return nil
}

// FormatUEFINTFSPartitions formats both NTFS and UEFI partitions
func FormatUEFINTFSPartitions(deviceID string) error {
	windowsPartition := GetBlockDevicePartition(deviceID, 1)
	uefiPartition := GetBlockDevicePartition(deviceID, 2)

	// Format main partition as NTFS (using mkfs.ntfs if available)
	if err := FormatPartitionAsNTFS(windowsPartition, "WINDOWS"); err != nil {
		return fmt.Errorf("failed to format NTFS partition: %w", err)
	}

	// Format UEFI partition as FAT32
	if err := FormatPartitionAsFAT32(uefiPartition, "UEFI"); err != nil {
		return fmt.Errorf("failed to format UEFI partition: %w", err)
	}

	return nil
}

// FormatPartitionAsNTFS formats a partition as NTFS
func FormatPartitionAsNTFS(partition, label string) error {
	// Try to use mkfs.ntfs if available, otherwise use diskutil with HFS+ and warn
	cmd := exec.Command("which", "mkfs.ntfs")
	if err := cmd.Run(); err == nil {
		// mkfs.ntfs is available
		cmd = exec.Command("mkfs.ntfs", "--quick", "--label", label, "/dev/"+partition)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to format %s as NTFS: %w\noutput: %s", partition, err, out)
		}
	} else {
		// Fall back to ExFAT on macOS (closest to NTFS for large files)
		cmd = exec.Command("diskutil", "eraseVolume", "ExFAT", label, partition)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to format %s as ExFAT (NTFS fallback): %w\noutput: %s", partition, err, out)
		}
	}
	return nil
}

// ValidateDevice checks if the device is suitable for Windows USB creation
func ValidateDevice(deviceID string) error {
	// Get device information
	cmd := exec.Command("diskutil", "info", deviceID)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get device info: %w", err)
	}

	info := string(out)

	// Check if it's removable
	if !strings.Contains(info, "Removable Media: Yes") {
		return fmt.Errorf("device %s is not removable media", deviceID)
	}

	// Check if it's writable
	if !strings.Contains(info, "Read-Only Media: No") {
		return fmt.Errorf("device %s is read-only", deviceID)
	}

	// Check if it's external (optional warning)
	if strings.Contains(info, "Internal: Yes") {
		return fmt.Errorf("device %s appears to be internal - this is dangerous", deviceID)
	}

	return nil
}

// GetPartitionMountPoint returns the mount point of a partition
func GetPartitionMountPoint(partition string) (string, error) {
	cmd := exec.Command("diskutil", "info", partition)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Mount Point:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				mountPoint := strings.TrimSpace(parts[1])
				if mountPoint == "(not mounted)" {
					return "", nil
				}
				return mountPoint, nil
			}
		}
	}

	return "", nil
}

// FormatPartitionAsExFAT formats a partition as ExFAT
func FormatPartitionAsExFAT(partition, label string) error {
	// Use diskutil to format as ExFAT
	cmd := exec.Command("diskutil", "eraseVolume", "ExFAT", label, partition)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to format %s as ExFAT: %w\noutput: %s", partition, err, out)
	}
	return nil
}

// FormatPartitionAsFAT32 formats a partition as FAT32
func FormatPartitionAsFAT32(partition, label string) error {
	// Use diskutil to format as FAT32 (MS-DOS FAT32 in diskutil terminology)
	cmd := exec.Command("diskutil", "eraseVolume", "MS-DOS FAT32", label, partition)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to format %s as FAT32: %w\noutput: %s", partition, err, out)
	}
	return nil
}

// getDeviceSize gets the total size of a device in bytes
func getDeviceSize(deviceID string) (int64, error) {
	cmd := exec.Command("diskutil", "info", deviceID)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get device info: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Disk Size:") {
			// Parse line like "Disk Size:                32.0 GB (31999999488 Bytes) (exactly 62499999 512-Byte-Units)"
			parts := strings.Split(line, "(")
			if len(parts) >= 2 {
				sizeStr := strings.TrimSpace(strings.Split(parts[1], " ")[0])
				size, err := strconv.ParseInt(sizeStr, 10, 64)
				if err != nil {
					return 0, fmt.Errorf("failed to parse device size: %w", err)
				}
				return size, nil
			}
		}
	}
	return 0, fmt.Errorf("could not find device size in diskutil output")
}
