package filesystem

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/entro314-labs/ember/internal/security"
)

//go:generate sh build-ms-sys.sh

// Embed ms-sys binary if it exists, otherwise it will be empty
var MsSysBin []byte // Changed from MsSysBin to follow Go naming conventions

// BootRecordType represents different types of master boot records
type BootRecordType int

const (
	MbrAuto BootRecordType = iota // Changed from MbrAuto to follow Go naming conventions
	MbrWin7                       // Changed from MbrWin7
	MbrVista                      // Changed from MbrVista
	Mbr2000                       // Changed from Mbr2000
	Mbr95B                        // Changed from Mbr95B
	MbrDos                        // Changed from MbrDos
	MbrSyslinux                   // Changed from MBR_SYSLINUX
	MbrGrub4Dos                   // Changed from MBR_GRUB4DOS
	MbrGrub2                      // Changed from MBR_GRUB2
	MbrRufus                      // Changed from MbrRufus
)

// BootRecordInfo contains information about a boot record type
type BootRecordInfo struct {
	Type        BootRecordType
	Name        string
	Description string
	Flag        string
	Recommended bool
}

var bootRecords = map[BootRecordType]BootRecordInfo{
	MbrWin7: {
		Type:        MbrWin7,
		Name:        "Windows 7/8/10/11",
		Description: "Modern Windows master boot record with LBA support",
		Flag:        "-7",
		Recommended: true,
	},
	MbrVista: {
		Type:        MbrVista,
		Name:        "Windows Vista",
		Description: "Windows Vista master boot record with LBA support",
		Flag:        "-i",
		Recommended: false,
	},
	Mbr2000: {
		Type:        Mbr2000,
		Name:        "Windows 2000/XP/2003",
		Description: "Windows 2000/XP/2003 master boot record with LBA support",
		Flag:        "-m",
		Recommended: false,
	},
	Mbr95B: {
		Type:        Mbr95B,
		Name:        "Windows 95B/98/ME",
		Description: "Windows 95B/98/98SE/ME master boot record",
		Flag:        "-9",
		Recommended: false,
	},
	MbrDos: {
		Type:        MbrDos,
		Name:        "DOS/Windows NT",
		Description: "DOS/Windows NT master boot record (no LBA)",
		Flag:        "-d",
		Recommended: false,
	},
	MbrRufus: {
		Type:        MbrRufus,
		Name:        "Rufus",
		Description: "Rufus master boot record",
		Flag:        "-r",
		Recommended: true,
	},
}

// IsMsSysAvailable checks if ms-sys binary is available
func IsMsSysAvailable() bool {
	return len(MsSysBin) > 0
}

// GetMsSysAsProgram extracts the embedded ms-sys binary to a temporary file
func GetMsSysAsProgram() (*os.File, error) {
	if len(MsSysBin) == 0 {
		return nil, fmt.Errorf("ms-sys binary not available (not built with ms-sys support)")
	}

	tempDir := os.TempDir()
	filename := "ms-sys"
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}

	f, err := os.CreateTemp(tempDir, filename+"-*.bin")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}

	// Write the binary data
	if _, err := f.Write(MsSysBin); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, fmt.Errorf("failed to write ms-sys binary: %w", err)
	}

	// Make it executable
	if err := f.Chmod(0755); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, fmt.Errorf("failed to make ms-sys executable: %w", err)
	}

	// Close the file but keep it for execution
	f.Close()

	// Reopen in read mode for execution
	execFile, err := os.Open(f.Name())
	if err != nil {
		os.Remove(f.Name())
		return nil, fmt.Errorf("failed to reopen ms-sys binary: %w", err)
	}

	return execFile, nil
}

// WriteMBRToPartition writes a master boot record to the specified partition
func WriteMBRToPartition(partition string) error {
	return WriteMBRToPartitionWithType(partition, MbrWin7)
}

// WriteMBRToPartitionWithType writes a specific type of MBR to the partition
func WriteMBRToPartitionWithType(partition string, recordType BootRecordType) error {
	if !IsMsSysAvailable() {
		return fmt.Errorf("ms-sys binary not available - build with ms-sys support to enable MBR writing")
	}

	msSysFile, err := GetMsSysAsProgram()
	if err != nil {
		return fmt.Errorf("failed to prepare ms-sys: %w", err)
	}
	defer func() {
		msSysFile.Close()
		os.Remove(msSysFile.Name())
	}()

	// Get boot record info
	info, exists := bootRecords[recordType]
	if !exists {
		info = bootRecords[MbrWin7] // fallback to Windows 7 MBR
	}

	// Build command arguments
	args := []string{info.Flag, partition}

	// Validate arguments for security
	validator := security.GetSecurityValidator()
	if err := validator.ValidateCommandArguments(msSysFile.Name(), args); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}

	// Execute ms-sys with validated arguments
	// #nosec G204 - Arguments are validated by security validator
	cmd := exec.Command(msSysFile.Name(), args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to write %s MBR to %s: %w\noutput: %s", info.Name, partition, err, out)
	}

	return nil
}

