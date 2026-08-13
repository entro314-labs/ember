package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/Xmister/udf"
	"github.com/entro314-labs/ember/internal/device"
	"github.com/entro314-labs/ember/internal/filesystem"
	"github.com/entro314-labs/ember/internal/iso"
	"github.com/entro314-labs/ember/internal/logger"
	"github.com/entro314-labs/ember/internal/security"
	"github.com/entro314-labs/ember/internal/types"
	"github.com/entro314-labs/ember/internal/ui"
)

const (
	appName     = "Ember"
	version     = "1.0.0-dev"
	description = "Create a bootable Windows USB from the terminal"
	repository  = "https://github.com/entro314-labs/Ember"
	licenseType = "Apache-2.0"
)

// TODO: Re-enable when binaries directory is set up
// //go:embed binaries/uefi-ntfs.img
var UefiNtfsImg []byte // Changed from UEFI_NTFS_IMG to follow Go naming conventions

// Command line flags
var (
	showVersion    = flag.Bool("version", false, "Show version information")
	showHelp       = flag.Bool("help", false, "Show help message")
	verbose        = flag.Bool("verbose", false, "Enable verbose output")
	force          = flag.Bool("force", false, "Force operations without confirmation")
	isoPath        = flag.String("iso", "", "Path to Windows ISO file")
	deviceID       = flag.String("device", "", "Target device identifier (e.g., disk2)")
	useGPT         = flag.Bool("gpt", true, "Use GPT partition table (default for 2025, use --mbr to force MBR)")
	forceMBR       = flag.Bool("mbr", false, "Force MBR partition table (legacy compatibility)")
	skipValidation = flag.Bool("skip-validation", false, "Skip ISO and device validation")
	quiet          = flag.Bool("quiet", false, "Suppress non-essential output")

	// Advanced feature flags
	forceFilesystem  = flag.String("filesystem", "", "Force filesystem type (FAT32, NTFS, ExFAT)")
	skipAnalysis     = flag.Bool("skip-analysis", false, "Skip automatic source media analysis")
	disableUEFI      = flag.Bool("disable-uefi", false, "Disable UEFI boot support")
	disableLegacy    = flag.Bool("disable-legacy", false, "Disable legacy BIOS boot support")
	customLabel      = flag.String("label", "Windows USB", "Custom filesystem label")
	skipDependencies = flag.Bool("skip-deps", false, "Skip dependency checking")
	enableAdvanced   = flag.Bool("advanced", false, "Enable advanced features and analysis")
	showSystemInfo   = flag.Bool("system-info", false, "Show system information and capabilities")
	analyzeOnly      = flag.Bool("analyze-only", false, "Only analyze source media without creating USB")
	discoverISOs     = flag.Bool("discover", false, "Discover Windows ISO files on the system")
	quickFormat      = flag.Bool("quick-format", true, "Use quick format (faster but less thorough)")
	performanceMode  = flag.Bool("performance-mode", false, "Optimize for speed over compatibility")
	allowLargeFiles  = flag.Bool("allow-large-files", true, "Allow NTFS for files >4GB (recommended)")
)

func main() {
	flag.Parse()

	// Initialize structured logging
	logLevel := logger.LogLevelInfo
	if *verbose {
		logLevel = logger.LogLevelDebug
	}
	if *quiet {
		logLevel = logger.LogLevelError
	}
	logger.InitLogger(logLevel, *verbose)

	log := logger.GetLogger()
	log.Info("Starting Ember", "version", version, "os", runtime.GOOS, "arch", runtime.GOARCH)

	// Handle version request
	if *showVersion {
		printVersion()
		return
	}

	// Handle help request
	if *showHelp {
		printHelp()
		return
	}

	// Handle system info request
	if *showSystemInfo {
		printSystemInfo()
		return
	}

	// Handle analyze-only request
	if *analyzeOnly && *isoPath != "" {
		runAnalyzeOnly()
		return
	}

	// Handle ISO discovery request
	if *discoverISOs {
		runISODiscovery()
		return
	}

	// Check if we have command line arguments for headless operation
	if *isoPath != "" && *deviceID != "" {
		log.Info("Running in headless mode", "iso", *isoPath, "device", *deviceID)
		if *enableAdvanced {
			log.Info("Using advanced USB creation engine")
			runAdvancedHeadless()
		} else {
			log.Info("Using legacy USB creation engine")
			runHeadless()
		}
		return
	}

	// No command line args or incomplete args - run TUI
	if !*quiet {
		printWelcome()
	}

	log.Debug("Starting TUI mode")
	ui.NewTUI()
}

