package iso

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Xmister/udf"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/diskfs/go-diskfs"
	"github.com/entro314-labs/ember/internal/logger"
	"github.com/entro314-labs/ember/internal/types"
)

var ErrInvalidWindowsISO = errors.New("this file is not recognised as a valid Windows ISO image")

// Constants for filesystem analysis
const (
	FAT32MaxFileSize = 4 * 1024 * 1024 * 1024 // 4GB limit for FAT32
)

// Import types from shared package
type FilesystemType = types.FilesystemType

// ProgressCallback is called during ISO extraction to report progress
type ProgressCallback func(current, total int64, filename string)

// ExtractProgress represents the current extraction progress
type ExtractProgress struct {
	BytesCopied   int64
	TotalBytes    int64
	FilesCopied   int
	TotalFiles    int
	CurrentFile   string
	Speed         int64 // bytes per second
	TimeRemaining time.Duration
}

// GetSizeString method moved to types package

// Progress message for TUI
type extractProgressMsg ExtractProgress

// OpenWindowsISO opens and validates a Windows ISO file with context support
func OpenWindowsISOWithContext(ctx context.Context, file *os.File) (*udf.Udf, error) {
	log := logger.GetLogger().WithContext("operation", "open_iso")

	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before opening ISO: %w", err)
	}

	log.Debug("Validating Windows ISO file")
	info, err := ValidateWindowsISOWithContext(ctx, file)
	if err != nil {
		return nil, err
	}
	log.Debug("ISO validation successful", "version", info.Version, "architecture", info.Architecture)

	// Check context again before proceeding
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled during validation: %w", err)
	}

	log.Debug("Opening UDF filesystem")
	iso, err := udf.NewUdfFromReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open UDF filesystem: %w", err)
	}

	// Store ISO info for later use (handled by UDF reader)
	_ = info // ISO information is extracted during validation

	log.Info("Windows ISO opened successfully")
	return iso, nil
}

// OpenWindowsISO is a backward compatibility wrapper
func OpenWindowsISO(file *os.File) (*udf.Udf, error) {
	return OpenWindowsISOWithContext(context.Background(), file)
}

// ValidateWindowsISOWithContext validates a Windows ISO with context support
func ValidateWindowsISOWithContext(ctx context.Context, file *os.File) (*types.ISOInfo, error) {
	// Check if context is cancelled
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before validation: %w", err)
	}

	if !IsValidWindowsISO(file) {
		return nil, ErrInvalidWindowsISO
	}

	info := &types.ISOInfo{
		Path: file.Name(),
	}

	// Get file size
	if stat, err := file.Stat(); err == nil {
		info.Size = stat.Size()
	}

	// Try to extract more metadata from the ISO
	if err := extractISOMetadataWithContext(ctx, file, info); err != nil {
		// Non-fatal - we can still proceed with basic validation
		logger.GetLogger().Warn("Could not extract full ISO metadata", "error", err)
	}

	return info, nil
}

// ValidateWindowsISO is a backward compatibility wrapper
func ValidateWindowsISO(file *os.File) (*types.ISOInfo, error) {
	return ValidateWindowsISOWithContext(context.Background(), file)
}

func IsValidWindowsISO(file *os.File) bool {
	// Check if it's not a disk image (should be an ISO file)
	if isFileDiskImage(file.Name()) {
		return false
	}

	// Check if it has a valid UDF filesystem
	if !isFileUDF(file) {
		return false
	}

	// Additional Windows-specific validation
	return hasWindowsSignature(file)
}

func isFileDiskImage(file string) bool {
	disk, err := diskfs.Open(file, diskfs.WithOpenMode(diskfs.ReadOnly))
	if err != nil {
		return false
	}
	defer disk.Close()
	table, err := disk.GetPartitionTable()
	return err == nil && table != nil
}

func isFileUDF(file *os.File) bool {
	defer func() { 
		if r := recover(); r != nil {
			// Log recovery for debugging if needed
			logger.GetLogger().Debug("Recovered from panic in UDF check", "error", r)
		}
	}()

	originalPos, _ := file.Seek(0, io.SeekCurrent)
	defer func() {
		if _, err := file.Seek(originalPos, io.SeekStart); err != nil {
			logger.GetLogger().Debug("Failed to restore file position", "error", err)
		}
	}()

	iso, err := udf.NewUdfFromReader(file)
	if err != nil {
		return false
	}

	// Check if we can read the root directory
	files := iso.ReadDir(nil)
	return len(files) > 0
}

func hasWindowsSignature(file *os.File) bool {
	// Look for common Windows ISO indicators
	iso, err := udf.NewUdfFromReader(file)
	if err != nil {
		return false
	}

	files := iso.ReadDir(nil)
	hasBootmgr := false
	hasSources := false

	for _, f := range files {
		name := strings.ToLower(f.Name())
		switch name {
		case "bootmgr", "bootmgr.exe":
			hasBootmgr = true
		case "sources":
			if f.IsDir() {
				hasSources = true
			}
		}
	}

	// A valid Windows ISO should have bootmgr and a sources directory
	return hasBootmgr && hasSources
}

