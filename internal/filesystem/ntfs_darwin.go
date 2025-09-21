//go:build darwin

package filesystem

import (
	"errors"
	"fmt"
	"os/exec"
)

func IsNTFSAvailable() bool {
	// macOS has limited NTFS write support
	// Check if ntfs-3g is installed via Homebrew or MacPorts
	paths := []string{
		"/usr/local/bin/mkfs.ntfs",
		"/opt/local/bin/mkfs.ntfs",
		"/usr/local/sbin/mkfs.ntfs",
		"/opt/homebrew/bin/mkfs.ntfs",
		"/opt/homebrew/sbin/mkfs.ntfs",
	}

	for _, path := range paths {
		if _, err := exec.LookPath(path); err == nil {
			return true
		}
	}

	// Also check if the command is available in PATH
	_, err := exec.LookPath("mkfs.ntfs")
	return err == nil
}

func MakeNTFS(device string) error {
	if !IsNTFSAvailable() {
		return errors.New("NTFS formatting not available on macOS. Please install ntfs-3g via Homebrew: 'brew install ntfs-3g'")
	}

	// Try to use mkfs.ntfs if available
	cmd := exec.Command("mkfs.ntfs", "-Q", "-v", device)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create NTFS filesystem on %s: %w\noutput: %s", device, err, out)
	}

	return nil
}

// Alternative method using diskutil (creates NTFS but read-only on macOS)
func MakeNTFSReadOnly(device string) error {
	// This creates an NTFS volume but it will be read-only on macOS
	cmd := exec.Command("diskutil", "eraseVolume", "NTFS", "WINDOWS", device)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create NTFS filesystem on %s: %w\noutput: %s", device, err, out)
	}

	return nil
}
