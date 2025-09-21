# GoReleaser Integration

This project now uses [GoReleaser](https://goreleaser.com/) for automated releases, which provides several advantages over our previous custom build pipeline:

## 🎯 **Benefits of GoReleaser**

### **Simplified Release Process**
- **Single configuration file** (`.goreleaser.yml`) instead of complex workflow matrices
- **Standardized Go project release** workflow
- **Built-in best practices** for Go applications

### **Enhanced Features**
- **Package manager integration**: Automatic Homebrew, Scoop, and AUR packages
- **Professional archive formats**: Consistent naming and packaging
- **Advanced signing**: Built-in Cosign integration for supply chain security
- **SBOM generation**: Automatic software bill of materials
- **Better changelog**: Conventional commit parsing and categorization

### **Multi-Platform Support**
- **Cross-compilation**: Linux, macOS, Windows, FreeBSD
- **Multi-architecture**: AMD64, ARM64, 386
- **Container images**: Multi-arch Docker images with proper manifests
- **Archive formats**: Platform-appropriate formats (tar.gz, zip)

## 🚀 **Release Process**

### **Automated Releases**
```bash
# Create and push a tag to trigger release
git tag v1.2.3
git push origin v1.2.3
```

### **Manual Releases**
```bash
# Use GitHub CLI to trigger workflow dispatch
gh workflow run goreleaser.yml --ref main -f tag=v1.2.3

# Or trigger via GitHub web interface
# Go to Actions → GoReleaser → Run workflow
```

### **Dry Run Testing**
```bash
# Test release without publishing
gh workflow run goreleaser.yml --ref main -f tag=v1.2.3 -f dry_run=true

# Or locally with GoReleaser CLI
goreleaser release --snapshot --clean
```

## 📦 **Package Managers**

GoReleaser automatically maintains packages for multiple package managers:

### **Homebrew (macOS/Linux)**
```bash
# Add tap (one time setup)
brew tap entro314-labs/tap

# Install Ember
brew install ember
```

### **Scoop (Windows)**
```bash
# Add bucket (one time setup)
scoop bucket add entro314-labs https://github.com/entro314-labs/scoop-bucket

# Install Ember
scoop install ember
```

### **AUR (Arch Linux)**
```bash
# Using yay or paru
yay -S ember-bin

# Or manually
git clone https://aur.archlinux.org/ember-bin.git
cd ember-bin
makepkg -si
```

## 🔧 **Configuration**

The `.goreleaser.yml` file configures:

- **Builds**: Cross-platform compilation settings
- **Archives**: Archive formats and contents
- **Docker**: Multi-arch container images
- **Package managers**: Homebrew, Scoop, AUR integration
- **Signing**: Cosign signatures for security
- **Release notes**: Automated changelog generation

## 🛠️ **Development**

### **Local Testing**
```bash
# Install GoReleaser
go install github.com/goreleaser/goreleaser@latest

# Test configuration
goreleaser check

# Build snapshot (no publishing)
goreleaser release --snapshot --clean

# View built artifacts
ls dist/
```

### **Configuration Validation**
```bash
# Validate .goreleaser.yml
goreleaser check

# Debug configuration
goreleaser release --help
```

## 🔒 **Security Features**

- **Cosign signing**: All artifacts are signed with Cosign
- **SBOM generation**: Software bill of materials for supply chain security
- **Attestations**: Build provenance and security metadata
- **Checksum verification**: SHA256 checksums for all binaries

## 📋 **Migration from Custom Pipeline**

The previous custom release workflow has been replaced with GoReleaser, providing:

1. **Reduced complexity**: ~400 lines of workflow → ~50 lines + GoReleaser config
2. **Better maintainability**: Industry-standard tool vs custom scripts
3. **Enhanced features**: Package managers, signing, SBOM generation
4. **Professional presentation**: Better release notes and asset organization

## 🎯 **Future Enhancements**

GoReleaser enables easy addition of:

- **More package managers**: Chocolatey, APT/RPM repositories
- **Code signing**: Windows/macOS code signing certificates
- **Release announcements**: Twitter, Discord, Slack notifications
- **Custom publishing**: Upload to custom repositories or CDNs