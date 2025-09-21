package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/entro314-labs/ember/internal/logger"
	"github.com/entro314-labs/ember/internal/types"
)

// Import types from shared package
type FilesystemType = types.FilesystemType
type FilesystemAnalysis = types.FilesystemAnalysis

const (
	FilesystemFAT32 = types.FilesystemFAT32
	FilesystemNTFS  = types.FilesystemNTFS
	FilesystemExFAT = types.FilesystemExFAT
)

// FAT32 limitations
const (
	FAT32MaxFileSize     = 4*1024*1024*1024 - 1 // 4GB - 1 byte
	FAT32MaxPartitionSize = 2 * 1024 * 1024 * 1024 * 1024 // 2TB theoretical limit
)

// AnalyzeSourceMedia performs comprehensive analysis of the source media
func AnalyzeSourceMedia(sourcePath string) (*FilesystemAnalysis, error) {
	analysis := &FilesystemAnalysis{
		RecommendedFS:       FilesystemFAT32, // Default to FAT32
		RequiresUEFI_NTFS:   false,
		LargeFiles:          make([]string, 0),
		RequiresWorkarounds: make([]string, 0),
	}

	log := logger.GetLogger()
	log.Info("Starting comprehensive source media analysis")

	// Walk through all files and analyze
	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Warn("Error accessing path", "path", path, "error", err)
			return nil // Continue walking
		}

		if info.IsDir() {
			return nil
		}

		// Update counters
		analysis.FileCount++
		analysis.TotalSize += info.Size()

		// Track maximum file size
		if info.Size() > analysis.MaxFileSize {
			analysis.MaxFileSize = info.Size()
		}

		// Check for files exceeding FAT32 limits
		if info.Size() > FAT32MaxFileSize {
			relPath, _ := filepath.Rel(sourcePath, path)
			analysis.LargeFiles = append(analysis.LargeFiles, relPath)
			log.Warn("Large file detected", "file", relPath, "size", formatBytes(info.Size()))
		}

		// Check for specific Windows files
		fileName := strings.ToLower(info.Name())
		switch {
		case strings.HasSuffix(fileName, ".wim"):
			analysis.HasWIMFiles = true
			if fileName == "install.wim" {
				analysis.HasInstallWIM = true
				analysis.InstallWimSize = info.Size()
				log.Info("install.wim detected", "size", formatBytes(info.Size()))
			}
		case fileName == "bootmgr.efi":
			// Windows 8+ with EFI support
		case fileName == "cversion.ini":
			// Detect Windows version
			if version := detectWindowsVersion(path); version != "" {
				analysis.WindowsVersion = version
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to analyze source media: %w", err)
	}

	// Make filesystem recommendation based on analysis
	analysis.RecommendedFS = determineOptimalFilesystem(analysis)

	// Determine if UEFI:NTFS support is needed
	analysis.RequiresUEFI_NTFS = analysis.RecommendedFS == FilesystemNTFS

	// Check for required workarounds
	analysis.RequiresWorkarounds = determineRequiredWorkarounds(analysis)

	log.Info("Analysis complete", "files", analysis.FileCount, "total_size", formatBytes(analysis.TotalSize), "recommended_fs", analysis.RecommendedFS)

	return analysis, nil
}

// hasLargeInstallWim checks if install.wim exceeds 4GB FAT32 limit
func hasLargeInstallWim(analysis *FilesystemAnalysis) bool {
	const fat32Limit = 4 * 1024 * 1024 * 1024 // 4GB
	return analysis.InstallWimSize > fat32Limit
}

// determineOptimalFilesystem selects the best filesystem based on 2025 best practices
func determineOptimalFilesystem(analysis *FilesystemAnalysis) FilesystemType {
	log := logger.GetLogger()

	// Check for install.wim > 4GB (common in Windows 11) - triggers dual-partition scheme
	if analysis.HasInstallWIM && hasLargeInstallWim(analysis) {
		log.Info("Large install.wim detected (>4GB), dual-partition UEFI:NTFS scheme required")
		analysis.RequiresDualPartition = true
		analysis.RequiresUEFI_NTFS = true
		return FilesystemNTFS
	}

	// If any file exceeds FAT32 limit, use NTFS
	if len(analysis.LargeFiles) > 0 {
		log.Info("Files exceed FAT32 4GB limit, switching to NTFS")
		return FilesystemNTFS
	}

	// For very large total sizes, consider NTFS for better performance
	if analysis.TotalSize > 32*1024*1024*1024 { // 32GB threshold
		log.Info("Large total size detected, NTFS recommended for performance")
		return FilesystemNTFS
	}

	// Modern Windows 11 benefits from NTFS by default (2025 best practice)
	if isModernWindows(analysis.WindowsVersion) && analysis.HasInstallWIM {
		log.Info("Modern Windows detected, NTFS recommended for 2025 compatibility")
		return FilesystemNTFS
	}

	// Default to FAT32 only for legacy compatibility
	return FilesystemFAT32
}