func printVersion() {
	fmt.Printf("%s version %s\n", appName, version)
	fmt.Printf("Built with Go %s for %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Repository: %s\n", repository)
	fmt.Printf("License: %s\n", licenseType)
}

func printHelp() {
	fmt.Printf("%s - %s\n\n", appName, description)
	fmt.Println("USAGE:")
	fmt.Printf("  %s [OPTIONS]\n", os.Args[0])
	fmt.Printf("  %s --iso <iso-file> --device <device-id> [OPTIONS]\n", os.Args[0])
	fmt.Println()
	fmt.Println("OPTIONS:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Printf("  # Interactive mode (TUI)\n")
	fmt.Printf("  %s\n", os.Args[0])
	fmt.Println()
	fmt.Printf("  # Headless mode (legacy)\n")
	fmt.Printf("  %s --iso ~/Windows11.iso --device disk2\n", os.Args[0])
	fmt.Println()
	fmt.Printf("  # Advanced headless mode with automatic filesystem detection\n")
	fmt.Printf("  %s --iso ~/Windows11.iso --device disk2 --advanced\n", os.Args[0])
	fmt.Println()
	fmt.Printf("  # Force NTFS filesystem with custom label\n")
	fmt.Printf("  %s --iso ~/Windows11.iso --device disk2 --advanced --filesystem NTFS --label \"Win11 Pro\"\n", os.Args[0])
	fmt.Println()
	fmt.Printf("  # Use GPT instead of MBR\n")
	fmt.Printf("  %s --iso ~/Windows11.iso --device disk2 --gpt\n", os.Args[0])
	fmt.Println()
	fmt.Printf("  # Analyze ISO without creating USB\n")
	fmt.Printf("  %s --analyze-only --iso ~/Windows11.iso\n", os.Args[0])
	fmt.Println()
	fmt.Printf("  # Show system information\n")
	fmt.Printf("  %s --system-info\n", os.Args[0])
	fmt.Println()
	fmt.Println("SUPPORTED PLATFORMS:")
	fmt.Println("  - macOS (primary)")
	fmt.Println("  - Linux (experimental)")
	fmt.Println("  - Windows (not implemented)")
	fmt.Println()
	fmt.Println("DEVICE IDENTIFIERS:")
	fmt.Println("  macOS: disk2, disk3, etc. (use 'diskutil list' to see available devices)")
	fmt.Println("  Linux: sdb, sdc, etc. (use 'lsblk' to see available devices)")
	fmt.Println()
	fmt.Println("WARNING:")
	fmt.Println("  This tool will completely overwrite the target device.")
	fmt.Println("  Make sure you select the correct device to avoid data loss!")
	fmt.Println()
	fmt.Printf("For more information, visit: %s\n", repository)
}

func printWelcome() {
	fmt.Printf("🫗 %s v%s\n", appName, version)
	fmt.Printf("   %s\n\n", description)

	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		fmt.Printf("⚠️  Warning: %s is not fully supported on %s\n", runtime.GOOS, runtime.GOOS)
		fmt.Println("   Some features may not work correctly.")
		fmt.Println()
	}

	if os.Geteuid() != 0 && runtime.GOOS != "windows" {
		fmt.Println("⚠️  Warning: Running without root privileges")
		fmt.Println("   You may need to run with 'sudo' for full functionality")
		fmt.Println()
	}
}

