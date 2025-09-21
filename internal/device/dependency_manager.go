package device

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/entro314-labs/ember/internal/logger"
)

// DependencyManager handles system dependency checking and recovery
type DependencyManager struct {
	requiredTools     map[string]ToolRequirement
	optionalTools     map[string]ToolRequirement
	missingRequired   []string
	missingOptional   []string
	systemCapability SystemCapability
}

// ToolRequirement defines requirements for a system tool
type ToolRequirement struct {
	Name         string
	Commands     []string // Alternative command names
	Purpose      string
	Installation string   // Installation instructions
	Critical     bool
	MinVersion   string
	CheckVersion func(string) (string, error)
}

// SystemCapability tracks system capabilities
type SystemCapability struct {
	Platform           string
	CanPartition       bool
	CanFormat          bool
	CanMount           bool
	CanExtractArchives bool
	CanInstallGRUB     bool
	HasAdminRights     bool
	SupportsUEFI       bool
}

// NewDependencyManager creates a new dependency manager
func NewDependencyManager() *DependencyManager {
	dm := &DependencyManager{
		requiredTools:   make(map[string]ToolRequirement),
		optionalTools:   make(map[string]ToolRequirement),
		missingRequired: make([]string, 0),
		missingOptional: make([]string, 0),
	}

	dm.initializeToolRequirements()
	return dm
}

