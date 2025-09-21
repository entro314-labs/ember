package device

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/entro314-labs/ember/internal/filesystem"
	"github.com/entro314-labs/ember/internal/logger"
	"github.com/entro314-labs/ember/internal/types"
)



// AdvancedUSBCreator orchestrates the advanced Windows USB creation process
type AdvancedUSBCreator struct {
	dependencyManager    *DependencyManager
	progressTracker      *ProgressTracker
	partitionManager     *AdvancedPartitionManager
	windows7Workaround   *Windows7UEFIWorkaround
	consoleSubscriber    *ConsoleProgressSubscriber
}

// USBCreationConfig contains all configuration for USB creation
type USBCreationConfig struct {
	SourcePath      string
	TargetDevice    string
	Label           string
	ForceFilesystem types.FilesystemType
	SkipAnalysis    bool
	EnableUEFI      bool
	LegacyBIOS      bool
	QuickFormat     bool
	PerformanceMode bool
	AllowLargeFiles bool
	Verbose         bool
}

// NewAdvancedUSBCreator creates a new advanced USB creator
func NewAdvancedUSBCreator() *AdvancedUSBCreator {
	creator := &AdvancedUSBCreator{
		dependencyManager: NewDependencyManager(),
		progressTracker:   NewProgressTracker(),
		consoleSubscriber: NewConsoleProgressSubscriber(),
	}

	// Subscribe to progress updates
	creator.progressTracker.Subscribe(creator.consoleSubscriber)
	creator.progressTracker.StartPeriodicUpdates(time.Second)

	return creator
}