// determineRequiredWorkarounds identifies needed compatibility fixes based on 2025 standards
func determineRequiredWorkarounds(analysis *FilesystemAnalysis) []string {
	var workarounds []string
	log := logger.GetLogger()

	// Windows 7 UEFI workaround (legacy support)
	if isWindows7(analysis.WindowsVersion) && analysis.HasInstallWIM {
		workarounds = append(workarounds, "windows7_uefi_bootloader")
		log.Info("Windows 7 detected: UEFI bootloader extraction required")
	}

	// Large install.wim handling (2025: common with Windows 11)
	if analysis.HasInstallWIM && hasLargeInstallWim(analysis) {
		workarounds = append(workarounds, "dual_partition_scheme")
		workarounds = append(workarounds, "uefi_ntfs_support")
		log.Info("Large install.wim detected: dual-partition UEFI:NTFS scheme required")
	}

	// Large file handling (any file >4GB)
	if len(analysis.LargeFiles) > 0 {
		workarounds = append(workarounds, "large_file_support")
		if !analysis.RequiresDualPartition {
			workarounds = append(workarounds, "ntfs_filesystem")
		}
	}

	// Modern Windows compatibility (2025 best practice)
	if isModernWindows(analysis.WindowsVersion) {
		workarounds = append(workarounds, "gpt_partition_table")
		workarounds = append(workarounds, "uefi_boot_support")
		log.Info("Modern Windows detected: GPT/UEFI configuration recommended")
	} else {
		// Legacy BIOS boot flag workaround (only for older Windows)
		workarounds = append(workarounds, "legacy_boot_flag")
		workarounds = append(workarounds, "mbr_compatibility")
	}

	return workarounds
}

// detectWindowsVersion attempts to determine Windows version from cversion.ini
func detectWindowsVersion(cversionPath string) string {
	content, err := os.ReadFile(cversionPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "MinServer=") {
			version := strings.TrimPrefix(line, "MinServer=")
			return parseWindowsVersion(version)
		}
	}

	return ""
}

// parseWindowsVersion converts version numbers to human-readable names
func parseWindowsVersion(version string) string {
	switch {
	case strings.HasPrefix(version, "7"):
		return "Windows 7"
	case strings.HasPrefix(version, "8"):
		return "Windows 8/8.1"
	case strings.HasPrefix(version, "10"):
		return "Windows 10"
	case strings.HasPrefix(version, "11"):
		return "Windows 11"
	default:
		return fmt.Sprintf("Windows (version %s)", version)
	}
}

// isWindows7 checks if the detected version is Windows 7
func isWindows7(version string) bool {
	return strings.Contains(strings.ToLower(version), "windows 7")
}

// isModernWindows checks if it's Windows 10 or newer (2025 standard)
func isModernWindows(version string) bool {
	lower := strings.ToLower(version)
	// 2025: Consider Windows 10+ as "modern" requiring UEFI/GPT
	// Windows 8/8.1 now considered legacy
	return strings.Contains(lower, "windows 10") ||
		   strings.Contains(lower, "windows 11") ||
		   strings.Contains(lower, "windows server 2016") ||
		   strings.Contains(lower, "windows server 2019") ||
		   strings.Contains(lower, "windows server 2022") ||
		   strings.Contains(lower, "windows server 2025")
}

// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// UserPreferences contains user's preferred settings
type UserPreferences struct {
	PreferredFilesystem FilesystemType // User's preferred filesystem (empty = auto)
	AlwaysPrompt        bool           // Always ask before switching filesystems
	PerformanceMode     bool           // Optimize for speed vs compatibility
	AllowLargeFiles     bool           // Allow NTFS for files >4GB
	QuickFormat         bool           // Use quick format by default
	AutoDetectVersion   bool           // Auto-detect Windows version requirements
}

// FilesystemCreationOptions contains parameters for filesystem creation
type FilesystemCreationOptions struct {
	Type          FilesystemType
	Label         string
	QuickFormat   bool
	ClusterSize   int
	EnableUEFI    bool
	RequiresNTFS  bool
	UserOverride  bool // User explicitly chose this filesystem
}

