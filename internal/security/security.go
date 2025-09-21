package security

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/entro314-labs/ember/internal/logger"
)

// SecurityValidator provides input validation and security checks
type SecurityValidator struct {
	logger *logger.Logger
}

// Global security validator instance
var globalValidator *SecurityValidator

// NewSecurityValidator creates a new security validator
func NewSecurityValidator() *SecurityValidator {
	return &SecurityValidator{
		logger: logger.GetLogger().WithContext("component", "security_validator"),
	}
}

// GetSecurityValidator returns the global security validator instance
func GetSecurityValidator() *SecurityValidator {
	if globalValidator == nil {
		globalValidator = NewSecurityValidator()
	}
	return globalValidator
}

// ValidateDeviceID validates and sanitizes device identifiers
func (sv *SecurityValidator) ValidateDeviceID(deviceID string) error {
	if deviceID == "" {
		return fmt.Errorf("device ID cannot be empty")
	}

	// Trim whitespace and normalize
	deviceID = strings.TrimSpace(deviceID)

	// Check for null bytes and control characters
	for _, char := range deviceID {
		if char < 32 && char != 9 && char != 10 && char != 13 { // Allow tab, newline, carriage return
			return fmt.Errorf("device ID contains invalid control characters")
		}
	}

	// Platform-specific validation
	switch runtime.GOOS {
	case "darwin":
		return sv.validateMacOSDeviceID(deviceID)
	case "linux":
		return sv.validateLinuxDeviceID(deviceID)
	case "windows":
		return sv.validateWindowsDeviceID(deviceID)
	default:
		return fmt.Errorf("device validation not supported on %s", runtime.GOOS)
	}
}

// validateMacOSDeviceID validates macOS device identifiers
func (sv *SecurityValidator) validateMacOSDeviceID(deviceID string) error {
	// Remove /dev/ prefix if present
	clean := strings.TrimPrefix(deviceID, "/dev/")

	// Must match disk[0-9]+ pattern
	matched, err := regexp.MatchString(`^disk[0-9]+$`, clean)
	if err != nil {
		return fmt.Errorf("regex validation failed: %w", err)
	}

	if !matched {
		return fmt.Errorf("invalid macOS device ID format: %s (expected: disk[0-9]+)", clean)
	}

	// Additional security: prevent system disk targeting
	if clean == "disk0" || clean == "disk1" {
		return fmt.Errorf("targeting system disks (disk0, disk1) is not allowed for safety")
	}

	return nil
}

// validateLinuxDeviceID validates Linux device identifiers
func (sv *SecurityValidator) validateLinuxDeviceID(deviceID string) error {
	// Remove /dev/ prefix if present
	clean := strings.TrimPrefix(deviceID, "/dev/")

	// Must match sd[a-z]+ or nvme[0-9]+n[0-9]+ patterns
	sdPattern := regexp.MustCompile(`^sd[a-z]+$`)
	nvmePattern := regexp.MustCompile(`^nvme[0-9]+n[0-9]+$`)

	if !sdPattern.MatchString(clean) && !nvmePattern.MatchString(clean) {
		return fmt.Errorf("invalid Linux device ID format: %s (expected: sd[a-z]+ or nvme[0-9]+n[0-9]+)", clean)
	}

	// Additional security: prevent system disk targeting
	systemDisks := []string{"sda", "nvme0n1"}
	for _, sysDisk := range systemDisks {
		if clean == sysDisk {
			sv.logger.Warn("Targeting potential system disk", "device", clean)
			// Don't block but warn - user might have multiple disks
		}
	}

	return nil
}

// validateWindowsDeviceID validates Windows device identifiers
func (sv *SecurityValidator) validateWindowsDeviceID(deviceID string) error {
	// Basic validation for Windows drive letters or physical disk numbers
	matched, err := regexp.MatchString(`^([A-Za-z]:|\\\\\.\\PhysicalDrive[0-9]+)$`, deviceID)
	if err != nil {
		return fmt.Errorf("regex validation failed: %w", err)
	}

	if !matched {
		return fmt.Errorf("invalid Windows device ID format: %s", deviceID)
	}

	return nil
}