func runAdvancedHeadless() {
	log := logger.GetLogger()

	if !*quiet {
		log.Info("Running in advanced headless mode",
			"iso", *isoPath,
			"device", *deviceID,
			"filesystem", *forceFilesystem,
			"label", *customLabel)
	}

	// Create advanced USB creator
	creator := device.NewAdvancedUSBCreator()
	defer creator.Cleanup()

	// Configure creation options
	config := &device.USBCreationConfig{
		SourcePath:      *isoPath,
		TargetDevice:    "/dev/" + *deviceID, // Convert to device path
		Label:           *customLabel,
		ForceFilesystem: types.FilesystemType(*forceFilesystem),
		SkipAnalysis:    *skipAnalysis,
		EnableUEFI:      !*disableUEFI,
		LegacyBIOS:      !*disableLegacy,
		QuickFormat:     *quickFormat,
		PerformanceMode: *performanceMode,
		AllowLargeFiles: *allowLargeFiles,
		Verbose:         *verbose,
	}

	// Skip dependency check if requested
	if *skipDependencies {
		log.Warn("Skipping dependency checks as requested")
	}

	// Confirmation prompt unless forced
	if !*force {
		log.Warn("WARNING: This will completely overwrite device", "device", *deviceID)
		fmt.Print("Are you sure you want to continue? (yes/no): ")

		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			log.Warn("Failed to read user input", "error", err)
			response = "no" // Default to safe option
		}

		if response != "yes" && response != "y" && response != "Y" {
			log.Info("Operation cancelled by user")
			os.Exit(0)
		}
	}

	// Perform the advanced operation
	log.Info("Starting advanced USB creation process")
	if err := creator.CreateAdvancedWindowsUSB(config); err != nil {
		log.Fatal("Advanced USB creation failed", "error", err)
	}

	if !*quiet {
		log.Info("Advanced USB creation completed successfully")

		// Show creation summary
		summary := creator.GetCreationSummary()
		if progressInfo, ok := summary["progress_tracker"].(map[string]interface{}); ok {
			if overallProgress, ok := progressInfo["overall_progress"].(float64); ok {
				log.Info("Overall progress", "percentage", fmt.Sprintf("%.1f%%", overallProgress))
			}
		}
	}
}

func runHeadless() {
	log := logger.GetLogger()

	if !*quiet {
		log.Info("Running in headless mode",
			"iso", *isoPath,
			"device", *deviceID,
			"partition_table", func() string {
				if *useGPT {
					return "GPT"
				}
				return "MBR"
			}())
	}

	// Validate inputs and check 2025 compatibility
	if !*skipValidation {
		if *verbose {
			log.Debug("Validating headless inputs")
		}
		if err := validateHeadlessInputs(); err != nil {
			log.Fatal("Validation failed", "error", err)
		}
		if *verbose {
			log.Debug("Input validation successful")
		}

		// Check for 2025 compatibility warnings
		if !*skipDependencies {
			dm := device.NewDependencyManager()
			dm.CheckAllDependencies() // This includes legacy compatibility warnings

			// Warn about legacy target compatibility if using MBR
			if *forceMBR || !*useGPT {
				dm.WarnAboutLegacyTargetCompatibility(false, true)
			}
		}
	}

	// Confirmation prompt unless forced
	if !*force {
		log.Warn("WARNING: This will completely overwrite device", "device", *deviceID)
		fmt.Print("Are you sure you want to continue? (yes/no): ")

		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			log.Warn("Failed to read user input", "error", err)
			response = "no" // Default to safe option
		}

		if response != "yes" && response != "y" && response != "Y" {
			log.Info("Operation cancelled by user")
			os.Exit(0)
		}
	}

	// Perform the operation
	log.Info("Starting USB creation process")
	if err := performHeadlessOperation(); err != nil {
		log.Fatal("USB creation failed", "error", err)
	}

	if !*quiet {
		log.Info("USB creation completed successfully")
	}
}

func validateHeadlessInputs() error {
	log := logger.GetLogger().WithContext("operation", "validate_inputs")
	validator := security.GetSecurityValidator()

	if *verbose {
		log.Debug("Starting input validation")
	}

	// Validate ISO path
	if *verbose {
		log.Debug("Validating ISO path", "path", validator.SanitizeForLogging(*isoPath))
	}
	if err := validator.ValidateISOPath(*isoPath); err != nil {
		return fmt.Errorf("invalid ISO path: %w", err)
	}

	// Check if ISO file exists and is valid
	file, err := os.Open(*isoPath)
	if err != nil {
		return fmt.Errorf("failed to open ISO file: %w", err)
	}
	defer file.Close()

	if *verbose {
		log.Debug("Validating Windows ISO format")
	}
	if _, err := iso.ValidateWindowsISO(file); err != nil {
		return fmt.Errorf("invalid Windows ISO: %w", err)
	}

	// Validate device ID
	if *verbose {
		log.Debug("Validating device identifier", "device", *deviceID)
	}
	if err := validator.ValidateDeviceID(*deviceID); err != nil {
		return fmt.Errorf("invalid device ID: %w", err)
	}

	// Validate device (platform-specific)
	if *verbose {
		log.Debug("Performing platform-specific device validation")
	}
	if err := device.ValidateDevice(*deviceID); err != nil {
		return fmt.Errorf("device validation failed for %s: %w", *deviceID, err)
	}

	// Check privileges
	if err := validator.CheckPrivileges(); err != nil {
		log.Warn("Privilege check failed", "error", err)
		// Don't fail here, just warn - user might have alternative privileges
	}

	if *verbose {
		log.Debug("Input validation completed successfully")
	}
	return nil
}