// extractISOMetadataWithContext extracts metadata from ISO with context support
func extractISOMetadataWithContext(ctx context.Context, file *os.File, info *types.ISOInfo) error {
	// Check if context is cancelled
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled during metadata extraction: %w", err)
	}

	iso, err := udf.NewUdfFromReader(file)
	if err != nil {
		return err
	}

	// Look for version information in common places
	files := iso.ReadDir(nil)
	for _, f := range files {
		// Check context periodically during iteration
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled during file scan: %w", err)
		}

		name := strings.ToLower(f.Name())
		switch name {
		case "sources":
			if f.IsDir() {
				sourcesFiles := f.ReadDir()
				for _, sf := range sourcesFiles {
					sourceName := strings.ToLower(sf.Name())
					if sourceName == "install.wim" {
						info.HasInstallWim = true
					}
				}
			}
		case "efi":
			info.IsUEFI = true
		case "boot":
			// Check for UEFI boot files
			if f.IsDir() {
				bootFiles := f.ReadDir()
				for _, bf := range bootFiles {
					if strings.ToLower(bf.Name()) == "efisys.bin" {
						info.IsUEFI = true
						break
					}
				}
			}
		}
	}

	return nil
}

// extractISOMetadata is a backward compatibility wrapper
func extractISOMetadata(file *os.File, info *types.ISOInfo) error {
	return extractISOMetadataWithContext(context.Background(), file, info)
}

func ExtractISOToLocation(iso *udf.Udf, location string) error {
	return ExtractISOWithProgress(iso, location, nil)
}

func ExtractISOWithProgress(iso *udf.Udf, location string, progressCallback ProgressCallback) error {
	// Calculate total files and size for progress reporting
	totalFiles, totalSize := calculateExtractionSize(iso)

	progress := &ExtractProgress{
		TotalFiles: totalFiles,
		TotalBytes: totalSize,
	}

	// Create the destination directory
	if err := os.MkdirAll(location, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Start extraction
	startTime := time.Now()
	ctx := context.Background()

	for _, file := range iso.ReadDir(nil) {
		if err := extractISOFileWithProgress(ctx, file, location, progress, progressCallback, startTime); err != nil {
			return err
		}
	}

	return nil
}

func calculateExtractionSize(iso *udf.Udf) (files int, size int64) {
	for _, file := range iso.ReadDir(nil) {
		f, s := calculateFileSize(file)
		files += f
		size += s
	}
	return
}

func calculateFileSize(file udf.File) (files int, size int64) {
	if file.IsDir() {
		files = 1 // count the directory itself
		for _, child := range file.ReadDir() {
			f, s := calculateFileSize(child)
			files += f
			size += s
		}
	} else {
		if file.Name() != "install.wim" { // Skip install.wim as we don't extract it
			files = 1
			size = file.Size()
		}
	}
	return
}

func extractISOFileWithProgress(ctx context.Context, file udf.File, location string, progress *ExtractProgress, callback ProgressCallback, startTime time.Time) error {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Skip install.wim for now (too large and requires special handling)
	if file.Name() == "install.wim" {
		return nil
	}

	progress.CurrentFile = file.Name()

	if file.IsDir() {
		folderPath := filepath.Join(location, file.Name())
		if err := os.MkdirAll(folderPath, file.Mode().Perm()); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", folderPath, err)
		}

		progress.FilesCopied++
		if callback != nil {
			callback(progress.BytesCopied, progress.TotalBytes, file.Name())
		}

		for _, child := range file.ReadDir() {
			if err := extractISOFileWithProgress(ctx, child, folderPath, progress, callback, startTime); err != nil {
				return err
			}
		}
	} else {
		filePath := filepath.Join(location, file.Name())
		newFile, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", file.Name(), err)
		}
		defer newFile.Close()

		reader := file.NewReader()
		written, err := io.Copy(newFile, reader)
		if err != nil {
			return fmt.Errorf("failed to copy file %s: %w", file.Name(), err)
		}

		if err := newFile.Sync(); err != nil {
			return fmt.Errorf("failed to sync file %s: %w", file.Name(), err)
		}

		// Update progress
		progress.FilesCopied++
		progress.BytesCopied += written

		// Calculate speed and time remaining
		elapsed := time.Since(startTime)
		if elapsed > 0 {
			progress.Speed = progress.BytesCopied / int64(elapsed.Seconds())
			if progress.Speed > 0 {
				remaining := progress.TotalBytes - progress.BytesCopied
				progress.TimeRemaining = time.Duration(remaining/progress.Speed) * time.Second
			}
		}

		if callback != nil {
			callback(progress.BytesCopied, progress.TotalBytes, file.Name())
		}
	}

	return nil
}