// GetFilesystemCreationOptions returns optimal creation parameters with user preferences
func GetFilesystemCreationOptions(analysis *FilesystemAnalysis, prefs *UserPreferences, label string) *FilesystemCreationOptions {
	recommendedFS := determineFilesystemWithPreferences(analysis, prefs)

	options := &FilesystemCreationOptions{
		Type:        recommendedFS,
		Label:       label,
		QuickFormat: prefs.QuickFormat,
		EnableUEFI:  true, // Enable UEFI support by default
	}

	// Apply user override if they have a strong preference
	if prefs.PreferredFilesystem != "" && prefs.PreferredFilesystem != recommendedFS {
		// Check if user preference is viable
		if isFilesystemViable(prefs.PreferredFilesystem, analysis) {
			options.Type = prefs.PreferredFilesystem
			options.UserOverride = true
		}
	}

	switch options.Type {
	case FilesystemFAT32:
		if prefs.PerformanceMode {
			options.ClusterSize = 32768 // 32KB cluster for better performance
		} else {
			options.ClusterSize = 4096  // 4KB cluster for compatibility
		}
	case FilesystemNTFS:
		options.ClusterSize = 4096  // 4KB cluster for NTFS
		options.RequiresNTFS = true
	case FilesystemExFAT:
		options.ClusterSize = 32768 // 32KB cluster for exFAT
	}

	return options
}

// determineFilesystemWithPreferences applies user preferences to filesystem selection
func determineFilesystemWithPreferences(analysis *FilesystemAnalysis, prefs *UserPreferences) FilesystemType {
	log := logger.GetLogger()

	// If user has disabled large file support, force FAT32 if possible
	if !prefs.AllowLargeFiles && len(analysis.LargeFiles) > 0 {
		log.Warn("Large files detected but user disabled NTFS support")
		// This will likely fail, but respect user preference
		return FilesystemFAT32
	}

	// If user prefers performance, consider NTFS for large ISOs even without large files
	if prefs.PerformanceMode && analysis.TotalSize > 16*1024*1024*1024 { // 16GB threshold
		log.Info("Performance mode: recommending NTFS for large ISO")
		return FilesystemNTFS
	}

	// Use standard analysis
	return determineOptimalFilesystem(analysis)
}

// isFilesystemViable checks if a filesystem choice is viable for the given ISO
func isFilesystemViable(fs FilesystemType, analysis *FilesystemAnalysis) bool {
	switch fs {
	case FilesystemFAT32:
		// FAT32 can't handle files >4GB
		if len(analysis.LargeFiles) > 0 {
			log := logger.GetLogger()
			log.Warn("FAT32 cannot handle files larger than 4GB")
			return false
		}
		return true
	case FilesystemNTFS, FilesystemExFAT:
		// These can handle any size
		return true
	default:
		return false
	}
}

// GetDefaultUserPreferences returns sensible defaults
func GetDefaultUserPreferences() *UserPreferences {
	return &UserPreferences{
		PreferredFilesystem: "", // Auto-detect
		AlwaysPrompt:        false,
		PerformanceMode:     false,
		AllowLargeFiles:     true,
		QuickFormat:         true,
		AutoDetectVersion:   true,
	}
}

// ValidateUserPreferences ensures preferences are sensible
func ValidateUserPreferences(prefs *UserPreferences) error {
	if prefs.PreferredFilesystem != "" {
		switch prefs.PreferredFilesystem {
		case FilesystemFAT32, FilesystemNTFS, FilesystemExFAT:
			// Valid
		default:
			return fmt.Errorf("invalid preferred filesystem: %s", prefs.PreferredFilesystem)
		}
	}
	return nil
}

// Import callback type from shared package
type FileProgressCallback = types.FileProgressCallback

// CopyWithProgress copies files with progress reporting and large file handling
func CopyWithProgress(srcDir, dstDir string, callback FileProgressCallback) error {
	var totalSize int64
	var copiedSize int64

	// First pass: calculate total size
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		totalSize += info.Size()
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to calculate total size: %w", err)
	}

	// Second pass: copy files with progress
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Report progress
		if callback != nil {
			callback(copiedSize, totalSize, relPath)
		}

		// Copy file with appropriate method based on size
		if info.Size() > 100*1024*1024 { // 100MB threshold for chunked copy
			err = copyLargeFile(path, dstPath, info.Size(), func(copied int64) {
				if callback != nil {
					callback(copiedSize+copied, totalSize, relPath)
				}
			})
		} else {
			err = copySmallFile(path, dstPath)
		}

		if err != nil {
			return fmt.Errorf("failed to copy %s: %w", relPath, err)
		}

		copiedSize += info.Size()
		return nil
	})
}

// copyLargeFile handles large files with progress reporting and cancellation support
func copyLargeFile(src, dst string, totalSize int64, progressCallback func(int64)) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	buffer := make([]byte, 5*1024*1024) // 5MB chunks for better performance
	var copied int64

	for {
		n, err := srcFile.Read(buffer)
		if n > 0 {
			if _, writeErr := dstFile.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}
			copied += int64(n)

			if progressCallback != nil {
				progressCallback(copied)
			}
		}

		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}
	}

	return nil
}

// copySmallFile handles small files efficiently
func copySmallFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, 0644)
}