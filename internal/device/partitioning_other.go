//go:build !linux && !darwin

package device

import "errors"

func FormatDiskForUEFINTFS(device string, useGpt bool) error {
	return errors.New("not implemented")
}

func GetBlockDevicePartition(blockDevice string, partNumber int) string {
	panic("GetBlockDevicePartition is not implemented on this OS")
}
