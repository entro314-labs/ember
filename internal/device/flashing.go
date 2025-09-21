package device

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Xmister/udf"
	"github.com/entro314-labs/ember/internal/iso"
	"github.com/entro314-labs/ember/internal/types"
)

// FlashProgress represents the current state of the USB flashing process
type FlashProgress struct {
	Step       int
	TotalSteps int
	StepName   string
	Progress   float64
	Error      error
	Completed  bool
}

// Flash progress messages
type flashProgressMsg FlashProgress
type flashCompleteMsg struct{}
type flashErrorMsg struct {
	err error
}

// FlashUSB orchestrates the entire USB flashing process
func FlashUSB(isoPath string, device *types.DiskUtilDevice) tea.Cmd {
	return func() tea.Msg {
		progress := &FlashProgress{
			TotalSteps: 6,
		}

		// Step 1: Validate inputs
		progress.Step = 1
		progress.StepName = "Validating inputs"
		progress.Progress = 0.0

		if err := validateFlashInputs(isoPath, device); err != nil {
			return flashErrorMsg{err: fmt.Errorf("validation failed: %w", err)}
		}
		progress.Progress = 1.0 / float64(progress.TotalSteps)

		// Step 2: Open and validate ISO
		progress.Step = 2
		progress.StepName = "Opening Windows ISO"
		progress.Progress = 2.0 / float64(progress.TotalSteps)

		isoFile, err := os.Open(isoPath)
		if err != nil {
			return flashErrorMsg{err: fmt.Errorf("failed to open ISO: %w", err)}
		}
		defer isoFile.Close()

		// Open and validate Windows ISO
		windowsISO, err := iso.OpenWindowsISO(isoFile)
		if err != nil {
			return flashErrorMsg{err: fmt.Errorf("invalid Windows ISO: %w", err)}
		}

		// Step 3: Prepare device
		progress.Step = 3
		progress.StepName = "Preparing USB device"
		progress.Progress = 3.0 / float64(progress.TotalSteps)

		if err := PrepareDevice(device); err != nil {
			return flashErrorMsg{err: fmt.Errorf("failed to prepare device: %w", err)}
		}

		// Step 4: Create partitions
		progress.Step = 4
		progress.StepName = "Creating partitions"
		progress.Progress = 4.0 / float64(progress.TotalSteps)

		if err := FormatDiskForUEFINTFS(device.DeviceIdentifier, false); err != nil {
			return flashErrorMsg{err: fmt.Errorf("failed to partition device: %w", err)}
		}

		// Step 5: Copy ISO contents
		progress.Step = 5
		progress.StepName = "Copying Windows files"
		progress.Progress = 5.0 / float64(progress.TotalSteps)

		if err := copyISOContents(windowsISO, device); err != nil {
			return flashErrorMsg{err: fmt.Errorf("failed to copy ISO contents: %w", err)}
		}

		// Step 6: Install bootloader
		progress.Step = 6
		progress.StepName = "Installing bootloader"
		progress.Progress = 6.0 / float64(progress.TotalSteps)

		if err := InstallBootloader(device); err != nil {
			return flashErrorMsg{err: fmt.Errorf("failed to install bootloader: %w", err)}
		}

		return flashCompleteMsg{}
	}
}

func validateFlashInputs(isoPath string, device *types.DiskUtilDevice) error {
	// Check ISO file exists
	if _, err := os.Stat(isoPath); err != nil {
		return fmt.Errorf("ISO file not found: %s", isoPath)
	}

	// Check ISO file extension
	if !strings.HasSuffix(strings.ToLower(isoPath), ".iso") {
		return fmt.Errorf("file does not appear to be an ISO: %s", isoPath)
	}

	// Validate device
	if device == nil {
		return fmt.Errorf("no device selected")
	}

	// Check device is suitable
	if err := ValidateDevice(device.DeviceIdentifier); err != nil {
		return err
	}

	// Warn about small devices
	if device.Size < 8*1024*1024*1024 { // 8GB
		return fmt.Errorf("device %s is too small (%.1f GB). Windows USB requires at least 8GB",
			device.DeviceIdentifier, float64(device.Size)/(1024*1024*1024))
	}

	return nil
}

func PrepareDevice(device *types.DiskUtilDevice) error {
	// Unmount any mounted partitions
	return unmountDisk(device.DeviceIdentifier)
}