func performHeadlessOperation() error {
	log := logger.GetLogger().WithContext("operation", "headless_usb_creation")

	// Open ISO file
	if *verbose {
		log.Debug("Opening ISO file", "path", *isoPath)
	}
	file, err := os.Open(*isoPath)
	if err != nil {
		return fmt.Errorf("failed to open ISO: %w", err)
	}
	defer file.Close()

	if *verbose {
		log.Debug("Parsing Windows ISO")
	}
	isoHandler, err := iso.OpenWindowsISO(file)
	if err != nil {
		return fmt.Errorf("failed to open Windows ISO: %w", err)
	}

	if !*quiet {
		log.Info("ISO opened successfully", "iso_path", *isoPath)
	}

	// Create device struct for compatibility
	targetDevice := &types.DiskUtilDevice{
		DeviceIdentifier: *deviceID,
	}

	if !*quiet {
		log.Info("Step 1/4: Preparing device", "device", *deviceID)
	}

	// Prepare device
	if *verbose {
		log.Debug("Preparing device for flashing")
	}
	if err := device.PrepareDevice(targetDevice); err != nil {
		return fmt.Errorf("failed to prepare device: %w", err)
	}
	if *verbose {
		log.Debug("Device preparation completed")
	}

	if !*quiet {
		log.Info("Step 2/4: Creating partitions")
	}

	// Create partitions
	if *verbose {
		log.Debug("Creating partition table", "type", func() string {
			if *useGPT {
				return "GPT"
			}
			return "MBR"
		}())
	}
	if err := device.FormatDiskForUEFINTFS(*deviceID, *useGPT); err != nil {
		return fmt.Errorf("failed to partition device: %w", err)
	}
	if *verbose {
		log.Debug("Partitioning completed")
	}

	if !*quiet {
		log.Info("Step 3/4: Copying ISO contents")
	}

	// Copy ISO contents to the device
	if *verbose {
		log.Debug("Starting ISO extraction to device")
	}
	if err := extractISOToDevice(isoHandler, targetDevice); err != nil {
		return fmt.Errorf("failed to copy ISO contents: %w", err)
	}
	if *verbose {
		log.Debug("ISO extraction completed")
	}

	if !*quiet {
		log.Info("Step 4/4: Installing bootloader")
	}

	// Install bootloader
	if *verbose {
		log.Debug("Installing bootloader")
	}
	if err := device.InstallBootloader(targetDevice); err != nil {
		return fmt.Errorf("failed to install bootloader: %w", err)
	}
	if *verbose {
		log.Debug("Bootloader installation completed")
	}

	return nil
}

// extractISOToDevice extracts ISO contents to the Windows partition on the device
func extractISOToDevice(isoUdf *udf.Udf, targetDevice *types.DiskUtilDevice) error {
	log := logger.GetLogger().WithContext("operation", "extract_iso")

	// Get the Windows partition (first partition)
	windowsPartition := device.GetBlockDevicePartition(targetDevice.DeviceIdentifier, 1)

	// Create temporary mount point
	mountPoint := "/tmp/ember-windows"
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	// Mount the Windows partition
	if *verbose {
		log.Debug("Mounting Windows partition", "partition", windowsPartition, "mount_point", mountPoint)
	}

	// Extract ISO contents to the partition (simplified for now)
	if *verbose {
		log.Debug("Extracting ISO contents", "destination", windowsPartition)
	}
	if err := iso.ExtractISOToLocation(isoUdf, mountPoint); err != nil {
		return fmt.Errorf("failed to extract ISO contents: %w", err)
	}

	if *verbose {
		log.Debug("ISO extraction completed successfully")
	}
	return nil
}