// initializeToolRequirements sets up all tool requirements
func (dm *DependencyManager) initializeToolRequirements() {
	// Platform-specific partitioning tools (required)
	if runtime.GOOS == "darwin" {
		dm.requiredTools["diskutil"] = ToolRequirement{
			Name:         "diskutil",
			Commands:     []string{"diskutil"},
			Purpose:      "macOS native disk utility for partitioning and formatting",
			Installation: "Built-in to macOS",
			Critical:     true,
		}
	} else {
		dm.requiredTools["parted"] = ToolRequirement{
			Name:         "parted",
			Commands:     []string{"parted"},
			Purpose:      "Partition management and creation",
			Installation: "apt install parted (Ubuntu/Debian) or yum install parted (RHEL/CentOS)",
			Critical:     true,
		}
	}

	// Linux-specific block device tools (not needed on macOS)
	if runtime.GOOS != "darwin" {
		dm.requiredTools["lsblk"] = ToolRequirement{
			Name:         "lsblk",
			Commands:     []string{"lsblk"},
			Purpose:      "Block device listing and information",
			Installation: "Usually included with util-linux package",
			Critical:     true,
		}

		dm.requiredTools["wipefs"] = ToolRequirement{
			Name:         "wipefs",
			Commands:     []string{"wipefs"},
			Purpose:      "Filesystem signature removal",
			Installation: "Usually included with util-linux package",
			Critical:     true,
		}

		dm.requiredTools["blockdev"] = ToolRequirement{
			Name:         "blockdev",
			Commands:     []string{"blockdev"},
			Purpose:      "Block device control and information",
			Installation: "Usually included with util-linux package",
			Critical:     true,
		}
	}

	// Filesystem creation tools (required)
	dm.requiredTools["mkfs.fat"] = ToolRequirement{
		Name:     "mkfs.fat",
		Commands: []string{"mkfs.fat", "mkdosfs", "mkfs.msdos", "mkfs.vfat"},
		Purpose:  "FAT filesystem creation",
		Installation: map[string]string{
			"darwin": "brew install dosfstools",
			"linux":  "apt install dosfstools (Ubuntu/Debian) or yum install dosfstools (RHEL/CentOS)",
		}[runtime.GOOS],
		Critical: true,
	}

	// NTFS support (only required on Linux; macOS uses diskutil with ExFAT fallback)
	if runtime.GOOS != "darwin" {
		dm.requiredTools["mkfs.ntfs"] = ToolRequirement{
			Name:         "mkfs.ntfs",
			Commands:     []string{"mkfs.ntfs", "mkntfs"},
			Purpose:      "NTFS filesystem creation",
			Installation: "apt install ntfs-3g (Ubuntu/Debian) or yum install ntfs-3g (RHEL/CentOS)",
			Critical:     true,
		}
	}

	// Mount tools (required)
	dm.requiredTools["mount"] = ToolRequirement{
		Name:     "mount",
		Commands: []string{"mount"},
		Purpose:  "Filesystem mounting",
		Installation: "Usually available by default on Unix systems",
		Critical:     true,
	}

	dm.requiredTools["umount"] = ToolRequirement{
		Name:     "umount",
		Commands: []string{"umount"},
		Purpose:  "Filesystem unmounting",
		Installation: "Usually available by default on Unix systems",
		Critical:     true,
	}

	// Archive extraction (required for Windows 7 workaround)
	dm.requiredTools["7z"] = ToolRequirement{
		Name:     "7z",
		Commands: []string{"7z", "7za", "7zr"},
		Purpose:  "Archive extraction (required for Windows 7 UEFI workaround)",
		Installation: map[string]string{
			"darwin": "brew install p7zip",
			"linux":  "apt install p7zip-full (Ubuntu/Debian) or yum install p7zip (RHEL/CentOS)",
		}[runtime.GOOS],
		Critical: true,
	}

	// Optional tools for enhanced functionality
	dm.optionalTools["grub-install"] = ToolRequirement{
		Name:     "grub-install",
		Commands: []string{"grub-install", "grub2-install"},
		Purpose:  "GRUB bootloader installation for legacy BIOS support",
		Installation: map[string]string{
			"darwin": "Not typically available on macOS",
			"linux":  "apt install grub2-common grub-pc-bin (Ubuntu/Debian) or yum install grub2-tools (RHEL/CentOS)",
		}[runtime.GOOS],
		Critical: false,
	}

	dm.optionalTools["sgdisk"] = ToolRequirement{
		Name:     "sgdisk",
		Commands: []string{"sgdisk"},
		Purpose:  "GPT partition table manipulation",
		Installation: map[string]string{
			"darwin": "brew install gptfdisk",
			"linux":  "apt install gdisk (Ubuntu/Debian) or yum install gdisk (RHEL/CentOS)",
		}[runtime.GOOS],
		Critical: false,
	}

	dm.optionalTools["partprobe"] = ToolRequirement{
		Name:     "partprobe",
		Commands: []string{"partprobe"},
		Purpose:  "Force kernel to re-read partition tables",
		Installation: map[string]string{
			"darwin": "Available via parted: brew install parted",
			"linux":  "Usually included with parted package",
		}[runtime.GOOS],
		Critical: false,
	}

	dm.optionalTools["mkfs.exfat"] = ToolRequirement{
		Name:     "mkfs.exfat",
		Commands: []string{"mkfs.exfat"},
		Purpose:  "exFAT filesystem creation",
		Installation: map[string]string{
			"darwin": "brew install exfat",
			"linux":  "apt install exfat-utils (Ubuntu/Debian) or yum install exfat-utils (RHEL/CentOS)",
		}[runtime.GOOS],
		Critical: false,
	}

	// NTFS support (optional on macOS for better Windows compatibility)
	if runtime.GOOS == "darwin" {
		dm.optionalTools["mkfs.ntfs"] = ToolRequirement{
			Name:         "mkfs.ntfs",
			Commands:     []string{"mkfs.ntfs", "mkntfs"},
			Purpose:      "NTFS filesystem creation for true Windows compatibility",
			Installation: "brew install ntfs-3g",
			Critical:     false,
		}
	}
}

// CheckAllDependencies performs comprehensive dependency checking
func (dm *DependencyManager) CheckAllDependencies() error {
	logger.GetLogger().Info("Performing comprehensive dependency check...")

	// Check system capabilities first
	err := dm.checkSystemCapabilities()
	if err != nil {
		return fmt.Errorf("system capability check failed: %w", err)
	}

	// Check required tools
	err = dm.checkRequiredTools()
	if err != nil {
		return fmt.Errorf("required tool check failed: %w", err)
	}

	// Check optional tools (warnings only)
	dm.checkOptionalTools()

	// Generate summary report
	dm.generateDependencyReport()

	if len(dm.missingRequired) > 0 {
		return fmt.Errorf("missing required dependencies: %v", dm.missingRequired)
	}

	logger.GetLogger().Info("Dependency check completed successfully")
	return nil
}