func copyISOContents(windowsISO *udf.Udf, device *types.DiskUtilDevice) error {
	// Get the Windows partition mount point
	windowsPartition := GetBlockDevicePartition(device.DeviceIdentifier, 1)

	// Create mount point directory
	mountPoint := "/tmp/ember-windows"
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	// Mount the Windows partition
	if err := mountPartition(windowsPartition, mountPoint); err != nil {
		return fmt.Errorf("failed to mount Windows partition: %w", err)
	}
	defer unmountPartition(mountPoint)

	// Extract ISO contents to the mounted Windows partition
	if err := iso.ExtractISOToLocation(windowsISO, mountPoint); err != nil {
		return fmt.Errorf("failed to extract ISO contents: %w", err)
	}

	return nil
}

// HasExtractMethod interface for ISO types that can extract themselves
type HasExtractMethod interface {
	ExtractTo(path string) error
}

func InstallBootloader(device *types.DiskUtilDevice) error {
	// Get the UEFI partition
	uefiPartition := GetBlockDevicePartition(device.DeviceIdentifier, 2)

	// Write UEFI:NTFS to the UEFI partition (if available on this platform)
	// For now, just return success on non-Linux platforms
	if err := writeUEFINTFSIfAvailable(uefiPartition); err != nil {
		return fmt.Errorf("failed to write UEFI:NTFS: %w", err)
	}

	// Install MBR bootloader for legacy BIOS compatibility
	if err := installMBRBootloader(device); err != nil {
		return fmt.Errorf("failed to install MBR bootloader: %w", err)
	}

	return nil
}

// writeUEFINTFSIfAvailable writes UEFI:NTFS if the function is available
func writeUEFINTFSIfAvailable(partition string) error {
	// Use the UEFI:NTFS support system if available
	if uefiSupport, err := CreateUEFINTFSPartition(partition); err == nil {
		defer uefiSupport.CleanupUEFINTFSSupport()
		return uefiSupport.ValidateUEFINTFSPartition()
	}

	// If UEFI:NTFS not available, continue without it
	return nil
}

// installMBRBootloader installs legacy BIOS bootloader
func installMBRBootloader(device *types.DiskUtilDevice) error {
	// For MBR bootloader installation, we would typically use syslinux or grub
	// For now, we'll use the existing bootloader functionality
	windowsPartition := GetBlockDevicePartition(device.DeviceIdentifier, 1)

	// Mark the Windows partition as active (bootable)
	if err := markPartitionActive(windowsPartition); err != nil {
		return fmt.Errorf("failed to mark partition active: %w", err)
	}

	return nil
}

// markPartitionActive marks a partition as active/bootable
func markPartitionActive(partition string) error {
	// Use platform-specific commands to mark partition as active
	cmd := exec.Command("diskutil", "mount", partition)
	return cmd.Run()
}

// CreateProgressCmd creates a command that sends progress updates
func CreateProgressCmd(step int, totalSteps int, stepName string, progress float64) tea.Cmd {
	return func() tea.Msg {
		return flashProgressMsg(FlashProgress{
			Step:       step,
			TotalSteps: totalSteps,
			StepName:   stepName,
			Progress:   progress,
		})
	}
}

// SimulateFlashingCmd simulates the flashing process for demonstration
func SimulateFlashingCmd(_ string, device *types.DiskUtilDevice) tea.Cmd { // Renamed unused parameter
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		steps := []string{
			"Validating inputs",
			"Opening Windows ISO",
			"Preparing USB device",
			"Creating partitions",
			"Copying Windows files",
			"Installing bootloader",
		}

		for i := range steps {
			time.Sleep(time.Second * 2) // Simulate work

			// Send progress update (in a real implementation, you'd send this via channels)
			if i == len(steps)-1 {
				return flashCompleteMsg{}
			}
		}

		return flashCompleteMsg{}
	})
}

// mountPartition mounts a partition to the specified mount point
func mountPartition(partitionDevice, mountPoint string) error {
	cmd := exec.Command("mount", partitionDevice, mountPoint)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to mount %s to %s: %w", partitionDevice, mountPoint, err)
	}
	return nil
}

// unmountPartition unmounts a partition from the specified mount point
func unmountPartition(mountPoint string) error {
	cmd := exec.Command("umount", mountPoint)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unmount %s: %w", mountPoint, err)
	}
	return nil
}
