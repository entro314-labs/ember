# Ember 🫗

A modern, intelligent terminal application for creating bootable Windows USB drives with advanced features and beautiful TUI interface.

[![Build and Test](https://github.com/entro314-labs/Ember/actions/workflows/build.yml/badge.svg)](https://github.com/entro314-labs/Ember/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/entro314-labs/Ember)](https://goreportcard.com/report/github.com/entro314-labs/Ember)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## ✨ Features

🧠 **Intelligent ISO Analysis** - Automatic detection of Windows version, install.wim size, and optimal filesystem
🔄 **Dual-Partition UEFI:NTFS** - Smart handling of large files (>4GB) with automatic dual-partition creation
⚡ **GPT-First Approach** - Modern GPT partition tables by default, MBR for legacy compatibility
🎯 **Smart ISO Discovery** - Automatically finds Windows ISOs on your system with detailed analysis
🖥️ **Dual TUI Modes** - Basic mode for quick tasks, Advanced mode for power users
🔧 **Sophisticated Preferences** - Performance mode, quick format, large file handling options
📊 **Real-time Analysis** - Live filesystem analysis with 2025 Windows 11 compatibility checks
🛡️ **Enhanced Safety** - Advanced device validation and safety warnings
🚀 **Legacy Compatibility Warnings** - Intelligent detection and warnings for older systems

## 🚀 Quick Start

### Interactive Mode (Recommended)
```bash
# Launch the beautiful TUI interface
./ember

# Or with administrative privileges (recommended)
sudo ./ember
```

### Smart ISO Discovery
```bash
# Find all Windows ISOs on your system
./ember --discover

# Analyze a specific ISO
./ember --analyze-only --iso ~/Windows11.iso
```

### Headless Mode
```bash
# Windows 11 with intelligent defaults
./ember --iso ~/Windows11.iso --device disk2

# With performance optimization
./ember --iso ~/Windows11.iso --device disk2 --performance-mode

# Legacy BIOS compatibility
./ember --iso ~/Windows11.iso --device disk2 --mbr --disable-uefi
```

## 📋 System Requirements

### Minimum Requirements
- **go 1.26.5+** (for building from source)
- **Administrative privileges** (sudo on macOS/Linux)
- **USB drive** (8GB+ recommended, 16GB+ for Windows 11)

### Platform Support

#### macOS 🍎 (Primary Support)
- ✅ Full `diskutil` integration
- ✅ GPT + UEFI:NTFS dual-partition schemes
- ✅ Advanced device safety analysis
- ✅ Optimal partition size calculation
- ✅ Legacy MBR compatibility mode

#### Linux 🐧 (Experimental)
- ✅ Basic device detection (`lsblk`)
- ✅ Standard partitioning tools
- ⚠️ Some advanced features may require additional packages

#### Windows 🪟 (Planned)
- ⚠️ Framework implemented
- ❌ Not production ready

## 🎯 Key Features

### Intelligent Analysis Engine
- **Automatic ISO Detection** - Finds Windows ISOs across common locations
- **Install.wim Analysis** - Detects large files requiring NTFS/dual-partition
- **Windows Version Detection** - Identifies Windows 7/8/10/11 requirements
- **Filesystem Recommendations** - Smart FAT32/NTFS/ExFAT selection

### Windows 11 Optimizations
- **Large Install.wim Handling** - Automatic dual-partition for >4GB files
- **Modern UEFI Support** - GPT partition tables with optimal ESP sizing
- **Windows 11 24H2 Ready** - Latest compatibility and requirements
- **Legacy Fallback** - Maintains Windows 7-10 compatibility

### Advanced User Options
- **Performance Mode** - Optimized for speed over maximum compatibility
- **Quick Format** - Faster formatting (enabled by default)
- **Large File Support** - Automatic NTFS when needed
- **Custom Labels** - Personalized USB volume names

### Beautiful Interface
- **Dual TUI Modes** - Basic for simplicity, Advanced for power users
- **Real-time Progress** - File-by-file progress with speed estimates
- **Color-coded Safety** - Visual safety indicators for device selection
- **Interactive Menus** - Intuitive navigation and configuration

## 📖 Documentation

- **[Advanced Documentation](ADVANCED.md)** - Comprehensive technical guide
- **[CLI Reference](ADVANCED.md#cli-reference)** - Complete command-line options
- **[TUI Guide](ADVANCED.md#tui-interfaces)** - Terminal interface walkthrough
- **[2025 Features](ADVANCED.md#2025-enhancements)** - Modern Windows USB standards

## 🛠️ Installation

### Pre-built Binaries
Download the latest release from the [releases page](https://github.com/entro314-labs/Ember/releases).

### Build from Source
```bash
# Clone the repository
git clone https://github.com/entro314-labs/Ember.git
cd Ember

# Build for current platform
go build -o ember ./cmd/ember

# Or use make for advanced builds
make build
```

## 🔧 Common Usage Examples

### For Windows 11 Users
```bash
# Auto-detect optimal settings
./ember --discover
./ember --iso ~/Downloads/Win11_24H2.iso --device disk2

# Performance mode for fast USB creation
./ember --iso ~/Win11.iso --device disk2 --performance-mode
```

### For Legacy Systems
```bash
# Windows 7/8 compatibility
./ember --iso ~/Win7.iso --device disk2 --mbr --filesystem FAT32

# Force legacy compatibility
./ember --iso ~/Win10.iso --device disk2 --mbr --disable-uefi
```

### For Power Users
```bash
# Advanced mode with custom settings
./ember --iso ~/Win11.iso --device disk2 --advanced \
        --filesystem NTFS --label "Windows11Pro" \
        --performance-mode --quick-format
```

## ⚠️ Safety & Warnings

**CRITICAL**: This tool completely overwrites the target device. Always:

1. ✅ **Verify device identifier** - Double-check with `diskutil list` (macOS) or `lsblk` (Linux)
2. ✅ **Backup important data** - All data on target device will be lost
3. ✅ **Use sudo/admin privileges** - Required for device access
4. ✅ **Check safety indicators** - Green = safe, Yellow = caution, Red = danger
5. ✅ **Test with spare devices first** - Never risk important hardware

## 🐛 Troubleshooting

### Permission Issues
```bash
# macOS/Linux: Run with sudo
sudo ./ember

# Check if device is mounted
diskutil list  # macOS
lsblk         # Linux
```

### Large File Support
```bash
# For ISOs with install.wim >4GB
./ember --iso ~/Win11.iso --device disk2 --allow-large-files

# Force NTFS for large files
./ember --iso ~/Win11.iso --device disk2 --filesystem NTFS
```

### Legacy Compatibility
```bash
# For older systems that don't support UEFI
./ember --iso ~/Win10.iso --device disk2 --mbr --disable-uefi

# Check system compatibility
./ember --system-info
```

## 🤝 Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Setup
```bash
git clone https://github.com/entro314-labs/Ember.git
cd Ember
go mod tidy
go build ./cmd/ember
```

## 📄 License

Licensed under the MIT License - see [LICENSE](LICENSE) for details.

## 🙏 Acknowledgments

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Amazing TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Beautiful terminal styling
- [UEFI:NTFS](https://github.com/pbatard/uefi-ntfs) - UEFI NTFS boot support
- Microsoft - Windows USB creation standards and best practices

---

**Made with ❤️ for the community. Star ⭐ if this helps you create Windows USBs!**