package device

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/entro314-labs/ember/internal/logger"
	"github.com/entro314-labs/ember/internal/types"
)

//go:embed uefi-ntfs.img
var uefiNtfsImage []byte

// UEFINTFSSupport handles UEFI:NTFS partition creation and management
type UEFINTFSSupport struct {
	device     string
	partNumber int
	mounted    bool
}

// CreateUEFINTFSPartition creates a UEFI:NTFS support partition
func CreateUEFINTFSPartition(device string) (*UEFINTFSSupport, error) {
	logger.GetLogger().Info("Creating UEFI:NTFS support partition", "device", device)

	// Create the UEFI:NTFS partition (512KB at the end of the device)
	// This partition enables UEFI booting from NTFS filesystems
	err := createUEFINTFSPartitionGeometry(device)
	if err != nil {
		return nil, fmt.Errorf("failed to create UEFI:NTFS partition geometry: %w", err)
	}

	// Determine the partition number (should be 2 for typical setup)
	partNumber, err := getLastPartitionNumber(device)
	if err != nil {
		return nil, fmt.Errorf("failed to determine UEFI:NTFS partition number: %w", err)
	}

	uefiSupport := &UEFINTFSSupport{
		device:     device,
		partNumber: partNumber,
		mounted:    false,
	}

	// Install the UEFI:NTFS bootloader
	err = uefiSupport.InstallBootloader()
	if err != nil {
		return nil, fmt.Errorf("failed to install UEFI:NTFS bootloader: %w", err)
	}

	logger.GetLogger().Info("UEFI:NTFS support partition created successfully")
	return uefiSupport, nil
}

// createUEFINTFSPartitionGeometry creates the partition structure
func createUEFINTFSPartitionGeometry(device string) error {
	// The UEFI:NTFS partition is a small FAT16 partition at the end of the device
	// It contains the UEFI:NTFS bootloader that can chainload from NTFS partitions

	cmd := exec.Command("parted",
		"--align", "none",  // Misaligned is OK for this small partition
		"--script",
		device,
		"mkpart",
		"primary",
		"fat16",
		"--", "-2048s", "-1s") // Last 1MB of device (2048 sectors)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("parted failed: %w\nOutput: %s", err, string(output))
	}

	// Wait for the system to recognize the new partition
	err = makeSystemRealizePartitionTableChanged(device)
	if err != nil {
		return fmt.Errorf("failed to refresh partition table: %w", err)
	}

	return nil
}

// InstallBootloader writes the UEFI:NTFS image to the partition
func (u *UEFINTFSSupport) InstallBootloader() error {
	partitionDevice := fmt.Sprintf("%s%d", u.device, u.partNumber)
	logger.GetLogger().Info("Installing UEFI:NTFS bootloader", "device", partitionDevice)

	// Write the embedded UEFI:NTFS image directly to the partition
	err := writeImageToPartition(uefiNtfsImage, partitionDevice)
	if err != nil {
		return fmt.Errorf("failed to write UEFI:NTFS image: %w", err)
	}

	// Set the partition type and flags for UEFI compatibility
	err = u.configurePartitionAttributes()
	if err != nil {
		logger.GetLogger().Warn("Failed to set partition attributes", "error", err)
		// Continue anyway, as this is not critical
	}

	logger.GetLogger().Info("UEFI:NTFS bootloader installed successfully")
	return nil
}

// writeImageToPartition writes binary data to a partition device
func writeImageToPartition(imageData []byte, partitionDevice string) error {
	// Open the partition device for writing
	file, err := os.OpenFile(partitionDevice, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open partition device %s: %w", partitionDevice, err)
	}
	defer file.Close()

	// Write the image data
	written, err := file.Write(imageData)
	if err != nil {
		return fmt.Errorf("failed to write image data: %w", err)
	}

	if written != len(imageData) {
		return fmt.Errorf("incomplete write: wrote %d bytes, expected %d", written, len(imageData))
	}

	// Sync to ensure data is written
	err = file.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync partition: %w", err)
	}

	return nil
}

// configurePartitionAttributes sets proper partition type and flags
func (u *UEFINTFSSupport) configurePartitionAttributes() error {
	_ = fmt.Sprintf("%s%d", u.device, u.partNumber)

	// Set partition type to EFI System Partition (ESP) for better UEFI compatibility
	cmd := exec.Command("parted", "--script", u.device, "set", strconv.Itoa(u.partNumber), "esp", "on")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.GetLogger().Warn("Failed to set ESP flag", "error", err, "output", string(output))
		// Try alternative approach with sgdisk if available
		return u.setPartitionTypeWithSgdisk()
	}

	return nil
}