// GetAvailableBootRecords returns a list of available boot record types
func GetAvailableBootRecords() []BootRecordInfo {
	var records []BootRecordInfo
	for _, info := range bootRecords {
		records = append(records, info)
	}
	return records
}

// GetRecommendedBootRecord returns the recommended boot record type for Windows USB
func GetRecommendedBootRecord() BootRecordType {
	return MbrWin7
}

// DetectBootRecord attempts to detect what type of boot record is on a device
func DetectBootRecord(device string) (BootRecordType, error) {
	if !IsMsSysAvailable() {
		return MbrAuto, fmt.Errorf("ms-sys not available for detection")
	}

	msSysFile, err := GetMsSysAsProgram()
	if err != nil {
		return MbrAuto, err
	}
	defer func() {
		msSysFile.Close()
		os.Remove(msSysFile.Name())
	}()

	// Validate command arguments for security
	validator := security.GetSecurityValidator()
	args := []string{device}
	if err := validator.ValidateCommandArguments(msSysFile.Name(), args); err != nil {
		return MbrAuto, fmt.Errorf("command validation failed: %w", err)
	}

	// Run ms-sys in diagnostic mode (no flags = inspect)
	// #nosec G204 - Arguments are validated by security validator
	cmd := exec.Command(msSysFile.Name(), device)
	out, err := cmd.Output()
	if err != nil {
		return MbrAuto, fmt.Errorf("failed to detect boot record: %w", err)
	}

	output := string(out)

	// Parse the output to determine boot record type
	switch {
	case strings.Contains(output, "Windows 7"):
		return MbrWin7, nil
	case strings.Contains(output, "Windows Vista"):
		return MbrVista, nil
	case strings.Contains(output, "Windows 2000") || strings.Contains(output, "Windows XP"):
		return Mbr2000, nil
	case strings.Contains(output, "Windows 95B") || strings.Contains(output, "Windows 98"):
		return Mbr95B, nil
	case strings.Contains(output, "DOS"):
		return MbrDos, nil
	case strings.Contains(output, "Rufus"):
		return MbrRufus, nil
	default:
		return MbrAuto, nil
	}
}

// WriteBootRecordWithFallback attempts to write a boot record with fallback options
func WriteBootRecordWithFallback(device string, preferredType BootRecordType) error {
	// Try the preferred type first
	if err := WriteMBRToPartitionWithType(device, preferredType); err == nil {
		return nil
	}

	// If that fails, try the recommended type
	if preferredType != MbrWin7 {
		if err := WriteMBRToPartitionWithType(device, MbrWin7); err == nil {
			return nil
		}
	}

	// If that fails, try Rufus MBR (known to work well)
	if preferredType != MbrRufus {
		if err := WriteMBRToPartitionWithType(device, MbrRufus); err == nil {
			return nil
		}
	}

	// If all specific types fail, try the basic Windows 2000 MBR
	return WriteMBRToPartitionWithType(device, Mbr2000)
}

// GetMsSysVersion returns the version of the embedded ms-sys binary
func GetMsSysVersion() (string, error) {
	if !IsMsSysAvailable() {
		return "", fmt.Errorf("ms-sys not available")
	}

	msSysFile, err := GetMsSysAsProgram()
	if err != nil {
		return "", err
	}
	defer func() {
		msSysFile.Close()
		os.Remove(msSysFile.Name())
	}()

	// Validate command arguments for security
	validator := security.GetSecurityValidator()
	args := []string{"--version"}
	if err := validator.ValidateCommandArguments(msSysFile.Name(), args); err != nil {
		return "", fmt.Errorf("command validation failed: %w", err)
	}

	// #nosec G204 - Static argument, no user input
	cmd := exec.Command(msSysFile.Name(), "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get ms-sys version: %w", err)
	}

	version := strings.TrimSpace(string(out))
	return version, nil
}

// ValidateMsSysInstallation checks if ms-sys is properly embedded and functional
func ValidateMsSysInstallation() error {
	if !IsMsSysAvailable() {
		return fmt.Errorf("ms-sys binary is not embedded")
	}

	// Try to get version to validate functionality
	if _, err := GetMsSysVersion(); err != nil {
		return fmt.Errorf("ms-sys binary is embedded but not functional: %w", err)
	}

	return nil
}

// CleanupTempFiles removes any temporary ms-sys files that might be left behind
func CleanupTempFiles() error {
	tempDir := os.TempDir()
	pattern := filepath.Join(tempDir, "ms-sys-*.bin")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			// Log but don't fail on cleanup errors
			fmt.Printf("Warning: Failed to cleanup temp file %s: %v\n", match, err)
		}
	}

	return nil
}
