//go:build darwin

package filesystem

import (
	"fmt"
	"os/exec"
	"strings"
)

func IsExFATAvailable() bool {
	// Check if diskutil supports exFAT formatting
	cmd := exec.Command("diskutil", "listFilesystems")
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	// Look for ExFAT in the output
	return strings.Contains(string(out), "ExFAT")
}

func MakeExFAT(device string) error {
	// Use diskutil to format the device as exFAT
	// First, unmount the device
	if err := exec.Command("diskutil", "unmountDisk", device).Run(); err != nil {
		// Ignore unmount errors as the device might not be mounted
	}

	// Format as exFAT with volume name "WINDOWS"
	cmd := exec.Command("diskutil", "eraseVolume", "ExFAT", "WINDOWS", device)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create exFAT filesystem on %s: %w\noutput: %s", device, err, out)
	}

	return nil
}

