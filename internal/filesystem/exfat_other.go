//go:build !linux && !darwin

package filesystem

import "errors"

func IsExFATAvailable() bool {
	return false
}

func MakeExFAT(device string) error {
	return errors.ErrUnsupported
}
