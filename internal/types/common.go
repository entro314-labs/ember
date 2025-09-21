package types

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FilesystemType represents the target filesystem type
type FilesystemType string

const (
	FilesystemFAT32 FilesystemType = "FAT32"
	FilesystemNTFS  FilesystemType = "NTFS"
	FilesystemExFAT FilesystemType = "ExFAT"
)

// SafetyLevel indicates how safe it is to write to a device
type SafetyLevel int

const (
	SafetyGreen  SafetyLevel = iota // Safe: Empty USB, previously used for Windows
	SafetyYellow                    // Caution: Contains data but appears to be non-critical
	SafetyRed                       // Dangerous: Contains important data, internal drive, etc.
)

// OperationStatus represents the status of an operation
type OperationStatus int

const (
	StatusPending OperationStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusCanceled
)

// ISOInfo contains metadata about the Windows ISO
type ISOInfo struct {
	Path             string
	Size             int64
	Version          string
	Architecture     string
	Edition          string
	Language         string
	BuildNumber      string
	IsUEFI           bool
	HasInstallWim    bool
	Source           string        // Discovery source: "Downloads", "Desktop", etc.
	LastModified     time.Time
	RecommendedFS    FilesystemType
	RequiresWorkaround bool
	EstimatedUSBSize int64
}

// DiskUtilDevice represents a disk device from diskutil
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

	// Enhanced safety and analysis fields
	SafetyLevel      SafetyLevel
	USBSpeed         string
	FreeSpace        int64
	LastUsedFor      string
	IsEmberCreated   bool
	EstimatedSpeed   int64 // bytes per second
	ContainsBackups  bool
}

// FilesystemAnalysis contains the results of source media analysis
type FilesystemAnalysis struct {
	RecommendedFS         FilesystemType
	RequiresUEFI_NTFS     bool
	RequiresDualPartition bool    // 2025: Dual-partition scheme needed for large install.wim
	LargeFiles            []string
	MaxFileSize           int64
	TotalSize             int64
	FileCount             int
	HasWIMFiles           bool
	HasInstallWIM         bool
	InstallWimSize        int64   // 2025: Size of install.wim file specifically
	WindowsVersion        string
	RequiresWorkarounds   []string
}

// FileProgressCallback defines the signature for progress reporting during file operations
type FileProgressCallback func(current, total int64, currentFile string)

// FormatBytes returns a human-readable size string for any byte count
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	size := float64(bytes)
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
		return fmt.Sprintf("%d B", bytes)
	}
}

// GetSizeString returns a human-readable size string for the ISO
func (iso *ISOInfo) GetSizeString() string {
	return FormatBytes(iso.Size)
}

// DiskUtilDevice methods

// GetDisplayName returns a human-readable name for the device
func (d *DiskUtilDevice) GetDisplayName() string {
	name := d.DeviceIdentifier

	if d.VolumeName != "" {
		name = fmt.Sprintf("%s (%s)", d.VolumeName, d.DeviceIdentifier)
	} else if d.MediaName != "" {
		name = fmt.Sprintf("%s (%s)", d.MediaName, d.DeviceIdentifier)
	}

	return name
}

// GetSizeString returns a human-readable size string
func (d *DiskUtilDevice) GetSizeString() string {
	return FormatBytes(d.Size)
}

