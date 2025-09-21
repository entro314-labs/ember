package device

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/entro314-labs/ember/internal/logger"
	"github.com/entro314-labs/ember/internal/types"
)

// AdvancedPartitionManager handles sophisticated partitioning operations
type AdvancedPartitionManager struct {
	device           string
	partitionTable   string
	alignment        string
	verifyOperations bool
}

// PartitionConfig defines partition creation parameters
type PartitionConfig struct {
	Number      int
	Type        string // primary, logical, extended
	Filesystem  string // fat32, ntfs, fat16
	Start       string // sector or size (e.g., "4MiB", "2048s")
	End         string // sector or size (e.g., "100%", "-2049s")
	Label       string
	Flags       []string // boot, esp, etc.
	Alignment   string   // optimal, minimal, none
}

// NewAdvancedPartitionManager creates a new partition manager
func NewAdvancedPartitionManager(device string) *AdvancedPartitionManager {
	return &AdvancedPartitionManager{
		device:           device,
		partitionTable:   "msdos", // Default to MBR for maximum compatibility
		alignment:        "optimal",
		verifyOperations: true,
	}
}

// WipeDevice completely wipes the device and verifies the operation
func (pm *AdvancedPartitionManager) WipeDevice() error {
	logger.GetLogger().Info("Wiping all existing partition table and filesystem signatures", "device", pm.device)

	// Step 1: Wipe filesystem signatures
	err := pm.wipeFilesystemSignatures()
	if err != nil {
		return fmt.Errorf("failed to wipe filesystem signatures: %w", err)
	}

	// Step 2: Verify the device is actually wiped
	err = pm.verifyDeviceIsWiped()
	if err != nil {
		return fmt.Errorf("device wipe verification failed: %w", err)
	}

	logger.GetLogger().Info("Device wiped successfully")
	return nil
}

// wipeFilesystemSignatures removes all filesystem and partition signatures
func (pm *AdvancedPartitionManager) wipeFilesystemSignatures() error {
	// Use wipefs to remove all signatures
	cmd := exec.Command("wipefs", "--all", pm.device)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wipefs failed: %w\nOutput: %s", err, string(output))
	}

	if len(output) > 0 {
		logger.GetLogger().Info("Wipefs output", "output", strings.TrimSpace(string(output)))
	}

	// Additional zero-out of first and last few sectors for stubborn devices
	err = pm.zeroOutCriticalSectors()
	if err != nil {
		logger.GetLogger().Warn("Failed to zero out critical sectors", "error", err)
		// Continue anyway, wipefs is usually sufficient
	}

	return nil
}

// zeroOutCriticalSectors zeros out the first and last sectors for stubborn devices
func (pm *AdvancedPartitionManager) zeroOutCriticalSectors() error {
	// Zero out first 10MB (usually more than enough for any boot sector/partition table)
	cmd := exec.Command("dd", "if=/dev/zero", "of="+pm.device, "bs=1M", "count=10", "conv=notrunc", "status=none")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to zero beginning of device: %w", err)
	}

	// Get device size to zero out the end
	sizeCmd := exec.Command("blockdev", "--getsize64", pm.device)
	sizeOutput, err := sizeCmd.Output()
	if err == nil {
		sizeStr := strings.TrimSpace(string(sizeOutput))
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
			// Zero out last 10MB
			skipBlocks := (size / (1024 * 1024)) - 10
			if skipBlocks > 0 {
				cmd = exec.Command("dd", "if=/dev/zero", "of="+pm.device, "bs=1M", "count=10",
					"seek="+strconv.FormatInt(skipBlocks, 10), "conv=notrunc", "status=none")
				cmd.Run() // Ignore errors for end-of-device writes
			}
		}
	}

	return nil
}

// verifyDeviceIsWiped checks that the device was actually wiped
func (pm *AdvancedPartitionManager) verifyDeviceIsWiped() error {
	logger.GetLogger().Info("Verifying device is completely wiped...")

	// Check that no partitions are detected
	cmd := exec.Command("lsblk", "--pairs", "--output", "NAME,TYPE", pm.device)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("lsblk verification failed: %w", err)
	}

	// Count partition entries
	lines := strings.Split(string(output), "\n")
	partitionCount := 0
	for _, line := range lines {
		if strings.Contains(line, `TYPE="part"`) {
			partitionCount++
		}
	}

	if partitionCount > 0 {
		return fmt.Errorf("device still contains %d partition(s) after wiping - device may be write-protected or at end of life", partitionCount)
	}

	// Additional check with parted
	cmd = exec.Command("parted", "--script", pm.device, "print")
	output, err = cmd.Output()
	if err == nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "Partition Table: ") && !strings.Contains(outputStr, "Partition Table: unknown") {
			logger.GetLogger().Warn("Parted still detects a partition table, but lsblk shows no partitions - proceeding")
		}
	}

	logger.GetLogger().Info("Device wipe verification successful")
	return nil
}