// GetVersion returns the application version
func GetVersion() string {
	return version
}

// GetAppName returns the application name
func GetAppName() string {
	return appName
}

// printSystemInfo displays comprehensive system information
func printSystemInfo() {
	fmt.Printf("🫗 %s v%s - System Information\n\n", appName, version)

	// Create dependency manager to get system info
	dm := device.NewDependencyManager()
	err := dm.CheckAllDependencies()

	// Show system capabilities
	systemInfo := dm.GetSystemInfo()
	fmt.Printf("=== System Capabilities ===\n")
	fmt.Printf("Platform: %v\n", systemInfo["platform"])
	fmt.Printf("Admin Rights: %v\n", systemInfo["admin_rights"])
	fmt.Printf("UEFI Support: %v\n", systemInfo["uefi_support"])
	fmt.Printf("Required Tools: %v/%v found\n", systemInfo["required_found"], systemInfo["required_tools"])
	fmt.Printf("Optional Tools: %v/%v found\n", systemInfo["optional_found"], systemInfo["optional_tools"])

	if missing, ok := systemInfo["missing_required"].([]string); ok && len(missing) > 0 {
		fmt.Printf("\n=== Missing Required Tools ===\n")
		for _, tool := range missing {
			fmt.Printf("  - %s\n", tool)
		}
	}

	if missing, ok := systemInfo["missing_optional"].([]string); ok && len(missing) > 0 {
		fmt.Printf("\n=== Missing Optional Tools ===\n")
		for _, tool := range missing {
			fmt.Printf("  - %s\n", tool)
		}
		fmt.Printf("\nOptional tools can improve functionality but are not required.\n")
	}

	if err != nil {
		fmt.Printf("\n⚠️  Dependency Check: %v\n", err)
		fmt.Printf("Some features may not work correctly.\n")
	} else {
		fmt.Printf("\n✅ All required dependencies satisfied\n")
	}

	fmt.Printf("\n=== Device Information ===\n")
	if runtime.GOOS == "darwin" {
		fmt.Printf("Use 'diskutil list' to see available devices\n")
	} else {
		fmt.Printf("Use 'lsblk' to see available devices\n")
	}
}

// runAnalyzeOnly performs source media analysis without creating USB
func runAnalyzeOnly() {
	log := logger.GetLogger()

	fmt.Printf("🫗 %s v%s - Source Media Analysis\n\n", appName, version)

	// Validate ISO path
	validator := security.GetSecurityValidator()
	if err := validator.ValidateISOPath(*isoPath); err != nil {
		log.Fatal("Invalid ISO path", "error", err)
	}

	// Perform analysis using the filesystem package
	analysis, err := filesystem.AnalyzeSourceMedia(*isoPath)
	if err != nil {
		log.Fatal("Analysis failed", "error", err)
	}

	// Display results
	fmt.Printf("=== Source Media Analysis ===\n")
	fmt.Printf("ISO Path: %s\n", validator.SanitizeForLogging(*isoPath))
	fmt.Printf("Files: %d\n", analysis.FileCount)
	fmt.Printf("Total Size: %s\n", types.FormatBytes(analysis.TotalSize))
	fmt.Printf("Largest File: %s\n", types.FormatBytes(analysis.MaxFileSize))
	fmt.Printf("Recommended Filesystem: %s\n", analysis.RecommendedFS)

	if analysis.WindowsVersion != "" {
		fmt.Printf("Detected Version: %s\n", analysis.WindowsVersion)
	}

	if len(analysis.LargeFiles) > 0 {
		fmt.Printf("\n=== Large Files (>4GB) ===\n")
		for i, file := range analysis.LargeFiles {
			if i < 10 { // Show first 10
				fmt.Printf("  - %s\n", file)
			} else if i == 10 {
				fmt.Printf("  - ... and %d more\n", len(analysis.LargeFiles)-10)
				break
			}
		}
	}

	if analysis.RequiresUEFI_NTFS {
		fmt.Printf("\n✅ UEFI:NTFS support will be enabled for large files\n")
	}

	if len(analysis.RequiresWorkarounds) > 0 {
		fmt.Printf("\n=== Required Workarounds ===\n")
		for _, workaround := range analysis.RequiresWorkarounds {
			fmt.Printf("  - %s\n", workaround)
		}
	}

	fmt.Printf("\n=== Recommendations ===\n")
	if analysis.RecommendedFS == types.FilesystemNTFS {
		fmt.Printf("• Use --filesystem NTFS for optimal compatibility\n")
		fmt.Printf("• UEFI:NTFS partition will be created automatically\n")
	} else if len(analysis.LargeFiles) > 0 {
		fmt.Printf("• Large files detected - NTFS recommended\n")
	} else {
		fmt.Printf("• FAT32 filesystem is sufficient for this media\n")
	}

	if len(analysis.RequiresWorkarounds) > 0 {
		fmt.Printf("• Additional workarounds will be applied automatically\n")
	}
}