// setPartitionTypeWithSgdisk uses sgdisk as fallback for partition type setting
func (u *UEFINTFSSupport) setPartitionTypeWithSgdisk() error {
	// Check if sgdisk is available
	if _, err := exec.LookPath("sgdisk"); err != nil {
		return nil // Skip if sgdisk not available
	}

	// Set partition type to EFI System Partition (C12A7328-F81F-11D2-BA4B-00A0C93EC93B)
	cmd := exec.Command("sgdisk",
		"--typecode", fmt.Sprintf("%d:EF00", u.partNumber), // EF00 = EFI System Partition
		u.device)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.GetLogger().Warn("Failed to set partition type with sgdisk", "error", err, "output", string(output))
	}

	return nil
}

// getLastPartitionNumber returns the number of the last partition on the device
func getLastPartitionNumber(device string) (int, error) {
	cmd := exec.Command("lsblk", "--output", "NAME", "--noheadings", "--raw", device)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("lsblk failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	maxPartNum := 0

	deviceBase := filepath.Base(device)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, deviceBase) && len(line) > len(deviceBase) {
			// Extract partition number
			suffix := line[len(deviceBase):]
			if num, err := strconv.Atoi(suffix); err == nil && num > maxPartNum {
				maxPartNum = num
			}
		}
	}

	if maxPartNum == 0 {
		return 0, fmt.Errorf("no partitions found on device %s", device)
	}

	return maxPartNum, nil
}

// IsUEFINTFSRequired determines if UEFI:NTFS support is needed
func IsUEFINTFSRequired(analysis *types.FilesystemAnalysis) bool {
	// UEFI:NTFS is required when:
	// 1. Using NTFS filesystem (for large file support)
	// 2. Need UEFI booting support
	// 3. Target firmware might not support NTFS natively

	return analysis.RecommendedFS == types.FilesystemNTFS || len(analysis.LargeFiles) > 0
}

// CheckUEFINTFSSupport verifies if the current system can create UEFI:NTFS partitions
func CheckUEFINTFSSupport() error {
	// Check for required tools (platform-specific)
	var requiredTools []string
	if runtime.GOOS == "darwin" {
		requiredTools = []string{"diskutil"}
	} else {
		requiredTools = []string{"parted", "lsblk"}
	}

	for _, tool := range requiredTools {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("required tool %s not found in PATH", tool)
		}
	}

	// Verify we have the embedded UEFI:NTFS image
	if len(uefiNtfsImage) == 0 {
		return fmt.Errorf("UEFI:NTFS image not embedded")
	}

	// Check image size (should be exactly 1MB)
	expectedSize := 1024 * 1024 // 1MB
	if len(uefiNtfsImage) != expectedSize {
		logger.GetLogger().Warn("UEFI:NTFS image size unexpected", "got", len(uefiNtfsImage), "expected", expectedSize)
	}

	logger.GetLogger().Info("UEFI:NTFS support available", "image_size", len(uefiNtfsImage))
	return nil
}

// ValidateUEFINTFSPartition checks if the UEFI:NTFS partition was created correctly
func (u *UEFINTFSSupport) ValidateUEFINTFSPartition() error {
	partitionDevice := fmt.Sprintf("%s%d", u.device, u.partNumber)

	// Check if partition exists
	if _, err := os.Stat(partitionDevice); err != nil {
		return fmt.Errorf("UEFI:NTFS partition %s not found: %w", partitionDevice, err)
	}

	// Check partition size (should be approximately 1MB)
	cmd := exec.Command("blockdev", "--getsize64", partitionDevice)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get partition size: %w", err)
	}

	sizeStr := strings.TrimSpace(string(output))
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse partition size: %w", err)
	}

	// Size should be at least 512KB (minimum for UEFI:NTFS) and at most 2MB
	minSize := int64(512 * 1024)    // 512KB
	maxSize := int64(2 * 1024 * 1024) // 2MB

	if size < minSize || size > maxSize {
		return fmt.Errorf("UEFI:NTFS partition size %d bytes is outside expected range (%d-%d)", size, minSize, maxSize)
	}

	logger.GetLogger().Info("UEFI:NTFS partition validation successful", "size", size)
	return nil
}

// CleanupUEFINTFSSupport removes UEFI:NTFS partition if needed
func (u *UEFINTFSSupport) CleanupUEFINTFSSupport() error {
	if u.mounted {
		partitionDevice := fmt.Sprintf("%s%d", u.device, u.partNumber)
		cmd := exec.Command("umount", partitionDevice)
		if err := cmd.Run(); err != nil {
			logger.GetLogger().Warn("Failed to unmount UEFI:NTFS partition", "error", err)
		}
		u.mounted = false
	}

	return nil
}

// GetUEFINTFSInfo returns information about the UEFI:NTFS setup
func (u *UEFINTFSSupport) GetUEFINTFSInfo() map[string]interface{} {
	return map[string]interface{}{
		"device":           u.device,
		"partition_number": u.partNumber,
		"partition_device": fmt.Sprintf("%s%d", u.device, u.partNumber),
		"image_size":       len(uefiNtfsImage),
		"mounted":          u.mounted,
		"purpose":          "Enables UEFI booting from NTFS filesystems",
	}
}