//go:build linux

package device

import "strconv"

func GetBlockDevicePartition(blockDevice string, partNumber int) string {
	blockDevicePartition := blockDevice
	if blockDevice[len(blockDevice)-1] >= '0' && blockDevice[len(blockDevice)-1] <= '9' {
		blockDevicePartition += "p"
	}
	return blockDevicePartition + strconv.Itoa(partNumber)
}