// GetDeviceDescription returns a detailed description of the device
func (d *DiskUtilDevice) GetDeviceDescription() string {
	parts := []string{
		d.GetSizeString(),
	}

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

// GetSafetyColor returns a color code for TUI display
func (d *DiskUtilDevice) GetSafetyColor() string {
	switch d.SafetyLevel {
	case SafetyGreen:
		return "green"
	case SafetyYellow:
		return "yellow"
	case SafetyRed:
		return "red"
	default:
		return "white"
	}
}

// GetSafetyIcon returns an emoji icon for the safety level
func (d *DiskUtilDevice) GetSafetyIcon() string {
	switch d.SafetyLevel {
	case SafetyGreen:
		return "🟢"
	case SafetyYellow:
		return "🟡"
	case SafetyRed:
		return "🔴"
	default:
		return "⚪"
	}
}

// GetSafetyMessage returns a descriptive safety message
func (d *DiskUtilDevice) GetSafetyMessage() string {
	switch d.SafetyLevel {
	case SafetyGreen:
		if d.IsEmberCreated {
			return "Previously used for Windows USB - safe to overwrite"
		}
		return "Empty or safe to overwrite"
	case SafetyYellow:
		return "Contains data - will be erased"
	case SafetyRed:
		return "Contains important data - use with extreme caution"
	default:
		return "Unknown safety level"
	}
}

// GetSpeedEstimate returns formatted speed estimate
func (d *DiskUtilDevice) GetSpeedEstimate(sizeGB float64) string {
	if d.EstimatedSpeed == 0 {
		return "Unknown"
	}

	timeSeconds := (sizeGB * 1024 * 1024 * 1024) / float64(d.EstimatedSpeed)
	if timeSeconds < 60 {
		return fmt.Sprintf("~%.0fs", timeSeconds)
	} else if timeSeconds < 3600 {
		return fmt.Sprintf("~%.0fm", timeSeconds/60)
	} else {
		return fmt.Sprintf("~%.1fh", timeSeconds/3600)
	}
}

// AnalyzeDeviceSafety performs comprehensive safety analysis on a device
func (d *DiskUtilDevice) AnalyzeDeviceSafety() {
	// Start with yellow (caution) as default
	d.SafetyLevel = SafetyYellow

	// Determine USB speed
	d.USBSpeed = determineUSBSpeed(d.BusProtocol)
	d.EstimatedSpeed = estimateTransferSpeed(d.BusProtocol)

	// Calculate free space
	if d.VolumeSize > 0 {
		d.FreeSpace = d.VolumeSize
		// For mounted drives, get actual free space
		if d.MountPoint != "" {
			if actualFree := getActualFreeSpace(d.MountPoint); actualFree > 0 {
				d.FreeSpace = actualFree
			}
		}
	} else {
		d.FreeSpace = d.Size // Assume entire device is free if no volume
	}

	// Check for Ember-created USBs
	d.IsEmberCreated = checkIfEmberCreated(d)

	// Check for backup data
	d.ContainsBackups = checkForBackups(d)

	// Determine what it was last used for
	d.LastUsedFor = determineLastUse(d)

	// Determine safety level based on analysis
	d.SafetyLevel = calculateSafetyLevel(d)
}

// Helper functions for device analysis

// determineUSBSpeed returns human-readable USB speed
func determineUSBSpeed(busProtocol string) string {
	protocol := strings.ToLower(busProtocol)
	switch {
	case strings.Contains(protocol, "usb 3"):
		return "USB 3.0+"
	case strings.Contains(protocol, "usb 2"):
		return "USB 2.0"
	case strings.Contains(protocol, "usb-c"):
		return "USB-C"
	case strings.Contains(protocol, "usb"):
		return "USB"
	default:
		return "Unknown"
	}
}

// estimateTransferSpeed returns estimated bytes per second
func estimateTransferSpeed(busProtocol string) int64 {
	protocol := strings.ToLower(busProtocol)
	switch {
	case strings.Contains(protocol, "usb 3"):
		return 100 * 1024 * 1024 // ~100 MB/s for USB 3.0
	case strings.Contains(protocol, "usb 2"):
		return 25 * 1024 * 1024  // ~25 MB/s for USB 2.0
	case strings.Contains(protocol, "usb-c"):
		return 200 * 1024 * 1024 // ~200 MB/s for USB-C
	default:
		return 10 * 1024 * 1024  // ~10 MB/s conservative estimate
	}
}

// getActualFreeSpace gets actual free space for mounted volumes
func getActualFreeSpace(mountPoint string) int64 {
	// Use os.Stat to get filesystem information
	if stat, err := os.Stat(mountPoint); err == nil && stat.IsDir() {
		// Try to read the directory to see if it's accessible
		if entries, err := os.ReadDir(mountPoint); err == nil {
			// Directory is accessible, but we can't get exact free space
			// without platform-specific code. Return 0 for now.
			_ = entries
		}
	}
	return 0
}

// checkIfEmberCreated checks if this USB was created by Ember
func checkIfEmberCreated(device *DiskUtilDevice) bool {
	// Check for Ember marker files
	if device.MountPoint == "" {
		return false
	}

	emberMarkers := []string{
		".ember_created",
		"ember.log",
		"bootmgr", // Windows bootloader
	}

	for _, marker := range emberMarkers {
		markerPath := filepath.Join(device.MountPoint, marker)
		if _, err := os.Stat(markerPath); err == nil {
			return true
		}
	}

	// Check volume label
	if strings.Contains(strings.ToLower(device.VolumeName), "windows") ||
	   strings.Contains(strings.ToLower(device.VolumeName), "ember") {
		return true
	}

	return false
}

// checkForBackups checks if the device contains backup data
func checkForBackups(device *DiskUtilDevice) bool {
	if device.MountPoint == "" {
		return false
	}

	backupIndicators := []string{
		"backup",
		"time machine",
		".backups",
		"documents",
		"photos",
		"music",
		"videos",
	}

	// Check volume name
	volumeName := strings.ToLower(device.VolumeName)
	for _, indicator := range backupIndicators {
		if strings.Contains(volumeName, indicator) {
			return true
		}
	}

	// Check for common backup directories
	backupDirs := []string{
		"Backups.backupdb",
		"Documents",
		"Photos",
		"Music",
		"Videos",
		".backup",
	}

	for _, dir := range backupDirs {
		dirPath := filepath.Join(device.MountPoint, dir)
		if stat, err := os.Stat(dirPath); err == nil && stat.IsDir() {
			return true
		}
	}

	return false
}

// determineLastUse determines what the device was last used for
func determineLastUse(device *DiskUtilDevice) string {
	if device.IsEmberCreated {
		return "Windows USB (Ember)"
	}

	if device.ContainsBackups {
		return "Backup Storage"
	}

	if device.MountPoint != "" {
		// Check for common file types
		entries, err := os.ReadDir(device.MountPoint)
		if err == nil && len(entries) == 0 {
			return "Empty"
		}

		if err == nil {
			hasDocuments := false
			hasMedia := false
			hasSystem := false

			for _, entry := range entries[:min(10, len(entries))] { // Check first 10 files
				name := strings.ToLower(entry.Name())
				switch {
				case strings.HasSuffix(name, ".doc") || strings.HasSuffix(name, ".pdf") ||
				     strings.HasSuffix(name, ".txt"):
					hasDocuments = true
				case strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".mp4") ||
				     strings.HasSuffix(name, ".mp3"):
					hasMedia = true
				case name == "bootmgr" || name == "boot" || strings.HasSuffix(name, ".efi"):
					hasSystem = true
				}
			}

			switch {
			case hasSystem:
				return "System/Boot Files"
			case hasDocuments:
				return "Documents"
			case hasMedia:
				return "Media Files"
			default:
				return "General Storage"
			}
		}
	}

	return "Unknown"
}

// calculateSafetyLevel determines the overall safety level
func calculateSafetyLevel(device *DiskUtilDevice) SafetyLevel {
	// Start with green (safe)
	safety := SafetyGreen

	// Check if internal drive (should never happen due to filtering, but safety first)
	if device.Internal {
		return SafetyRed
	}

	// If it contains backups, it's dangerous
	if device.ContainsBackups {
		return SafetyRed
	}

	// If it's not empty and not Ember-created, it's at least yellow
	if device.LastUsedFor != "Empty" && !device.IsEmberCreated {
		safety = SafetyYellow
	}

	// If it contains important files, it's red
	if device.LastUsedFor == "Documents" || device.LastUsedFor == "Backup Storage" {
		safety = SafetyRed
	}

	// If it's Ember-created or empty, it's safe
	if device.IsEmberCreated || device.LastUsedFor == "Empty" {
		safety = SafetyGreen
	}

	return safety
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}