// checkSystemCapabilities determines what the system can do
func (dm *DependencyManager) checkSystemCapabilities() error {
	dm.systemCapability.Platform = runtime.GOOS

	// Check admin rights
	dm.systemCapability.HasAdminRights = dm.checkAdminRights()
	if !dm.systemCapability.HasAdminRights {
		logger.GetLogger().Warn("Running without administrative privileges - some operations may fail")
	}

	// Check UEFI support
	dm.systemCapability.SupportsUEFI = dm.checkUEFISupport()

	logger.GetLogger().Info("System capabilities: Platform=%s, Admin=%v, UEFI=%v",
		dm.systemCapability.Platform,
		dm.systemCapability.HasAdminRights,
		dm.systemCapability.SupportsUEFI)

	return nil
}

// checkAdminRights determines if running with administrative privileges
func (dm *DependencyManager) checkAdminRights() bool {
	switch runtime.GOOS {
	case "linux", "darwin":
		return os.Getuid() == 0
	case "windows":
		// For Windows, we'd need to check token elevation
		// For now, assume true if we can write to system areas
		return true
	default:
		return false
	}
}

// checkUEFISupport determines if the system supports UEFI
func (dm *DependencyManager) checkUEFISupport() bool {
	// Check for EFI variables directory
	efiPaths := []string{
		"/sys/firmware/efi",
		"/proc/efi",
		"/sys/firmware/efi/vars",
	}

	for _, path := range efiPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// checkRequiredTools verifies all required tools are available
func (dm *DependencyManager) checkRequiredTools() error {
	logger.GetLogger().Info("Checking required system tools...")

	for name, req := range dm.requiredTools {
		found := false
		var foundCommand string

		for _, cmd := range req.Commands {
			if path, err := exec.LookPath(cmd); err == nil {
				found = true
				foundCommand = cmd
				logger.GetLogger().Debug("Found %s: %s at %s", name, cmd, path)
				break
			}
		}

		if !found {
			dm.missingRequired = append(dm.missingRequired, name)
			logger.GetLogger().Error("Missing required tool: %s (%s)", name, req.Purpose)
			logger.GetLogger().Error("Installation: %s", req.Installation)
		} else {
			// Test the tool works
			if err := dm.testTool(foundCommand, req); err != nil {
				logger.GetLogger().Warn("Tool %s found but may not work correctly: %v", foundCommand, err)
			}
		}
	}

	return nil
}

// checkOptionalTools verifies optional tools (warnings only)
func (dm *DependencyManager) checkOptionalTools() {
	logger.GetLogger().Info("Checking optional system tools...")

	for name, req := range dm.optionalTools {
		found := false
		var foundCommand string

		for _, cmd := range req.Commands {
			if path, err := exec.LookPath(cmd); err == nil {
				found = true
				foundCommand = cmd
				logger.GetLogger().Debug("Found optional tool %s: %s at %s", name, cmd, path)
				break
			}
		}

		if !found {
			dm.missingOptional = append(dm.missingOptional, name)
			logger.GetLogger().Warn("Missing optional tool: %s (%s)", name, req.Purpose)
			logger.GetLogger().Warn("Installation: %s", req.Installation)
		} else {
			// Test the tool works
			if err := dm.testTool(foundCommand, req); err != nil {
				logger.GetLogger().Warn("Optional tool %s found but may not work correctly: %v", foundCommand, err)
			}
		}
	}
}

// testTool performs a basic functionality test on a tool
func (dm *DependencyManager) testTool(command string, req ToolRequirement) error {
	// Create a context with timeout for testing
	switch command {
	case "parted":
		cmd := exec.Command(command, "--version")
		return dm.runTestCommand(cmd, 5*time.Second)
	case "lsblk":
		cmd := exec.Command(command, "--help")
		return dm.runTestCommand(cmd, 5*time.Second)
	case "7z", "7za", "7zr":
		cmd := exec.Command(command)
		err := dm.runTestCommand(cmd, 5*time.Second)
		// 7z returns non-zero when run without arguments, that's expected
		if err != nil && !strings.Contains(err.Error(), "exit status") {
			return err
		}
		return nil
	case "mount", "umount":
		// Don't test mount/umount as they might require root
		return nil
	default:
		cmd := exec.Command(command, "--version")
		err := dm.runTestCommand(cmd, 5*time.Second)
		if err != nil {
			// Try --help as fallback
			cmd = exec.Command(command, "--help")
			return dm.runTestCommand(cmd, 5*time.Second)
		}
		return nil
	}
}

// runTestCommand runs a command with timeout for testing
func (dm *DependencyManager) runTestCommand(cmd *exec.Cmd, timeout time.Duration) error {
	done := make(chan error, 1)

	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		cmd.Process.Kill()
		return fmt.Errorf("command timed out after %v", timeout)
	}
}

