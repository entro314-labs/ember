//go:build linux

package device

import (
	"fmt"
	"os"

	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/partition"
	"github.com/diskfs/go-diskfs/partition/gpt"
	"github.com/diskfs/go-diskfs/partition/mbr"
)

// FormatDiskWithUEFINTFS formats a disk with 2 partitions:
// - A 1 MiB FAT32 ESP partition holding UEFI:NTFS
// - The remaining disk is spanned by an mbr.NTFS / gpt.MicrosoftBasicData partition
func FormatDiskForUEFINTFS(name string, useGpt bool) error {
	log := GetLogger().WithContext("operation", "format_disk", "device", name, "partition_type", func() string {
		if useGpt {
			return "GPT"
		}
		return "MBR"
	}())

	log.Debug("Opening disk for formatting")
	disk, err := diskfs.Open(name, diskfs.WithOpenMode(diskfs.ReadWrite))
	if err != nil {
		log.Error("Failed to open destination", "error", err)
		return fmt.Errorf("failed to open destination: %w", err)
	}
	defer disk.Close()

	log.Debug("Calculating partition layout", "disk_size", disk.Size, "logical_block_size", disk.LogicalBlocksize)

	// Windows partition
	primaryPartitionStart := int64(1024*1024 /* 1 MiB */) / disk.LogicalBlocksize
	primaryPartitionSize := (disk.Size / disk.LogicalBlocksize) - (primaryPartitionStart * 2)
	primaryPartitionEnd := primaryPartitionStart + primaryPartitionSize - 1
	// UEFI:NTFS partition
	secondaryPartitionStart := primaryPartitionEnd + 1
	secondaryPartitionSize := int64(1024*1024 /* 1 MiB */) / disk.LogicalBlocksize
	secondaryPartitionEnd := secondaryPartitionStart + secondaryPartitionSize - 1

	var table partition.Table
	// TODO: GPT doesn't seem to work properly for some reason
	if useGpt {
		// Reserve 2048 sectors at the end just like fdisk
		secondaryPartitionStart -= 2048
		secondaryPartitionEnd -= 2048
		primaryPartitionSize -= 2048
		primaryPartitionEnd := primaryPartitionStart + primaryPartitionSize - 1
		table = &gpt.Table{
			Partitions: []*gpt.Partition{
				{Start: uint64(primaryPartitionStart), End: uint64(primaryPartitionEnd), Type: gpt.MicrosoftBasicData, Name: "Windows ISO"},
				{Start: uint64(secondaryPartitionStart), End: uint64(secondaryPartitionEnd), Type: gpt.EFISystemPartition, Name: "EFI System"},
			},
		}
	} else {
		table = &mbr.Table{
			Partitions: []*mbr.Partition{
				{Start: uint32(primaryPartitionStart), Size: uint32(primaryPartitionSize), Type: mbr.NTFS, Bootable: true},
				{Start: uint32(secondaryPartitionStart), Size: uint32(secondaryPartitionSize), Type: mbr.EFISystem, Bootable: false},
			},
		}
	}

	if err := disk.Partition(table); err != nil {
		return fmt.Errorf("failed to create partition table: %w", err)
	}
	return nil
}

func WriteUEFINTFSToPartition(partition string) error {
	file, err := os.OpenFile(partition, os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer file.Close()
	n, err := file.Write(UefiNtfsImg)
	if err != nil {
		return err
	} else if n != len(UefiNtfsImg) {
		return fmt.Errorf("short write: wrote %d of %d bytes", n, len(UefiNtfsImg))
	}
	return nil
}
