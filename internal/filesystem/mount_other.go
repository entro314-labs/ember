//go:build !linux && !darwin

package filesystem

import (
	"errors"
)

func LoopMountFile(name string) (string, error) {
	return "", errors.New("not implemented")
}

func LoopUnmountFile(loopDevice string) error {
	return errors.New("not implemented")
}

func MountPartition(partition string, mountPoint string) error {
	return errors.New("not implemented")
}

func UnmountPartition(mountPoint string) error {
	return errors.New("not implemented")
}