// generateDependencyReport creates a summary report
func (dm *DependencyManager) generateDependencyReport() {
	logger.GetLogger().Info("=== Dependency Check Summary ===")
	logger.GetLogger().Info("Platform: %s", dm.systemCapability.Platform)
	logger.GetLogger().Info("Admin Rights: %v", dm.systemCapability.HasAdminRights)
	logger.GetLogger().Info("UEFI Support: %v", dm.systemCapability.SupportsUEFI)

	totalRequired := len(dm.requiredTools)
	foundRequired := totalRequired - len(dm.missingRequired)
	logger.GetLogger().Info("Required Tools: %d/%d found", foundRequired, totalRequired)

	if len(dm.missingRequired) > 0 {
		logger.GetLogger().Error("Missing required tools: %v", dm.missingRequired)
	}

	totalOptional := len(dm.optionalTools)
	foundOptional := totalOptional - len(dm.missingOptional)
	logger.GetLogger().Info("Optional Tools: %d/%d found", foundOptional, totalOptional)

	if len(dm.missingOptional) > 0 {
		logger.GetLogger().Warn("Missing optional tools: %v", dm.missingOptional)
	}

	logger.GetLogger().Info("=== End Summary ===")

	// Add 2025 compatibility warnings
	dm.checkLegacyCompatibilityWarnings()
}

// checkLegacyCompatibilityWarnings provides warnings for 2025 Windows requirements
func (dm *DependencyManager) checkLegacyCompatibilityWarnings() {
	logger.GetLogger().Info("=== 2025 Windows USB Compatibility Check ===")

	// Warn about GPT/UEFI requirements for modern Windows
	if !dm.systemCapability.SupportsUEFI {
		logger.GetLogger().Warn("⚠️  LEGACY SYSTEM DETECTED:")
		logger.GetLogger().Warn("   • This system appears to be BIOS-only (no UEFI)")
		logger.GetLogger().Warn("   • Windows 11 requires UEFI boot mode")
		logger.GetLogger().Warn("   • USB creation will default to legacy MBR/BIOS compatibility")
		logger.GetLogger().Warn("   • Target machines must support BIOS boot for created USBs")
	}

	// Warn about admin rights needed for modern features
	if !dm.systemCapability.HasAdminRights {
		logger.GetLogger().Warn("⚠️  LIMITED PRIVILEGES DETECTED:")
		logger.GetLogger().Warn("   • Running without administrative/root privileges")
		logger.GetLogger().Warn("   • Some 2025 features may not work properly:")
		logger.GetLogger().Warn("     - GPT partition creation")
		logger.GetLogger().Warn("     - UEFI:NTFS dual-partition setup")
		logger.GetLogger().Warn("     - Large file (>4GB) handling")
		logger.GetLogger().Warn("   • Consider running with 'sudo' on macOS/Linux")
	}

	// Platform-specific warnings
	switch dm.systemCapability.Platform {
	case "darwin":
		logger.GetLogger().Info("✅ macOS: Good compatibility for Windows USB creation")
		if len(dm.missingOptional) > 0 {
			logger.GetLogger().Warn("💡 OPTIONAL: Install missing tools for enhanced features:")
			for _, tool := range dm.missingOptional {
				if req, exists := dm.optionalTools[tool]; exists {
					logger.GetLogger().Warn("   • %s: %s", tool, req.Purpose)
				}
			}
		}
	case "linux":
		logger.GetLogger().Info("✅ Linux: Experimental support for Windows USB creation")
		logger.GetLogger().Warn("💡 Linux users: Consider using WoeUSB for proven Windows USB creation")
	default:
		logger.GetLogger().Warn("⚠️  UNSUPPORTED PLATFORM: %s", dm.systemCapability.Platform)
		logger.GetLogger().Warn("   • Windows USB creation not tested on this platform")
		logger.GetLogger().Warn("   • Results may vary or fail completely")
	}

	logger.GetLogger().Info("=== End Compatibility Check ===")
}

