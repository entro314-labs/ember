//go:build !linux && !darwin

package filesystem

import (
	"errors"
)

func IsNTFSAvailable() bool {
	return false
}

func MakeNTFS(device string) error {
	return errors.ErrUnsupported
}