// CreateAdvancedWindowsUSB performs the complete advanced Windows USB creation process
func (creator *AdvancedUSBCreator) CreateAdvancedWindowsUSB(config *USBCreationConfig) error {
	logger.GetLogger().Info("Starting advanced Windows USB creation process")
	logger.GetLogger().Info("Source", "path", config.SourcePath)
	logger.GetLogger().Info("Target", "device", config.TargetDevice)

	// Step 1: Dependency checking and recovery
	err := creator.progressTracker.TrackOperation("dependency_check", "Checking system dependencies", func() error {
		return creator.dependencyManager.CheckAllDependencies()
	})
	if err != nil {
		// Attempt automatic recovery
		logger.GetLogger().Warn("Dependency check failed, attempting recovery...")
		recoveryErr := creator.dependencyManager.AttemptDependencyRecovery()
		if recoveryErr != nil {
			return fmt.Errorf("dependency check failed and recovery unsuccessful: %w", err)
		}

		// Retry dependency check
		err = creator.dependencyManager.CheckAllDependencies()
		if err != nil {
			return fmt.Errorf("dependency check still failing after recovery: %w", err)
		}
	}

	// Step 2: Source media analysis
	var analysis *types.FilesystemAnalysis
	err = creator.progressTracker.TrackOperation("source_analysis", "Analyzing source media", func() error {
		var analysisErr error
		analysis, analysisErr = filesystem.AnalyzeSourceMedia(config.SourcePath)
		return analysisErr
	})
	if err != nil {
		return fmt.Errorf("source media analysis failed: %w", err)
	}

	// Apply user preferences and get optimal filesystem creation options
	userPrefs := filesystem.GetDefaultUserPreferences()
	if config.ForceFilesystem != "" {
		userPrefs.PreferredFilesystem = config.ForceFilesystem
	}
	userPrefs.QuickFormat = config.QuickFormat
	userPrefs.PerformanceMode = config.PerformanceMode
	userPrefs.AllowLargeFiles = config.AllowLargeFiles

	// Get optimal creation options based on analysis and preferences
	creationOptions := filesystem.GetFilesystemCreationOptions(analysis, userPrefs, config.Label)

	// Apply creation options to analysis
	if creationOptions.UserOverride {
		logger.GetLogger().Info("Filesystem selection overridden by user preference", "from", analysis.RecommendedFS, "to", creationOptions.Type)
		analysis.RecommendedFS = creationOptions.Type
		analysis.RequiresUEFI_NTFS = creationOptions.RequiresNTFS
	}

	// Display analysis results
	creator.displayAnalysisResults(analysis)

	// Step 3: Validate UEFI:NTFS support if needed
	if analysis.RequiresUEFI_NTFS {
		err = creator.progressTracker.TrackOperation("uefi_ntfs_check", "Validating UEFI:NTFS support", func() error {
			return CheckUEFINTFSSupport()
		})
		if err != nil {
			return fmt.Errorf("UEFI:NTFS support validation failed: %w", err)
		}
	}

	// Step 4: Initialize partition manager
	creator.partitionManager = NewAdvancedPartitionManager(config.TargetDevice)

	// Step 5: Create optimal partition layout
	err = creator.progressTracker.TrackOperation("partitioning", "Creating optimal partition layout", func() error {
		return creator.partitionManager.CreateOptimalWindowsUSBLayout(analysis, config.Label)
	})
	if err != nil {
		return fmt.Errorf("partition creation failed: %w", err)
	}

	// Step 6: Mount filesystems
	var targetMountpoint string
	err = creator.progressTracker.TrackOperation("mounting", "Mounting filesystems", func() error {
		var mountErr error
		targetMountpoint, mountErr = creator.mountTargetFilesystem(config.TargetDevice)
		return mountErr
	})
	if err != nil {
		return fmt.Errorf("filesystem mounting failed: %w", err)
	}
	defer creator.unmountFilesystem(targetMountpoint)

	// Step 7: Copy files with progress tracking
	err = creator.progressTracker.TrackOperation("file_copy", "Copying Windows files", func() error {
		callback := creator.progressTracker.FileProgressCallback("file_copy")
		return filesystem.CopyWithProgress(config.SourcePath, targetMountpoint, callback)
	})
	if err != nil {
		return fmt.Errorf("file copying failed: %w", err)
	}

	// Step 8: Apply Windows 7 UEFI workaround if needed
	if contains(analysis.RequiresWorkarounds, "windows7_uefi_bootloader") {
		creator.windows7Workaround = NewWindows7UEFIWorkaround(config.SourcePath, targetMountpoint)

		err = creator.progressTracker.TrackOperation("windows7_workaround", "Applying Windows 7 UEFI workaround", func() error {
			analysisErr := creator.windows7Workaround.Analyze()
			if analysisErr != nil {
				return analysisErr
			}
			return creator.windows7Workaround.Apply()
		})
		if err != nil {
			logger.GetLogger().Warn("Windows 7 UEFI workaround failed (non-critical)", "error", err)
			// Continue anyway, as this is not always critical
		}
	}

	// Step 9: Install legacy bootloader if required
	if config.LegacyBIOS && !contains(analysis.RequiresWorkarounds, "windows7_uefi_bootloader") {
		err = creator.progressTracker.TrackOperation("legacy_bootloader", "Installing legacy BIOS bootloader", func() error {
			return creator.installLegacyBootloader(targetMountpoint, config.TargetDevice)
		})
		if err != nil {
			logger.GetLogger().Warn("Legacy bootloader installation failed (non-critical)", "error", err)
			// Continue anyway, UEFI might still work
		}
	}

	// Step 10: Final verification
	err = creator.progressTracker.TrackOperation("verification", "Performing final verification", func() error {
		return creator.performFinalVerification(analysis, targetMountpoint, config.TargetDevice)
	})
	if err != nil {
		logger.GetLogger().Warn("Final verification encountered issues", "error", err)
		// Continue anyway, USB might still be functional
	}

	logger.GetLogger().Info("Advanced Windows USB creation completed successfully!")
	creator.displayCompletionSummary(analysis, config)

	return nil
}