// PromptForOptionalTools asks user if they want to install optional tools for better functionality
func (dm *DependencyManager) PromptForOptionalTools() bool {
	if len(dm.missingOptional) == 0 {
		return false
	}

	logger.GetLogger().Info("Optional tools found that would improve functionality:")
	for _, missing := range dm.missingOptional {
		if tool, exists := dm.optionalTools[missing]; exists {
			logger.GetLogger().Info("  - %s: %s", tool.Name, tool.Purpose)
		}
	}

	fmt.Print("Would you like to install these optional tools? (y/n): ")
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false
	}

	return response == "y" || response == "yes" || response == "Y"
}

// WarnAboutLegacyTargetCompatibility warns user about creating USBs for legacy systems
func (dm *DependencyManager) WarnAboutLegacyTargetCompatibility(useGPT bool, targetIsLegacy bool) {
	if !useGPT && targetIsLegacy {
		logger.GetLogger().Info("=== LEGACY TARGET COMPATIBILITY ===")
		logger.GetLogger().Warn("Creating USB for legacy BIOS systems:")
		logger.GetLogger().Warn("• Using MBR partition table (legacy compatibility)")
		logger.GetLogger().Warn("• Target system must support BIOS boot")
		logger.GetLogger().Warn("• Windows 11 may not install on legacy hardware")
		logger.GetLogger().Warn("• Consider upgrading target system to UEFI if possible")
		logger.GetLogger().Info("=== END LEGACY WARNING ===")
	} else if useGPT && !dm.systemCapability.SupportsUEFI {
		logger.GetLogger().Warn("=== CREATING MODERN USB ON LEGACY SYSTEM ===")
		logger.GetLogger().Warn("• Creating GPT/UEFI USB on BIOS-only creation system")
		logger.GetLogger().Warn("• USB will work on UEFI target systems")
		logger.GetLogger().Warn("• Creation system compatibility: Limited")
		logger.GetLogger().Info("=== END MODERN USB WARNING ===")
	}
}

// CheckModernWindowsRequirements validates system meets 2025 Windows requirements
func (dm *DependencyManager) CheckModernWindowsRequirements() []string {
	var warnings []string

	if !dm.systemCapability.SupportsUEFI {
		warnings = append(warnings, "UEFI boot not detected - Windows 11 requires UEFI")
	}

	if !dm.systemCapability.HasAdminRights {
		warnings = append(warnings, "Administrative privileges required for optimal USB creation")
	}

	if dm.systemCapability.Platform != "darwin" && dm.systemCapability.Platform != "linux" {
		warnings = append(warnings, fmt.Sprintf("Platform %s not officially supported", dm.systemCapability.Platform))
	}

	return warnings
}

// AttemptDependencyRecovery tries to install missing dependencies automatically
func (dm *DependencyManager) AttemptDependencyRecovery() error {
	if len(dm.missingRequired) == 0 {
		return nil
	}

	logger.GetLogger().Info("Attempting automatic dependency recovery...")

	switch runtime.GOOS {
	case "darwin":
		return dm.recoverMacOSDependencies()
	case "linux":
		return dm.recoverLinuxDependencies()
	default:
		return fmt.Errorf("automatic dependency recovery not supported on %s", runtime.GOOS)
	}
}