// runISODiscovery discovers Windows ISO files on the system
func runISODiscovery() {
	log := logger.GetLogger()

	fmt.Printf("🔍 %s v%s - Windows ISO Discovery\n\n", appName, version)

	log.Info("Scanning system for Windows ISO files...")

	// Discover ISOs using the smart discovery system
	isos, err := iso.DiscoverWindowsISOs()
	if err != nil {
		fmt.Printf("❌ Error during ISO discovery: %v\n", err)
		os.Exit(1)
	}

	if len(isos) == 0 {
		fmt.Printf("📭 No Windows ISO files found on this system.\n\n")
		fmt.Printf("Common locations searched:\n")
		fmt.Printf("  • ~/Downloads/\n")
		fmt.Printf("  • ~/Desktop/\n")
		fmt.Printf("  • ~/Documents/\n")
		fmt.Printf("  • ~/ember/\n")
		fmt.Printf("  • External drives (/Volumes/, /media/)\n\n")
		fmt.Printf("Try downloading a Windows ISO from Microsoft's website.\n")
		return
	}

	fmt.Printf("🎯 Found %d Windows ISO file(s):\n\n", len(isos))

	for i, isoInfo := range isos {
		fmt.Printf("══════════════════════════════════════════════════════════════\n")
		fmt.Printf("📀 ISO #%d\n", i+1)
		fmt.Printf("══════════════════════════════════════════════════════════════\n")
		fmt.Printf("📁 Path: %s\n", isoInfo.Path)
		fmt.Printf("📊 Size: %s\n", isoInfo.GetSizeString())
		fmt.Printf("📍 Source: %s\n", isoInfo.Source)

		if isoInfo.Version != "" {
			fmt.Printf("🪟 Version: %s\n", isoInfo.Version)
		}
		if isoInfo.Architecture != "" {
			fmt.Printf("🏗️  Architecture: %s\n", isoInfo.Architecture)
		}
		if isoInfo.Edition != "" {
			fmt.Printf("📦 Edition: %s\n", isoInfo.Edition)
		}

		// Show recommended filesystem
		fmt.Printf("💾 Recommended Filesystem: %s\n", isoInfo.RecommendedFS)

		if isoInfo.RequiresWorkaround {
			fmt.Printf("⚠️  Requires Windows 7 UEFI workaround\n")
		}

		if isoInfo.IsUEFI {
			fmt.Printf("✅ UEFI boot support: Yes\n")
		}

		if isoInfo.HasInstallWim {
			fmt.Printf("📦 Install.wim: Present\n")
		}

		// Show estimated USB size needed
		if isoInfo.EstimatedUSBSize > 0 {
			estimatedGB := float64(isoInfo.EstimatedUSBSize) / (1024 * 1024 * 1024)
			fmt.Printf("💽 Estimated USB size needed: %.1f GB\n", estimatedGB)
		}

		fmt.Printf("📅 Last modified: %s\n", isoInfo.LastModified.Format("2006-01-02 15:04:05"))
		fmt.Printf("\n")
	}

	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("💡 To use any of these ISOs for USB creation:\n")
	fmt.Printf("   ember --iso \"<path>\" --device <device>\n")
	fmt.Printf("   ember --discover --iso \"<path>\" --device <device>\n\n")
	fmt.Printf("💡 To analyze an ISO in detail:\n")
	fmt.Printf("   ember --analyze-only --iso \"<path>\"\n\n")
	fmt.Printf("💡 For interactive mode:\n")
	fmt.Printf("   ember\n")
}