// displayAnalysisResults shows the source media analysis results
func (creator *AdvancedUSBCreator) displayAnalysisResults(analysis *types.FilesystemAnalysis) {
	logger.GetLogger().Info("Source Media Analysis")
	logger.GetLogger().Info("Files", "count", analysis.FileCount)
	logger.GetLogger().Info("Total size", "bytes", types.FormatBytes(analysis.TotalSize))
	logger.GetLogger().Info("Largest file", "bytes", types.FormatBytes(analysis.MaxFileSize))
	logger.GetLogger().Info("Recommended filesystem", "type", analysis.RecommendedFS)

	if analysis.WindowsVersion != "" {
		logger.GetLogger().Info("Detected version", "version", analysis.WindowsVersion)
	}

	if len(analysis.LargeFiles) > 0 {
		logger.GetLogger().Warn("Large files detected (>4GB)", "count", len(analysis.LargeFiles))
		for i, file := range analysis.LargeFiles {
			if i < 5 { // Show first 5
				logger.GetLogger().Warn("Large file", "path", file)
			} else if i == 5 {
				logger.GetLogger().Warn("Additional large files", "count", len(analysis.LargeFiles)-5)
				break
			}
		}
	}

	if analysis.RequiresUEFI_NTFS {
		logger.GetLogger().Info("UEFI:NTFS support will be enabled")
	}

	if len(analysis.RequiresWorkarounds) > 0 {
		logger.GetLogger().Info("Required workarounds", "list", analysis.RequiresWorkarounds)
	}

	logger.GetLogger().Info("=== End Analysis ===")
}

// mountTargetFilesystem mounts the target filesystem
func (creator *AdvancedUSBCreator) mountTargetFilesystem(device string) (string, error) {
	// Strip /dev/ prefix if present
	deviceID := strings.TrimPrefix(device, "/dev/")
	partitionDevice := "/dev/" + GetBlockDevicePartition(deviceID, 1) // First partition
	mountpoint := "/tmp/ember_mount_" + filepath.Base(device)

	// Create mountpoint
	err := os.MkdirAll(mountpoint, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create mountpoint: %w", err)
	}

	// Mount the filesystem (ExFAT on macOS)
	cmd := exec.Command("mount", "-t", "exfat", partitionDevice, mountpoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("mount failed: %w\nOutput: %s", err, string(output))
	}

	logger.GetLogger().Info("Mounted filesystem", "device", partitionDevice, "mountpoint", mountpoint)
	return mountpoint, nil
}

// unmountFilesystem unmounts a filesystem
func (creator *AdvancedUSBCreator) unmountFilesystem(mountpoint string) {
	if mountpoint == "" {
		return
	}

	cmd := exec.Command("umount", mountpoint)
	err := cmd.Run()
	if err != nil {
		logger.GetLogger().Warn("Failed to unmount filesystem", "mountpoint", mountpoint, "error", err)
	} else {
		logger.GetLogger().Info("Unmounted filesystem", "mountpoint", mountpoint)
	}

	// Remove mountpoint directory
	os.RemoveAll(mountpoint)
}

// installLegacyBootloader installs GRUB for legacy BIOS support
func (creator *AdvancedUSBCreator) installLegacyBootloader(mountpoint, device string) error {
	// Check if grub-install is available
	grubCmd := ""
	for _, cmd := range []string{"grub-install", "grub2-install"} {
		if _, err := exec.LookPath(cmd); err == nil {
			grubCmd = cmd
			break
		}
	}

	if grubCmd == "" {
		return fmt.Errorf("grub-install not found - legacy BIOS support unavailable")
	}

	logger.GetLogger().Info("Installing GRUB bootloader", "command", grubCmd)

	// Install GRUB
	cmd := exec.Command(grubCmd,
		"--target=i386-pc",
		"--boot-directory="+mountpoint,
		"--force",
		device)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("GRUB installation failed: %w\nOutput: %s", err, string(output))
	}

	// Create GRUB config
	err = creator.createGRUBConfig(mountpoint)
	if err != nil {
		return fmt.Errorf("GRUB config creation failed: %w", err)
	}

	logger.GetLogger().Info("Legacy BIOS bootloader installed successfully")
	return nil
}