// recoverMacOSDependencies attempts to install missing tools on macOS
func (dm *DependencyManager) recoverMacOSDependencies() error {
	// Check if Homebrew is available
	if _, err := exec.LookPath("brew"); err != nil {
		return fmt.Errorf("Homebrew not found - please install Homebrew first")
	}

	logger.GetLogger().Info("Using Homebrew to install missing dependencies...")

	packages := []string{}

	// Handle required tools
	for _, missing := range dm.missingRequired {
		switch missing {
		case "parted", "lsblk", "wipefs", "blockdev":
			packages = append(packages, "parted", "util-linux")
		case "mkfs.fat":
			packages = append(packages, "dosfstools")
		case "mkfs.ntfs":
			packages = append(packages, "ntfs-3g")
		case "7z":
			packages = append(packages, "p7zip")
		}
	}

	// Ask about optional tools installation
	installOptional := dm.PromptForOptionalTools()
	if installOptional {
		for _, missing := range dm.missingOptional {
			switch missing {
			case "mkfs.ntfs":
				packages = append(packages, "ntfs-3g")
				logger.GetLogger().Info("Adding optional NTFS support for better Windows compatibility")
			case "mkfs.exfat":
				packages = append(packages, "exfat")
			case "sgdisk":
				packages = append(packages, "gptfdisk")
			}
		}
	}

	// Remove duplicates
	packages = removeDuplicates(packages)

	for _, pkg := range packages {
		logger.GetLogger().Info("Installing %s...", pkg)
		cmd := exec.Command("brew", "install", pkg)
		err := cmd.Run()
		if err != nil {
			logger.GetLogger().Warn("Failed to install %s: %v", pkg, err)
		}
	}

	return nil
}

// recoverLinuxDependencies attempts to install missing tools on Linux
func (dm *DependencyManager) recoverLinuxDependencies() error {
	// Detect package manager
	var packageManager string
	var installCmd []string

	if _, err := exec.LookPath("apt"); err == nil {
		packageManager = "apt"
		installCmd = []string{"apt", "install", "-y"}
	} else if _, err := exec.LookPath("yum"); err == nil {
		packageManager = "yum"
		installCmd = []string{"yum", "install", "-y"}
	} else if _, err := exec.LookPath("dnf"); err == nil {
		packageManager = "dnf"
		installCmd = []string{"dnf", "install", "-y"}
	} else if _, err := exec.LookPath("pacman"); err == nil {
		packageManager = "pacman"
		installCmd = []string{"pacman", "-S", "--noconfirm"}
	} else {
		return fmt.Errorf("no supported package manager found")
	}

	logger.GetLogger().Info("Using %s to install missing dependencies...", packageManager)

	packages := []string{}

	// Handle required tools
	for _, missing := range dm.missingRequired {
		switch missing {
		case "parted", "lsblk", "wipefs", "blockdev":
			if packageManager == "apt" {
				packages = append(packages, "parted", "util-linux")
			} else {
				packages = append(packages, "parted", "util-linux")
			}
		case "mkfs.fat":
			packages = append(packages, "dosfstools")
		case "mkfs.ntfs":
			packages = append(packages, "ntfs-3g")
		case "7z":
			if packageManager == "apt" {
				packages = append(packages, "p7zip-full")
			} else {
				packages = append(packages, "p7zip")
			}
		}
	}

	// Ask about optional tools installation
	installOptional := dm.PromptForOptionalTools()
	if installOptional {
		for _, missing := range dm.missingOptional {
			switch missing {
			case "mkfs.exfat":
				if packageManager == "apt" {
					packages = append(packages, "exfat-utils")
				} else {
					packages = append(packages, "exfat-utils")
				}
			case "sgdisk":
				if packageManager == "apt" {
					packages = append(packages, "gdisk")
				} else {
					packages = append(packages, "gdisk")
				}
			}
		}
	}

	// Remove duplicates
	packages = removeDuplicates(packages)

	cmd := append(installCmd, packages...)
	logger.GetLogger().Info("Running: %s", strings.Join(cmd, " "))

	execCmd := exec.Command(cmd[0], cmd[1:]...)
	err := execCmd.Run()
	if err != nil {
		return fmt.Errorf("package installation failed: %w", err)
	}

	return nil
}

// GetSystemInfo returns comprehensive system information
func (dm *DependencyManager) GetSystemInfo() map[string]interface{} {
	return map[string]interface{}{
		"platform":           dm.systemCapability.Platform,
		"admin_rights":       dm.systemCapability.HasAdminRights,
		"uefi_support":       dm.systemCapability.SupportsUEFI,
		"required_tools":     len(dm.requiredTools),
		"missing_required":   dm.missingRequired,
		"optional_tools":     len(dm.optionalTools),
		"missing_optional":   dm.missingOptional,
		"can_partition":      len(dm.missingRequired) == 0,
		"can_auto_recover":   runtime.GOOS == "darwin" || runtime.GOOS == "linux",
	}
}

// removeDuplicates removes duplicate strings from a slice
func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	result := []string{}

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}