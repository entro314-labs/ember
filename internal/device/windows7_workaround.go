package device

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/entro314-labs/ember/internal/logger"
)

// Windows7UEFIWorkaround handles Windows 7 UEFI boot compatibility fixes
type Windows7UEFIWorkaround struct {
	sourcePath   string
	targetPath   string
	detectedVersion string
	hasBootmgrEfi bool
	hasInstallWim bool
}

// NewWindows7UEFIWorkaround creates a new workaround instance
func NewWindows7UEFIWorkaround(sourcePath, targetPath string) *Windows7UEFIWorkaround {
	return &Windows7UEFIWorkaround{
		sourcePath: sourcePath,
		targetPath: targetPath,
	}
}

// Analyze determines if Windows 7 UEFI workaround is needed
func (w *Windows7UEFIWorkaround) Analyze() error {
	logger.GetLogger().Info("Analyzing for Windows 7 UEFI compatibility issues...")

	// Check for cversion.ini to detect Windows version
	cversionPath := filepath.Join(w.sourcePath, "sources", "cversion.ini")
	if _, err := os.Stat(cversionPath); err == nil {
		version, err := w.detectWindowsVersionFromCVersion(cversionPath)
		if err != nil {
			logger.GetLogger().Warn("Failed to detect Windows version", "error", err)
		} else {
			w.detectedVersion = version
			logger.GetLogger().Info("Detected Windows version", "version", version)
		}
	}

	// Check for bootmgr.efi
	bootmgrEfiPath := filepath.Join(w.sourcePath, "bootmgr.efi")
	if _, err := os.Stat(bootmgrEfiPath); err == nil {
		w.hasBootmgrEfi = true
		logger.GetLogger().Info("Found bootmgr.efi")
	}

	// Check for install.wim
	installWimPath := filepath.Join(w.sourcePath, "sources", "install.wim")
	if _, err := os.Stat(installWimPath); err == nil {
		w.hasInstallWim = true
		logger.GetLogger().Info("Found install.wim")
	}

	return nil
}

// IsRequired determines if the workaround is necessary
func (w *Windows7UEFIWorkaround) IsRequired() bool {
	// Windows 7 UEFI workaround is needed when:
	// 1. It's Windows 7 (detected from cversion.ini)
	// 2. Has install.wim but missing proper EFI bootloader
	// 3. Has bootmgr.efi but EFI directory structure is incomplete

	isWindows7 := strings.Contains(strings.ToLower(w.detectedVersion), "7")
	needsWorkaround := isWindows7 && w.hasInstallWim && !w.hasCompleteEFIStructure()

	if needsWorkaround {
		logger.GetLogger().Info("Windows 7 UEFI workaround required")
	}

	return needsWorkaround
}

// Apply executes the Windows 7 UEFI compatibility fix
func (w *Windows7UEFIWorkaround) Apply() error {
	if !w.IsRequired() {
		logger.GetLogger().Info("Windows 7 UEFI workaround not required")
		return nil
	}

	logger.GetLogger().Info("Applying Windows 7 UEFI compatibility workaround...")

	// Step 1: Create EFI directory structure
	err := w.createEFIDirectoryStructure()
	if err != nil {
		return fmt.Errorf("failed to create EFI directory structure: %w", err)
	}

	// Step 2: Extract bootmgfw.efi from install.wim
	err = w.extractBootloaderFromWIM()
	if err != nil {
		return fmt.Errorf("failed to extract EFI bootloader: %w", err)
	}

	// Step 3: Copy additional EFI files if available
	err = w.copyAdditionalEFIFiles()
	if err != nil {
		logger.GetLogger().Warn("Failed to copy additional EFI files", "error", err)
		// Continue anyway, main bootloader is more important
	}

	logger.GetLogger().Info("Windows 7 UEFI workaround applied successfully")
	return nil
}