// CreatePartitionTable creates a new partition table with proper alignment
func (pm *AdvancedPartitionManager) CreatePartitionTable(tableType string) error {
	logger.GetLogger().Info("Creating partition table", "type", tableType, "device", pm.device)

	var partedTableType string
	switch strings.ToLower(tableType) {
	case "msdos", "mbr", "legacy":
		partedTableType = "msdos"
		pm.partitionTable = "msdos"
	case "gpt", "guid":
		partedTableType = "gpt"
		pm.partitionTable = "gpt"
	default:
		return fmt.Errorf("unsupported partition table type: %s", tableType)
	}

	// Create the partition table
	cmd := exec.Command("parted", "--script", pm.device, "mklabel", partedTableType)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create partition table: %w\nOutput: %s", err, string(output))
	}

	// Verify partition table creation
	err = pm.verifyPartitionTable(partedTableType)
	if err != nil {
		return fmt.Errorf("partition table verification failed: %w", err)
	}

	logger.GetLogger().Info("Partition table created successfully")
	return nil
}

// CreatePartition creates a partition with advanced options
func (pm *AdvancedPartitionManager) CreatePartition(config *PartitionConfig) error {
	logger.GetLogger().Info("Creating partition",
		"number", config.Number,
		"type", config.Type,
		"filesystem", config.Filesystem,
		"start", config.Start,
		"end", config.End)

	// Validate configuration
	err := pm.validatePartitionConfig(config)
	if err != nil {
		return fmt.Errorf("invalid partition configuration: %w", err)
	}

	// Build parted command for partition creation
	args := []string{"--script"}

	// Set alignment if specified
	if config.Alignment != "" {
		args = append(args, "--align", config.Alignment)
	} else if pm.alignment != "" {
		args = append(args, "--align", pm.alignment)
	}

	args = append(args, pm.device, "mkpart", config.Type)

	// Add filesystem type for MBR
	if pm.partitionTable == "msdos" && config.Filesystem != "" {
		args = append(args, config.Filesystem)
	}

	args = append(args, config.Start, config.End)

	// Create the partition
	cmd := exec.Command("parted", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create partition: %w\nOutput: %s", err, string(output))
	}

	// Wait for system to recognize the partition
	err = makeSystemRealizePartitionTableChanged(pm.device)
	if err != nil {
		return fmt.Errorf("failed to refresh partition table: %w", err)
	}

	// Set partition flags if specified
	if len(config.Flags) > 0 {
		err = pm.setPartitionFlags(config.Number, config.Flags)
		if err != nil {
			logger.GetLogger().Warn("Failed to set partition flags", "error", err)
			// Continue anyway, flags are not always critical
		}
	}

	// Verify partition creation
	if pm.verifyOperations {
		err = pm.verifyPartitionCreation(config.Number)
		if err != nil {
			return fmt.Errorf("partition verification failed: %w", err)
		}
	}

	logger.GetLogger().Info("Partition created successfully", "number", config.Number)
	return nil
}

// validatePartitionConfig validates partition configuration parameters
func (pm *AdvancedPartitionManager) validatePartitionConfig(config *PartitionConfig) error {
	if config.Number <= 0 {
		return fmt.Errorf("partition number must be positive")
	}

	if config.Type == "" {
		config.Type = "primary" // Default to primary
	}

	validTypes := []string{"primary", "logical", "extended"}
	if !contains(validTypes, config.Type) {
		return fmt.Errorf("invalid partition type: %s", config.Type)
	}

	if config.Start == "" {
		return fmt.Errorf("partition start position required")
	}

	if config.End == "" {
		return fmt.Errorf("partition end position required")
	}

	return nil
}

// setPartitionFlags sets flags on a partition (boot, esp, etc.)
func (pm *AdvancedPartitionManager) setPartitionFlags(partNumber int, flags []string) error {
	for _, flag := range flags {
		logger.GetLogger().Info("Setting partition flag", "flag", flag, "partition", partNumber)

		cmd := exec.Command("parted", "--script", pm.device, "set",
			strconv.Itoa(partNumber), flag, "on")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to set flag %s: %w\nOutput: %s", flag, err, string(output))
		}
	}

	return nil
}

// FormatPartition formats a partition with the specified filesystem
func (pm *AdvancedPartitionManager) FormatPartition(partNumber int, fsType, label string) error {
	partitionDevice := fmt.Sprintf("%s%d", pm.device, partNumber)
	logger.GetLogger().Info("Formatting partition", "device", partitionDevice, "filesystem", fsType, "label", label)

	var cmd *exec.Cmd
	switch strings.ToLower(fsType) {
	case "fat32", "vfat":
		// Use optimal cluster size for FAT32
		cmd = exec.Command("mkfs.fat", "-F", "32", "-n", label, partitionDevice)
	case "ntfs":
		// Quick format with label
		cmd = exec.Command("mkfs.ntfs", "--quick", "--label", label, partitionDevice)
	case "fat16":
		cmd = exec.Command("mkfs.fat", "-F", "16", "-n", label, partitionDevice)
	case "exfat":
		cmd = exec.Command("mkfs.exfat", "-n", label, partitionDevice)
	default:
		return fmt.Errorf("unsupported filesystem type: %s", fsType)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to format partition: %w\nOutput: %s", err, string(output))
	}

	logger.GetLogger().Info("Partition formatted successfully")
	return nil
}