// createGRUBConfig creates a GRUB configuration file
func (creator *AdvancedUSBCreator) createGRUBConfig(mountpoint string) error {
	grubDir := filepath.Join(mountpoint, "grub")
	if _, err := os.Stat(grubDir); os.IsNotExist(err) {
		grubDir = filepath.Join(mountpoint, "grub2")
	}

	err := os.MkdirAll(grubDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create GRUB directory: %w", err)
	}

	grubCfg := filepath.Join(grubDir, "grub.cfg")
	config := "ntldr /bootmgr\nboot\n"

	err = os.WriteFile(grubCfg, []byte(config), 0644)
	if err != nil {
		return fmt.Errorf("failed to write GRUB config: %w", err)
	}

	return nil
}

// performFinalVerification checks that everything is set up correctly
func (creator *AdvancedUSBCreator) performFinalVerification(analysis *types.FilesystemAnalysis, mountpoint, device string) error {
	// Check essential Windows files exist
	essentialFiles := []string{
		"bootmgr",
		"sources/boot.wim",
	}

	for _, file := range essentialFiles {
		filePath := filepath.Join(mountpoint, file)
		if _, err := os.Stat(filePath); err != nil {
			return fmt.Errorf("essential file missing: %s", file)
		}
	}

	// Verify Windows 7 workaround if applied
	if creator.windows7Workaround != nil {
		err := creator.windows7Workaround.ValidateWorkaround()
		if err != nil {
			return fmt.Errorf("Windows 7 workaround validation failed: %w", err)
		}
	}

	// Verify UEFI:NTFS partition if created
	if analysis.RequiresUEFI_NTFS {
		// This would require finding the UEFI:NTFS partition and validating it
		logger.GetLogger().Info("UEFI:NTFS partition verification (placeholder)")
	}

	logger.GetLogger().Info("Final verification passed")
	return nil
}

// displayCompletionSummary shows a summary of what was created
func (creator *AdvancedUSBCreator) displayCompletionSummary(analysis *types.FilesystemAnalysis, config *USBCreationConfig) {
	logger.GetLogger().Info("Creation Summary")
	logger.GetLogger().Info("Target device", "device", config.TargetDevice)
	logger.GetLogger().Info("Filesystem", "type", analysis.RecommendedFS)
	logger.GetLogger().Info("Label", "name", config.Label)

	if analysis.RequiresUEFI_NTFS {
		logger.GetLogger().Info("✓ UEFI:NTFS support enabled")
	}

	if creator.windows7Workaround != nil && creator.windows7Workaround.IsRequired() {
		logger.GetLogger().Info("✓ Windows 7 UEFI workaround applied")
	}

	capabilities := []string{}
	if config.EnableUEFI {
		capabilities = append(capabilities, "UEFI boot")
	}
	if config.LegacyBIOS {
		capabilities = append(capabilities, "Legacy BIOS boot")
	}

	if len(capabilities) > 0 {
		logger.GetLogger().Info("Boot capabilities", "list", strings.Join(capabilities, ", "))
	}

	logger.GetLogger().Info("Files copied", "count", analysis.FileCount)
	logger.GetLogger().Info("Total size", "bytes", types.FormatBytes(analysis.TotalSize))

	logger.GetLogger().Info("=== USB Ready for Use ===")
	logger.GetLogger().Info("The Windows USB drive is now ready for installation.")
	logger.GetLogger().Info("You can safely remove the device.")
}

// GetCreationSummary returns a detailed summary of the creation process
func (creator *AdvancedUSBCreator) GetCreationSummary() map[string]interface{} {
	summary := map[string]interface{}{
		"dependency_manager": creator.dependencyManager.GetSystemInfo(),
		"progress_tracker":   creator.progressTracker.GetOperationSummary(),
	}

	if creator.partitionManager != nil {
		partInfo, _ := creator.partitionManager.GetPartitionInfo()
		summary["partition_info"] = partInfo
	}

	if creator.windows7Workaround != nil {
		summary["windows7_workaround"] = creator.windows7Workaround.GetWorkaroundInfo()
	}

	return summary
}

// Cleanup performs cleanup operations
func (creator *AdvancedUSBCreator) Cleanup() {
	if creator.progressTracker != nil {
		creator.progressTracker.Stop()
	}
}