// detectWindowsVersionFromCVersion parses cversion.ini to detect Windows version
func (w *Windows7UEFIWorkaround) detectWindowsVersionFromCVersion(cversionPath string) (string, error) {
	content, err := os.ReadFile(cversionPath)
	if err != nil {
		return "", err
	}

	// Look for MinServer line to determine version
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "MinServer=") {
			versionStr := strings.TrimPrefix(line, "MinServer=")

			// Use regex to extract version number
			re := regexp.MustCompile(`^(\d+)`)
			matches := re.FindStringSubmatch(versionStr)
			if len(matches) > 1 {
				majorVersion := matches[1]
				switch majorVersion {
				case "7":
					return "Windows 7", nil
				case "8":
					return "Windows 8", nil
				case "10":
					return "Windows 10", nil
				case "11":
					return "Windows 11", nil
				default:
					return fmt.Sprintf("Windows (version %s)", versionStr), nil
				}
			}
		}
	}

	return "", fmt.Errorf("version not found in cversion.ini")
}

// hasCompleteEFIStructure checks if EFI boot structure is already complete
func (w *Windows7UEFIWorkaround) hasCompleteEFIStructure() bool {
	// Check for existing EFI bootloader
	efiPaths := []string{
		filepath.Join(w.targetPath, "efi", "boot", "bootx64.efi"),
		filepath.Join(w.targetPath, "EFI", "BOOT", "BOOTX64.EFI"),
		filepath.Join(w.targetPath, "efi", "microsoft", "boot", "bootmgfw.efi"),
	}

	for _, path := range efiPaths {
		if _, err := os.Stat(path); err == nil {
			logger.GetLogger().Info("Found existing EFI bootloader", "path", path)
			return true
		}
	}

	return false
}

// createEFIDirectoryStructure creates the necessary EFI directory structure
func (w *Windows7UEFIWorkaround) createEFIDirectoryStructure() error {
	// Create standard EFI directory structure
	efiDirs := []string{
		filepath.Join(w.targetPath, "efi"),
		filepath.Join(w.targetPath, "efi", "boot"),
		filepath.Join(w.targetPath, "efi", "microsoft"),
		filepath.Join(w.targetPath, "efi", "microsoft", "boot"),
	}

	for _, dir := range efiDirs {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		logger.GetLogger().Debug("Created EFI directory", "path", dir)
	}

	return nil
}

// extractBootloaderFromWIM extracts bootmgfw.efi from install.wim
func (w *Windows7UEFIWorkaround) extractBootloaderFromWIM() error {
	installWimPath := filepath.Join(w.sourcePath, "sources", "install.wim")
	if _, err := os.Stat(installWimPath); err != nil {
		return fmt.Errorf("install.wim not found: %w", err)
	}

	// Check if 7z is available
	if _, err := exec.LookPath("7z"); err != nil {
		return fmt.Errorf("7z command not found, required for WIM extraction: %w", err)
	}

	// Extract bootmgfw.efi from the WIM file
	targetBootloader := filepath.Join(w.targetPath, "efi", "boot", "bootx64.efi")

	logger.GetLogger().Info("Extracting EFI bootloader from install.wim...")

	// Use 7z to extract the specific file
	cmd := exec.Command("7z", "e", "-so", installWimPath, "Windows/Boot/EFI/bootmgfw.efi")
	output, err := cmd.Output()
	if err != nil {
		// Try alternative path
		cmd = exec.Command("7z", "e", "-so", installWimPath, "1/Windows/Boot/EFI/bootmgfw.efi")
		output, err = cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to extract bootmgfw.efi from WIM: %w", err)
		}
	}

	// Write the extracted bootloader
	err = os.WriteFile(targetBootloader, output, 0644)
	if err != nil {
		return fmt.Errorf("failed to write bootloader: %w", err)
	}

	// Verify the extracted file
	if len(output) == 0 {
		return fmt.Errorf("extracted bootloader is empty")
	}

	logger.GetLogger().Info("Successfully extracted EFI bootloader", "size", len(output), "target", targetBootloader)

	// Also copy as the Microsoft bootloader name for compatibility
	microsoftBootloader := filepath.Join(w.targetPath, "efi", "microsoft", "boot", "bootmgfw.efi")
	err = os.WriteFile(microsoftBootloader, output, 0644)
	if err != nil {
		logger.GetLogger().Warn("Failed to copy bootloader to Microsoft path", "error", err)
	}

	return nil
}