// CreateOptimalWindowsUSBLayout creates the optimal partition layout for Windows USB
func (pm *AdvancedPartitionManager) CreateOptimalWindowsUSBLayout(analysis *types.FilesystemAnalysis, label string) error {
	logger.GetLogger().Info("Creating optimal Windows USB partition layout using native macOS tools")

	// Use the existing working macOS-native implementation
	deviceID := strings.TrimPrefix(pm.device, "/dev/")
	useGPT := pm.partitionTable == "gpt"

	if analysis.RequiresUEFI_NTFS {
		logger.GetLogger().Info("Creating UEFI:NTFS partition scheme for large files")
		err := CreateUEFINTFSPartitionScheme(deviceID, useGPT, analysis.InstallWimSize)
		if err != nil {
			return fmt.Errorf("failed to create UEFI:NTFS partition scheme: %w", err)
		}

		// Format both partitions
		return FormatUEFINTFSPartitions(deviceID)
	} else {
		logger.GetLogger().Info("Creating standard FAT32 partition scheme")
		return FormatDiskForUEFINTFS(deviceID, useGPT)
	}
}

// verifyPartitionTable verifies that the partition table was created correctly
func (pm *AdvancedPartitionManager) verifyPartitionTable(expectedType string) error {
	cmd := exec.Command("parted", "--script", pm.device, "print")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to read partition table: %w", err)
	}

	outputStr := string(output)
	expectedLine := fmt.Sprintf("Partition Table: %s", expectedType)
	if !strings.Contains(outputStr, expectedLine) {
		return fmt.Errorf("partition table type verification failed: expected %s", expectedType)
	}

	return nil
}

// verifyPartitionCreation verifies that a partition was created correctly
func (pm *AdvancedPartitionManager) verifyPartitionCreation(partNumber int) error {
	partitionDevice := fmt.Sprintf("%s%d", pm.device, partNumber)

	// Check if partition device exists
	if _, err := os.Stat(partitionDevice); err != nil {
		return fmt.Errorf("partition device %s not found: %w", partitionDevice, err)
	}

	// Verify with lsblk
	cmd := exec.Command("lsblk", "--output", "NAME,TYPE", "--noheadings", pm.device)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("lsblk verification failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	partitionFound := false
	expectedPartName := fmt.Sprintf("%s%d", strings.TrimPrefix(pm.device, "/dev/"), partNumber)

	for _, line := range lines {
		if strings.Contains(line, expectedPartName) && strings.Contains(line, "part") {
			partitionFound = true
			break
		}
	}

	if !partitionFound {
		return fmt.Errorf("partition %d not found in lsblk output", partNumber)
	}

	return nil
}

// makeSystemRealizePartitionTableChanged forces the system to re-read partition table
func makeSystemRealizePartitionTableChanged(device string) error {
	logger.GetLogger().Info("Refreshing partition table", "device", device)

	// Use blockdev to re-read partition table
	cmd := exec.Command("blockdev", "--rereadpt", device)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.GetLogger().Warn("blockdev --rereadpt failed", "error", err, "output", string(output))
		// Continue anyway, partprobe might work
	}

	// Try partprobe as alternative
	if _, err := exec.LookPath("partprobe"); err == nil {
		cmd = exec.Command("partprobe", device)
		cmd.Run() // Ignore errors
	}

	// Wait for device nodes to appear
	logger.GetLogger().Info("Waiting for device nodes to populate...")
	time.Sleep(3 * time.Second)

	// Verify device nodes exist
	return verifyDeviceNodes(device)
}

// verifyDeviceNodes checks that all expected device nodes exist
func verifyDeviceNodes(device string) error {
	// Get list of partitions from lsblk
	cmd := exec.Command("lsblk", "--output", "NAME", "--noheadings", "--raw", device)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list partitions: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	deviceBase := strings.TrimPrefix(device, "/dev/")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, deviceBase) && len(line) > len(deviceBase) {
			partitionDevice := "/dev/" + line
			if _, err := os.Stat(partitionDevice); err != nil {
				return fmt.Errorf("partition device %s not accessible: %w", partitionDevice, err)
			}
		}
	}

	return nil
}

// GetPartitionInfo returns detailed information about the device partitions
func (pm *AdvancedPartitionManager) GetPartitionInfo() (map[string]interface{}, error) {
	cmd := exec.Command("lsblk", "--json", "--output", "NAME,SIZE,TYPE,FSTYPE,LABEL,MOUNTPOINT", pm.device)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get partition info: %w", err)
	}

	// Also get parted output for additional details
	partedCmd := exec.Command("parted", "--script", pm.device, "print")
	partedOutput, err := partedCmd.Output()
	if err != nil {
		logger.GetLogger().Warn("Failed to get parted output", "error", err)
	}

	return map[string]interface{}{
		"device":         pm.device,
		"partition_table": pm.partitionTable,
		"lsblk_output":   string(output),
		"parted_output":  string(partedOutput),
		"alignment":      pm.alignment,
	}, nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}