// ValidateISOPath validates and sanitizes ISO file paths
func (sv *SecurityValidator) ValidateISOPath(isoPath string) error {
	if isoPath == "" {
		return fmt.Errorf("ISO path cannot be empty")
	}

	// Clean and resolve the path
	cleanPath := filepath.Clean(isoPath)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal detected in ISO path: %s", isoPath)
	}

	// Verify file exists and is readable
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("ISO file does not exist: %s", absPath)
		}
		return fmt.Errorf("cannot access ISO file: %w", err)
	}

	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return fmt.Errorf("ISO path is not a regular file: %s", absPath)
	}

	// Check file size (reasonable limits)
	const maxISOSize = 50 * 1024 * 1024 * 1024 // 50 GB
	if info.Size() > maxISOSize {
		return fmt.Errorf("ISO file too large: %d bytes (max: %d bytes)", info.Size(), maxISOSize)
	}

	const minISOSize = 100 * 1024 * 1024 // 100 MB
	if info.Size() < minISOSize {
		sv.logger.Warn("ISO file smaller than expected", "size", info.Size(), "min_expected", minISOSize)
	}

	return nil
}

// ValidateCommandArguments validates command execution arguments
func (sv *SecurityValidator) ValidateCommandArguments(command string, args []string) error {
	if command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	// Blocked commands that are too dangerous
	blockedCommands := []string{
		"rm", "del", "format", "fdisk", "mkfs.ext4", "dd",
		"sudo", "su", "chmod", "chown",
	}

	baseCommand := filepath.Base(command)
	for _, blocked := range blockedCommands {
		if baseCommand == blocked {
			return fmt.Errorf("command '%s' is blocked for security reasons", blocked)
		}
	}

	// Check for command injection in arguments
	for i, arg := range args {
		if strings.Contains(arg, ";") || strings.Contains(arg, "&") ||
			strings.Contains(arg, "|") || strings.Contains(arg, "`") ||
			strings.Contains(arg, "$") {
			return fmt.Errorf("potential command injection in argument %d: %s", i, arg)
		}

		// Check for path traversal in arguments
		if strings.Contains(arg, "..") {
			return fmt.Errorf("potential path traversal in argument %d: %s", i, arg)
		}
	}

	return nil
}

// ValidateWithTimeout validates operations with a timeout
func (sv *SecurityValidator) ValidateWithTimeout(ctx context.Context, operation func() error, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- operation()
	}()

	select {
	case err := <-done:
		return err
	case <-timeoutCtx.Done():
		return fmt.Errorf("validation timed out after %v: %w", timeout, timeoutCtx.Err())
	}
}

// SanitizeForLogging sanitizes sensitive data for logging
func (sv *SecurityValidator) SanitizeForLogging(data string) string {
	// Remove or mask sensitive patterns
	// This is a basic implementation - extend as needed

	// Mask potential passwords or tokens
	passwordPattern := regexp.MustCompile(`(?i)(password|token|key|secret)[:=]\s*([^\s,}]+)`)
	data = passwordPattern.ReplaceAllString(data, "$1: [REDACTED]")

	// Mask file paths to protect privacy
	homePattern := regexp.MustCompile(`/home/[^/\s]+`)
	data = homePattern.ReplaceAllString(data, "/home/[USER]")

	usersPattern := regexp.MustCompile(`/Users/[^/\s]+`)
	data = usersPattern.ReplaceAllString(data, "/Users/[USER]")

	return data
}

// CheckPrivileges verifies required privileges for operations
func (sv *SecurityValidator) CheckPrivileges() error {
	if runtime.GOOS == "windows" {
		// On Windows, we need to check for admin privileges
		// This is a simplified check - real implementation would use Windows APIs
		return nil
	}

	// On Unix-like systems, check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("root privileges required for disk operations")
	}

	return nil
}

// Removed duplicate GetSecurityValidator function