// copyAdditionalEFIFiles copies other EFI-related files if available
func (w *Windows7UEFIWorkaround) copyAdditionalEFIFiles() error {
	// List of additional EFI files that might be useful
	additionalFiles := map[string]string{
		"bootmgr.efi": "efi/boot/bootmgr.efi",
		"memtest.efi": "efi/boot/memtest.efi",
	}

	for srcFile, dstPath := range additionalFiles {
		srcPath := filepath.Join(w.sourcePath, srcFile)
		dstFullPath := filepath.Join(w.targetPath, dstPath)

		if _, err := os.Stat(srcPath); err == nil {
			// Ensure destination directory exists
			dstDir := filepath.Dir(dstFullPath)
			err = os.MkdirAll(dstDir, 0755)
			if err != nil {
				logger.GetLogger().Warn("Failed to create directory for %s: %v", dstPath, err)
				continue
			}

			// Copy the file
			data, err := os.ReadFile(srcPath)
			if err != nil {
				logger.GetLogger().Warn("Failed to read %s: %v", srcFile, err)
				continue
			}

			err = os.WriteFile(dstFullPath, data, 0644)
			if err != nil {
				logger.GetLogger().Warn("Failed to write %s: %v", dstPath, err)
				continue
			}

			logger.GetLogger().Info("Copied additional EFI file: %s -> %s", srcFile, dstPath)
		}
	}

	return nil
}

// ValidateWorkaround verifies that the workaround was applied correctly
func (w *Windows7UEFIWorkaround) ValidateWorkaround() error {
	if !w.IsRequired() {
		return nil
	}

	// Check that the main bootloader exists
	bootloaderPath := filepath.Join(w.targetPath, "efi", "boot", "bootx64.efi")
	info, err := os.Stat(bootloaderPath)
	if err != nil {
		return fmt.Errorf("EFI bootloader not found after workaround: %w", err)
	}

	if info.Size() == 0 {
		return fmt.Errorf("EFI bootloader is empty")
	}

	// Check that it's a valid PE file (Windows executable)
	err = validatePEFile(bootloaderPath)
	if err != nil {
		return fmt.Errorf("extracted bootloader is not a valid PE file: %w", err)
	}

	logger.GetLogger().Info("Windows 7 UEFI workaround validation successful")
	return nil
}

// validatePEFile checks if a file is a valid PE (Portable Executable) file
func validatePEFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read DOS header
	dosHeader := make([]byte, 64)
	_, err = file.Read(dosHeader)
	if err != nil {
		return err
	}

	// Check DOS signature (MZ)
	if dosHeader[0] != 'M' || dosHeader[1] != 'Z' {
		return fmt.Errorf("invalid DOS signature")
	}

	// Get PE header offset
	peOffset := int64(dosHeader[60]) | int64(dosHeader[61])<<8 | int64(dosHeader[62])<<16 | int64(dosHeader[63])<<24

	// Seek to PE header
	_, err = file.Seek(peOffset, 0)
	if err != nil {
		return err
	}

	// Read PE signature
	peSignature := make([]byte, 4)
	_, err = file.Read(peSignature)
	if err != nil {
		return err
	}

	// Check PE signature
	if string(peSignature) != "PE\x00\x00" {
		return fmt.Errorf("invalid PE signature")
	}

	return nil
}

// GetWorkaroundInfo returns information about the applied workaround
func (w *Windows7UEFIWorkaround) GetWorkaroundInfo() map[string]interface{} {
	return map[string]interface{}{
		"source_path":       w.sourcePath,
		"target_path":       w.targetPath,
		"detected_version":  w.detectedVersion,
		"has_bootmgr_efi":   w.hasBootmgrEfi,
		"has_install_wim":   w.hasInstallWim,
		"is_required":       w.IsRequired(),
		"has_efi_structure": w.hasCompleteEFIStructure(),
	}
}