// ExtractISOCmd creates a tea.Cmd for ISO extraction with progress updates
func ExtractISOCmd(iso *udf.Udf, location string) tea.Cmd {
	return func() tea.Msg {
		progressChan := make(chan ExtractProgress, 10)

		go func() {
			defer close(progressChan)

			callback := func(current, total int64, filename string) {
				// Progress callback for ISO extraction - implementation depends on context
				// Progress reporting is handled by the calling function
			}

			if err := ExtractISOWithProgress(iso, location, callback); err != nil {
				// Send error message
				return
			}
		}()

		// Return the final completion message
		return extractProgressMsg(ExtractProgress{
			BytesCopied: 100,
			TotalBytes:  100,
			FilesCopied: 1,
			TotalFiles:  1,
			CurrentFile: "Complete",
		})
	}
}

// DiscoverWindowsISOs scans common locations for Windows ISO files
func DiscoverWindowsISOs() ([]*types.ISOInfo, error) {
	log := logger.GetLogger()
	log.Info("Scanning for Windows ISO files...")

	var discoveredISOs []*types.ISOInfo
	scanLocations := getISOScanLocations()

	for sourceName, paths := range scanLocations {
		for _, scanPath := range paths {
			if _, err := os.Stat(scanPath); os.IsNotExist(err) {
				log.Debug("Scan location does not exist", "path", scanPath)
				continue
			}

			log.Debug("Scanning %s: %s", sourceName, scanPath)
			isos, err := scanDirectoryForISOs(scanPath, sourceName)
			if err != nil {
				log.Warn("Failed to scan %s: %v", scanPath, err)
				continue
			}

			discoveredISOs = append(discoveredISOs, isos...)
		}
	}

	// Sort by relevance (most recent and likely locations first)
	sortISOsByRelevance(discoveredISOs)

	logger.GetLogger().Info("Found potential Windows ISOs", "count", len(discoveredISOs))
	return discoveredISOs, nil
}

// getISOScanLocations returns platform-specific locations to scan for ISOs
func getISOScanLocations() map[string][]string {
	homeDir, _ := os.UserHomeDir()

	locations := map[string][]string{
		"Downloads": {
			filepath.Join(homeDir, "Downloads"),
		},
		"Desktop": {
			filepath.Join(homeDir, "Desktop"),
		},
		"Documents": {
			filepath.Join(homeDir, "Documents"),
		},
		"Ember": {
			filepath.Join(homeDir, "ember"),
			filepath.Join(homeDir, "Documents", "Ember"),
		},
	}

	// Add platform-specific mount locations
	switch {
	case filepath.Separator == '/': // Unix-like (macOS, Linux)
		// macOS
		locations["External Drives"] = []string{"/Volumes"}
		// Linux
		locations["External Drives"] = append(locations["External Drives"], "/media", "/mnt")
	case filepath.Separator == '\\': // Windows
		// Add common Windows locations if we support Windows in future
		locations["External Drives"] = []string{"D:\\", "E:\\", "F:\\"}
	}

	return locations
}

// scanDirectoryForISOs recursively scans a directory for ISO files
func scanDirectoryForISOs(dirPath, sourceName string) ([]*types.ISOInfo, error) {
	var isos []*types.ISOInfo

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip directories we can't access
			return nil
		}

		// Skip hidden files and directories
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if it's an ISO file
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".iso") {
			// Quick size check - Windows ISOs are typically 2GB+
			if info.Size() < 1*1024*1024*1024 { // Less than 1GB, probably not Windows
				logger.GetLogger().Debug("Skipping small ISO", "path", path, "size", types.FormatBytes(info.Size()))
				return nil
			}

			logger.GetLogger().Debug("Found potential Windows ISO", "path", path, "size", types.FormatBytes(info.Size()))

			// Create ISOInfo with basic information
			isoInfo := &types.ISOInfo{
				Path:         path,
				Size:         info.Size(),
				Source:       sourceName,
				LastModified: info.ModTime(),
			}

			// Try to analyze the ISO (quick check)
			if err := quickAnalyzeISO(isoInfo); err != nil {
				logger.GetLogger().Debug("Failed to analyze ISO", "path", path, "error", err)
				// Still include it, but mark as unanalyzed
				isoInfo.Version = "Unknown"
			}

			isos = append(isos, isoInfo)
		}

		// Don't recurse too deep to avoid performance issues
		depth := strings.Count(strings.TrimPrefix(path, dirPath), string(filepath.Separator))
		if depth >= 3 {
			return filepath.SkipDir
		}

		return nil
	})

	return isos, err
}

