# Ember Advanced Documentation 🔬

Comprehensive technical guide for power users, developers, and system administrators.

## Table of Contents

- [2025 Enhancements](#2025-enhancements)
- [CLI Reference](#cli-reference)
- [TUI Interfaces](#tui-interfaces)
- [Technical Architecture](#technical-architecture)
- [Filesystem Analysis](#filesystem-analysis)
- [Partition Schemes](#partition-schemes)
- [Platform-Specific Details](#platform-specific-details)
- [Developer Guide](#developer-guide)
- [Troubleshooting](#troubleshooting)

## 2025 Enhancements

### Modern Windows USB Standards

Ember implements the latest 2025 best practices for Windows USB creation:

#### GPT-First Approach
```bash
# GPT is now the  standard
./ember --iso Win11.iso --device disk2  # Uses GPT

# Explicit GPT usage
./ember --iso Win11.iso --device disk2 --gpt

# Legacy MBR only when needed
./ember --iso Win7.iso --device disk2 --mbr
```

#### Intelligent Dual-Partition Schemes
For Windows ISOs with large install.wim files (>4GB):

```
USB Layout (Dual-Partition UEFI:NTFS):
┌─────────────────────────────────────┐
│ Partition 1: WINDOWS (ExFAT/NTFS)  │ ← Main Windows files
│ Size: Remaining space - UEFI size  │
├─────────────────────────────────────┤
│ Partition 2: UEFI (FAT32)          │ ← Boot compatibility
│ Size: 512MB-1GB (Microsoft 2025)   │
└─────────────────────────────────────┘
```

#### Optimal UEFI System Partition Sizing
- **Small USBs (<32GB)**: 512MB UEFI partition
- **Large USBs (>64GB)**: 1GB UEFI partition
- **Very Large Install.wim**: Dynamic sizing based on content

### Windows 11 24H2 Compatibility

#### Automatic Detection
```bash
# Analyzes Windows version and requirements
./ember --analyze-only --iso ~/Windows11_24H2.iso
```

Output includes:
- Windows edition detection
- Install.wim size analysis
- UEFI/Legacy requirements
- Recommended partition scheme

#### Modern Filesystem Selection
```tree
Decision Matrix (2025):
├── install.wim > 4GB     → NTFS + Dual-partition
├── Windows 10/11         → NTFS (performance)
├── Total size > 32GB     → NTFS (optimization)
└── Legacy compatibility  → FAT32
```

## CLI Reference

### Complete Flag Reference

#### Essential Flags
```bash
--iso <path>           # Path to Windows ISO file
--device <id>          # Target device (disk2, sdb, etc.)
--advanced             # Enable advanced features
--force                # Skip confirmation prompts
--verbose              # Detailed logging output
--quiet                # Minimal output
```

#### Filesystem & Partitioning
```bash
--gpt                  # Use GPT partition table (default: true)
--mbr                  # Force MBR partition table (legacy)
--filesystem <type>    # Force filesystem (FAT32/NTFS/ExFAT)
--label <name>         # Custom volume label (default: "Windows USB")
```

#### Performance & Quality
```bash
--quick-format         # Fast format (default: true)
--performance-mode     # Optimize for speed over compatibility
--allow-large-files    # Enable NTFS for >4GB files (default: true)
```

#### Analysis & Discovery
```bash
--discover             # Find Windows ISOs on system
--analyze-only         # Analyze ISO without creating USB
--system-info          # Show system capabilities
```

#### Boot Configuration
```bash
--disable-uefi         # Disable UEFI boot support
--disable-legacy       # Disable legacy BIOS support
```

#### Development & Debug
```bash
--skip-analysis        # Skip automatic ISO analysis
--skip-deps            # Skip dependency checking
--skip-validation      # Skip ISO/device validation
--version              # Show version information
--help                 # Show help message
```

### Advanced CLI Examples

#### Windows 11 Scenarios
```bash
# Standard Windows 11 24H2 (auto-detects dual-partition)
./ember --iso ~/Win11_24H2.iso --device disk2

# Performance-optimized creation
./ember --iso ~/Win11.iso --device disk2 \
        --performance-mode --quick-format

# Large USB with optimal partitioning
./ember --iso ~/Win11.iso --device disk2 \
        --advanced --label "Win11Pro"
```

#### Legacy System Support
```bash
# Windows 7 BIOS compatibility
./ember --iso ~/Win7.iso --device disk2 \
        --mbr --disable-uefi --filesystem FAT32

# Windows 8.1 hybrid compatibility
./ember --iso ~/Win8.1.iso --device disk2 \
        --mbr --allow-large-files
```

#### Developer/Testing Scenarios
```bash
# Skip all validation (dangerous - testing only)
./ember --iso ~/test.iso --device disk2 \
        --skip-validation --skip-deps --force

# Maximum verbosity for debugging
./ember --iso ~/Win11.iso --device disk2 \
        --verbose --advanced
```

#### System Analysis
```bash
# Comprehensive system check
./ember --system-info

# ISO analysis without USB creation
./ember --analyze-only --iso ~/mysterious.iso

# Find all Windows ISOs
./ember --discover
```

## TUI Interfaces

### Basic TUI Mode

Perfect for quick, straightforward USB creation:

```
┌─────────────────────────────────────┐
│ Ember - Windows USB Creator                          │
├─────────────────────────────────────┤
│ 🔍 Smart ISO Discovery                                      │
│ 💿 Browse for ISO                                             
│ 🚀 Advanced Mode                    │
│ ❌ Exit                                                     │
└─────────────────────────────────────┘
```

**Workflow:**
1. Select ISO source (discovery or browse)
2. Choose target USB device with safety indicators
3. Automatic configuration with smart defaults
4. Real-time progress with file-by-file tracking

### Advanced TUI Mode

For power users requiring fine-grained control:

```
┌─────────────────────────────────────┐
│ 🫗 Ember Advanced Mode                                  │
├─────────────────────────────────────┤
│ 🔍 Select Windows ISO               │
│ 💿 Select Target Device             │
│ ⚙️  Advanced Options               │
│ 🔬 Analyze Source Media             │
│ 🚀 Create Bootable USB              │
│ 📊 System Information               │
│ ❌ Exit                             │
└─────────────────────────────────────┘
```

#### Advanced Options Menu
```
┌─────────────────────────────────────┐
│ ⚙️  Advanced Configuration          │
├─────────────────────────────────────┤
│ Filesystem: [Auto/FAT32/NTFS/ExFAT] │
│ Partition: [Auto/GPT/MBR]           │
│ Boot Mode: [Auto/UEFI/Legacy/Both]  │
│ Performance: [Compatibility/Speed]  │
│ Format: [Quick/Full]                │
│ Label: [Custom Volume Name]         │
│ Large Files: [Allow/Block]          │
└─────────────────────────────────────┘
```

### Safety Indicators

Color-coded device safety system:

- 🟢 **Green (Safe)**: Empty drive or previous Ember creation
- 🟡 **Yellow (Caution)**: Contains data but appears non-critical
- 🔴 **Red (Danger)**: Contains important data, backups, or system files

```
Device Selection:
┌─────────────────────────────────────┐
│ 🟢 USB Drive (disk2) - 32.0 GB     │ ← Safe to use
│ 🟡 Data Drive (disk3) - 1.0 TB     │ ← Contains data
│ 🔴 Backup Drive (disk4) - 2.0 TB   │ ← Dangerous!
└─────────────────────────────────────┘
```

## Technical Architecture

### Core Components

```tree
Ember Architecture:
┌─────────────────────────────────────┐
│ CLI Interface (cmd/ember/main.go)               │
├─────────────────────────────────────┤
│ TUI Interfaces (internal/ui/)                             │
│ ├─ Basic TUI (tui.go)                                        │
│ └─ Advanced TUI (advanced_tui.go)   │
├─────────────────────────────────────┤
│ Core Engine (internal/device/)      │
│ ├─ USB Creator                     │
│ ├─ Progress Tracker                │
│ └─ Dependency Manager              │
├─────────────────────────────────────┤
│ Analysis Engine (internal/)         │
│ ├─ Filesystem Analysis             │
│ ├─ ISO Discovery                   │
│ └─ Device Management               │
├─────────────────────────────────────┤
│ Platform Layer                      │
│ ├─ macOS (darwin)                  │
│ ├─ Linux                          │
│ └─ Windows (planned)                │
└─────────────────────────────────────┘
```

### Data Flow

```text
User Input → Analysis → Configuration → Creation
    │            │           │             │
    │            │           │             └─ Progress Tracking
    │            │           └─ User Preferences
    │            └─ ISO Analysis + Device Safety
    └─ CLI Flags / TUI Selections
```

### Key Algorithms

#### Filesystem Selection Logic
```text
determineOptimalFilesystem():
1. Check install.wim size
   ├─ >4GB → NTFS + Dual-partition
   └─ ≤4GB → Continue analysis
2. Check total ISO size
   ├─ >32GB → NTFS (performance)
   └─ ≤32GB → Continue analysis
3. Check Windows version
   ├─ Windows 10/11 → NTFS (modern)
   └─ Windows 7/8 → FAT32 (compatibility)
4. Apply user preferences
5. Return final recommendation
```

## Filesystem Analysis

### Comprehensive ISO Analysis

```bash
# Example analysis output
./ember --analyze-only --iso ~/Windows11.iso
```

```text
🔬 ISO Analysis Results:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📁 File: Windows11_24H2_x64.iso
📊 Size: 5.4 GB (5,840,281,600 bytes)
🔢 Files: 3,247 files analyzed
📦 install.wim: 4.8 GB (requires NTFS)
🏷️  Version: Windows 11 Pro (24H2)
🏗️  Architecture: x64 (64-bit)
🌐 Language: English (United States)
📅 Build: 26100.1742

🎯 Recommendations:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💾 Filesystem: NTFS (large install.wim)
🗂️  Partition: Dual-partition (UEFI:NTFS)
⚙️  Boot: UEFI + Legacy support
📏 USB Size: Minimum 8GB, Recommended 16GB

⚠️  Requirements:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
• Large file support (install.wim >4GB)
• Dual-partition UEFI:NTFS scheme
• Modern UEFI-capable target system
• TPM 2.0 + Secure Boot (Windows 11)
```

### Detection Capabilities

#### Windows Version Detection
- Registry analysis (`cversion.ini`)
- File signature analysis
- Build number extraction
- Edition identification (Pro, Home, Enterprise)

#### Install.wim Analysis
- File size detection (critical for >4GB)
- Compression analysis
- Index counting (multiple Windows editions)
- Integrity verification

#### File System Requirements
- Large file detection (>4GB threshold)
- Unicode filename support
- Case sensitivity requirements
- Performance characteristics

## Partition Schemes

### Modern Dual-Partition (2025 Standard)

For ISOs with large install.wim files:

```text
GPT Partition Table:
┌─────────────────────────────────────┐
│ EFI System Partition (ESP)          │
│ Type: EFI System (C12A7328-...)     │
│ Size: 512MB - 1GB                   │
│ Format: FAT32                       │
│ Label: UEFI                         │
│ Contents: UEFI:NTFS bootloader      │
├─────────────────────────────────────┤
│ Microsoft Basic Data                │
│ Type: Basic Data (EBD0A0A2-...)     │
│ Size: Remaining space               │
│ Format: NTFS/ExFAT                  │
│ Label: WINDOWS                      │
│ Contents: Windows installation files │
└─────────────────────────────────────┘
```

### Legacy Single-Partition

For older Windows versions or smaller ISOs:

```text
MBR Partition Table:
┌─────────────────────────────────────┐
│ Primary Partition                                            │
│ Type: 0x0C (FAT32 LBA)             │
│ Size: Full USB capacity            │
│ Format: FAT32                       │
│ Label: WINDOWS                      │
│ Boot: Active/Bootable               │
│ Contents: All Windows files         │
└─────────────────────────────────────┘
```

### Optimal Sizing Algorithm

```go
// Simplified sizing logic
func calculateOptimalPartitionSizes(usbSize, wimSize int64) (main, uefi string) {
    const (
        minUEFI = 300 * MB  // Microsoft minimum
        safeUEFI = 512 * MB // 2025 safe default
        optimalUEFI = 1 * GB // Large USB optimal
    )

    switch {
    case usbSize > 64*GB:
        return "R1GB", "1GB"      // Large USB
    case wimSize > 4*GB:
        return "R512MB", "512MB"  // Large install.wim
    default:
        return "R512MB", "512MB"  // 2025 standard
    }
}
```

## Platform-Specific Details

### macOS Implementation

#### Device Detection
```bash
# Uses diskutil for comprehensive device info
diskutil list -plist external
```

Features:
- Full device metadata (size, type, bus protocol)
- Safety analysis (removable, writable, internal check)
- Mount point detection and management
- Bus protocol detection (USB 2.0/3.0/3.1/C)

#### Partitioning
```bash
# GPT partition creation
diskutil partitionDisk disk2 2 GPT \
    ExFAT "WINDOWS" R512MB \
    "MS-DOS FAT32" "UEFI" 512MB
```

#### Formatting
```bash
# ExFAT formatting (NTFS fallback)
diskutil eraseVolume ExFAT "WINDOWS" disk2s1

# FAT32 for UEFI partition
diskutil eraseVolume "MS-DOS FAT32" "UEFI" disk2s2
```

### Linux Implementation

#### Device Detection
```bash
# Uses lsblk for device enumeration
lsblk -J -o NAME,SIZE,TYPE,MOUNTPOINT,RM
```

Features:
- Basic device information
- Removable media detection
- Mount point tracking
- Block device filtering

#### Partitioning
```bash
# GPT creation with parted
parted /dev/sdb mklabel gpt
parted /dev/sdb mkpart primary fat32 1MB 513MB
parted /dev/sdb mkpart primary ntfs 513MB 100%
```

#### Formatting
```bash
# NTFS formatting
mkfs.ntfs -f -L "WINDOWS" /dev/sdb1

# FAT32 formatting
mkfs.fat -F32 -n "UEFI" /dev/sdb2
```

## Developer Guide

### Building from Source

#### Prerequisites
```bash
# go 1.26 or later
go version

# Platform-specific tools
# macOS: Xcode Command Line Tools
xcode-select --install

# Linux: build-essential
sudo apt install build-essential
```

#### Build Commands
```bash
# Standard build
go build -o ember ./cmd/ember

# Release build with optimizations
go build -ldflags="-s -w" -o ember ./cmd/ember

# Cross-compilation
GOOS=linux GOARCH=amd64 go build -o ember-linux ./cmd/ember
GOOS=darwin GOARCH=arm64 go build -o ember-macos-arm64 ./cmd/ember
```

### Project Structure

```tree
ember/
├── cmd/ember/              # Main application entry point
│   └── main.go            # CLI interface and flag parsing
├── internal/              # Private application code
│   ├── device/           # Device management and USB creation
│   │   ├── advanced_usb_creator.go
│   │   ├── partitioning_darwin.go
│   │   ├── dependency_manager.go
│   │   └── progress_tracker.go
│   ├── filesystem/       # Filesystem analysis and utilities
│   │   └── filesystem.go
│   ├── iso/             # ISO discovery and analysis
│   │   └── iso.go
│   ├── logger/          # Structured logging
│   │   └── logger.go
│   ├── types/           # Shared data structures
│   │   └── common.go
│   └── ui/              # Terminal user interfaces
│       ├── tui.go       # Basic TUI implementation
│       └── advanced_tui.go  # Advanced TUI implementation
├── go.mod              # Go module definition
├── go.sum              # Go module checksums
├── README.md           # Main documentation
└── ADVANCED.md         # This file
```

### Adding Platform Support

#### 1. Create Platform-Specific Files
```tree
internal/device/
├── partitioning_darwin.go   # macOS implementation
├── partitioning_linux.go    # Linux implementation
└── partitioning_windows.go  # Windows implementation
```

#### 2. Use Build Tags
```go
//go:build windows

package device

func FormatDiskForUEFINTFS(device string, gpt bool) error {
    // Windows-specific implementation
}
```

#### 3. Implement Required Interfaces
```go
type PartitionManager interface {
    CreatePartition(device, size, filesystem string) error
    FormatPartition(partition, filesystem, label string) error
    ValidateDevice(device string) error
}
```

### Testing Framework

#### Unit Tests
```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Verbose test output
go test -v ./internal/filesystem/
```

#### Integration Tests
```bash
# Platform-specific tests (requires USB device)
go test -tags=integration ./internal/device/
```

#### Mock Testing
```go
type MockDevice struct {
    Size     int64
    IsRemovable bool
}

func (m *MockDevice) GetSize() int64 { return m.Size }
func (m *MockDevice) IsRemovable() bool { return m.IsRemovable }
```

## Troubleshooting

### Common Issues

#### 1. Permission Denied
```text
Error: failed to access device: permission denied

Solution:
sudo ./ember --iso file.iso --device disk2
```

#### 2. Device Busy/Mounted
```text
Error: device is currently mounted

Solution (macOS):
diskutil unmountDisk /dev/disk2
./ember --iso file.iso --device disk2

Solution (Linux):
umount /dev/sdb*
./ember --iso file.iso --device sdb
```

#### 3. Large File Support
```text
Error: install.wim (4.8GB) exceeds FAT32 limit

Solution:
./ember --iso file.iso --device disk2 --filesystem NTFS
# or
./ember --iso file.iso --device disk2 --allow-large-files
```

#### 4. Legacy BIOS Boot Issues
```text
Warning: created USB may not boot on legacy systems

Solution:
./ember --iso file.iso --device disk2 --mbr --disable-uefi
```

#### 5. Missing Dependencies
```text
Warning: sgdisk not found - limited GPT support

Solution (macOS):
brew install gptfdisk

Solution (Linux):
sudo apt install gdisk
```

### Debug Mode

#### Enable Verbose Logging
```bash
./ember --verbose --iso file.iso --device disk2
```

#### Skip Validation (Testing Only)
```bash
./ember --skip-validation --skip-deps --force \
        --iso file.iso --device disk2
```

#### System Information
```bash
./ember --system-info
```

### Log Analysis

#### Log Levels
- **DEBUG**: Detailed technical information
- **INFO**: General operational messages
- **WARN**: Non-fatal issues and warnings
- **ERROR**: Critical errors requiring attention

#### Key Log Messages
```text
# Successful partition creation
INFO: Partition creation completed successfully

# Filesystem analysis
INFO: Large install.wim detected (>4GB), dual-partition UEFI:NTFS scheme required

# Legacy compatibility warnings
WARN: Legacy system detected - Windows 11 requires UEFI boot
```

## Performance Optimization

### Speed vs Compatibility

#### Performance Mode
```bash
# Optimizes for speed over maximum compatibility
./ember --performance-mode --quick-format
```

Benefits:
- Larger cluster sizes for better throughput
- Quick format instead of full format
- Optimized for modern systems
- May reduce compatibility with very old hardware

#### Compatibility Mode (Default)
```bash
# Optimizes for maximum compatibility
./ember --iso file.iso --device disk2
```

Benefits:
- Smaller cluster sizes for better compatibility
- Full format for reliability
- Works with older systems
- Broader hardware support

### USB Speed Optimization

#### USB 3.0+ Detection
```tree
Device Analysis:
├─ USB 3.0+ → Performance mode recommended
├─ USB 2.0  → Compatibility mode recommended
└─ Unknown → Conservative defaults
```

#### Transfer Speed Estimates
```text
USB 2.0:    ~25 MB/s  → 5GB ISO = ~3.5 minutes
USB 3.0:    ~100 MB/s → 5GB ISO = ~50 seconds
USB 3.1:    ~200 MB/s → 5GB ISO = ~25 seconds
USB-C 3.2:  ~400 MB/s → 5GB ISO = ~12 seconds
```

---

## Support & Resources

- **GitHub Issues**: [Report bugs and feature requests](https://github.com/entro314-labs/Ember/issues)
- **Discussions**: [Community support and questions](https://github.com/entro314-labs/Ember/discussions)
- **Wiki**: [Additional documentation and guides](https://github.com/entro314-labs/Ember/wiki)

---

**⚡ Pro Tip**: Use `--discover` to find optimal ISOs and `--system-info` to verify compatibility before creating USBs!