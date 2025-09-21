//go:build darwin

package filesystem

import (
	"os/exec"
	"strings"
)

func MountPartition(partition string, mountPoint string) error {
	return exec.Command("diskutil", "mount", "-mountPoint", mountPoint, partition).Run()
}

func UnmountPartition(mountPoint string) error {
	return exec.Command("diskutil", "unmount", mountPoint).Run()
}

func LoopMountFile(name string) (string, error) {
	// macOS uses hdiutil to attach disk images
	out, err := exec.Command("hdiutil", "attach", "-nomount", name).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func LoopUnmountFile(loopDevice string) error {
	return exec.Command("hdiutil", "detach", loopDevice).Run()
}