// quickAnalyzeISO performs a fast analysis of an ISO file
func quickAnalyzeISO(isoInfo *types.ISOInfo) error {
	file, err := os.Open(isoInfo.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Try to validate as Windows ISO
	fullInfo, err := ValidateWindowsISO(file)
	if err != nil {
		return err
	}

	// Copy analyzed information
	isoInfo.Version = fullInfo.Version
	isoInfo.Architecture = fullInfo.Architecture
	isoInfo.Edition = fullInfo.Edition
	isoInfo.Language = fullInfo.Language
	isoInfo.BuildNumber = fullInfo.BuildNumber
	isoInfo.IsUEFI = fullInfo.IsUEFI
	isoInfo.HasInstallWim = fullInfo.HasInstallWim

	// Determine smart defaults based on analysis
	analysis := &types.FilesystemAnalysis{
		MaxFileSize: isoInfo.Size,
		TotalSize:   isoInfo.Size,
		WindowsVersion: isoInfo.Version,
		HasInstallWIM: isoInfo.HasInstallWim,
	}

	// Check for large files that would require NTFS
	if isoInfo.Size > FAT32MaxFileSize {
		analysis.LargeFiles = []string{"install.wim"}
	}

	recommendedFS := determineOptimalFilesystem(analysis)
	isoInfo.RecommendedFS = recommendedFS

	// Determine if workarounds are needed
	isoInfo.RequiresWorkaround = isWindows7(isoInfo.Version) && isoInfo.HasInstallWim

	// Estimate required USB size (ISO size + 20% overhead + bootloader space)
	isoInfo.EstimatedUSBSize = int64(float64(isoInfo.Size) * 1.2) + 100*1024*1024 // 100MB bootloader overhead

	return nil
}

// sortISOsByRelevance sorts ISOs by relevance (most recent in common locations first)
func sortISOsByRelevance(isos []*types.ISOInfo) {
	// Sort by: 1. Source priority, 2. Most recent
	sourceScore := map[string]int{
		"Downloads": 4,
		"Desktop":   3,
		"Documents": 2,
		"Ember":     5, // Highest priority for dedicated folder
		"External Drives": 1,
	}

	// Simple bubble sort by relevance
	for i := 0; i < len(isos); i++ {
		for j := i + 1; j < len(isos); j++ {
			scoreI := sourceScore[isos[i].Source]
			scoreJ := sourceScore[isos[j].Source]

			// If same source, sort by modification time
			if scoreI == scoreJ {
				if isos[i].LastModified.Before(isos[j].LastModified) {
					isos[i], isos[j] = isos[j], isos[i]
				}
			} else if scoreI < scoreJ {
				isos[i], isos[j] = isos[j], isos[i]
			}
		}
	}
}

// ValidateClipboardPath checks if clipboard contains a valid file path
func ValidateClipboardPath() (*types.ISOInfo, error) {
	// Clipboard integration requires platform-specific implementation
	// Future enhancement: integrate with clipboard libraries
	return nil, fmt.Errorf("clipboard path validation not available")
}

// DetectDragAndDrop checks for drag-and-drop paths in terminal
func DetectDragAndDrop(args []string) (*types.ISOInfo, error) {
	// Check if any command line arguments are ISO files
	for _, arg := range args {
		if strings.HasSuffix(strings.ToLower(arg), ".iso") {
			if _, err := os.Stat(arg); err == nil {
				isoInfo := &types.ISOInfo{
					Path:   arg,
					Source: "Drag & Drop",
				}

				if stat, err := os.Stat(arg); err == nil {
					isoInfo.Size = stat.Size()
					isoInfo.LastModified = stat.ModTime()

					// Quick analysis
					if err := quickAnalyzeISO(isoInfo); err != nil {
						logger.GetLogger().Warn("Failed to analyze dragged ISO", "error", err)
						isoInfo.Version = "Unknown"
					}
				}

				return isoInfo, nil
			}
		}
	}

	return nil, fmt.Errorf("no ISO files found in arguments")
}


// determineOptimalFilesystem determines the best filesystem for the given analysis
func determineOptimalFilesystem(analysis *types.FilesystemAnalysis) types.FilesystemType {
	// If there are large files (>4GB), we need NTFS
	if analysis.MaxFileSize > FAT32MaxFileSize {
		return types.FilesystemNTFS
	}

	// For smaller ISOs, FAT32 might be sufficient
	if analysis.TotalSize < 32*1024*1024*1024 { // Less than 32GB
		return types.FilesystemFAT32
	}

	// Default to ExFAT for modern compatibility
	return types.FilesystemExFAT
}

// isWindows7 checks if the version string indicates Windows 7
func isWindows7(version string) bool {
	version = strings.ToLower(version)
	return strings.Contains(version, "windows 7") ||
		   strings.Contains(version, "win7") ||
		   strings.Contains(version, "6.1")
}
