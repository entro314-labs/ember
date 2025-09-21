package filesystem

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Xmister/udf"
	"github.com/entro314-labs/ember/internal/iso"
	"github.com/entro314-labs/ember/internal/logger"
	"github.com/entro314-labs/ember/internal/security"
	"github.com/entro314-labs/ember/internal/types"
)

// DeviceManager defines the interface for device management operations
type DeviceManager interface {
	// ListDevices returns all available devices
	ListDevices(ctx context.Context) ([]Device, error)

	// ValidateDevice checks if a device is suitable for operations
	ValidateDevice(ctx context.Context, deviceID string) error

	// PrepareDevice prepares a device for operations
	PrepareDevice(ctx context.Context, device Device) error

	// FormatDevice formats a device with the specified filesystem
	FormatDevice(ctx context.Context, device Device, filesystem string) error
}

// ISOManager defines the interface for ISO operations
type ISOManager interface {
	// OpenISO opens and validates an ISO file
	OpenISO(ctx context.Context, file io.ReadSeeker) (ISO, error)

	// ValidateISO validates an ISO file
	ValidateISO(ctx context.Context, file io.ReadSeeker) (*types.ISOInfo, error)

	// ExtractISO extracts ISO contents to a location
	ExtractISO(ctx context.Context, iso ISO, destination string, progress ISOProgressCallback) error
}

// FlashingManager defines the interface for USB flashing operations
type FlashingManager interface {
	// CreateBootableUSB creates a bootable USB from an ISO
	CreateBootableUSB(ctx context.Context, iso ISO, device Device, options FlashingOptions) error

	// InstallBootloader installs bootloader on the device
	InstallBootloader(ctx context.Context, device Device, bootloaderType string) error
}

// Device represents a storage device
type Device interface {
	// GetID returns the device identifier
	GetID() string

	// GetSize returns the device size in bytes
	GetSize() int64

	// GetName returns the human-readable device name
	GetName() string

	// IsRemovable returns true if the device is removable
	IsRemovable() bool

	// GetType returns the device type (USB, HDD, SSD, etc.)
	GetType() string

	// GetMountPoint returns the mount point if mounted
	GetMountPoint() string
}

// ISO represents an ISO file
type ISO interface {
	// GetInfo returns ISO metadata
	GetInfo() *types.ISOInfo

	// GetUDF returns the UDF filesystem
	GetUDF() *udf.Udf

	// Close closes the ISO file
	Close() error
}

// FlashingOptions contains options for flashing operations
type FlashingOptions struct {
	UseGPT         bool
	PartitionLabel string
	SkipValidation bool
	Force          bool
	Verbose        bool
}

// ISOProgressCallback is called during long-running operations
type ISOProgressCallback func(current, total int64, message string)

// DeviceImpl implements the Device interface
type DeviceImpl struct {
	ID         string
	Size       int64
	Name       string
	Removable  bool
	Type       string
	MountPoint string
}

func (d *DeviceImpl) GetID() string         { return d.ID }
func (d *DeviceImpl) GetSize() int64        { return d.Size }
func (d *DeviceImpl) GetName() string       { return d.Name }
func (d *DeviceImpl) IsRemovable() bool     { return d.Removable }
func (d *DeviceImpl) GetType() string       { return d.Type }
func (d *DeviceImpl) GetMountPoint() string { return d.MountPoint }

// ISOImpl implements the ISO interface
type ISOImpl struct {
	info *types.ISOInfo
	udf  *udf.Udf
	file io.Closer
}

func (i *ISOImpl) GetInfo() *types.ISOInfo {
	return i.info
}

func (i *ISOImpl) GetUDF() *udf.Udf {
	return i.udf
}

func (i *ISOImpl) Close() error {
	if i.file != nil {
		return i.file.Close()
	}
	return nil
}

// PlatformDeviceManager implements DeviceManager for platform-specific operations
type PlatformDeviceManager struct {
	logger    *logger.Logger
	validator *security.SecurityValidator
}

// NewPlatformDeviceManager creates a new platform device manager
func NewPlatformDeviceManager() *PlatformDeviceManager {
	return &PlatformDeviceManager{
		logger:    logger.GetLogger().WithContext("component", "device_manager"),
		validator: security.GetSecurityValidator(),
	}
}

func (pdm *PlatformDeviceManager) ListDevices(ctx context.Context) ([]Device, error) {
	pdm.logger.Debug("Listing available devices")

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// This would call platform-specific device listing
	// For now, return empty list - implement based on existing code
	return []Device{}, nil
}

func (pdm *PlatformDeviceManager) ValidateDevice(ctx context.Context, deviceID string) error {
	pdm.logger.Debug("Validating device", "device_id", deviceID)

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return err
	}

	// Use security validator
	if err := pdm.validator.ValidateDeviceID(deviceID); err != nil {
		return err
	}

	// Use existing device validation system
	// return device.ValidateDevice(deviceID)
	return nil // Stub for now to avoid circular import
}

func (pdm *PlatformDeviceManager) PrepareDevice(ctx context.Context, dev Device) error {
	pdm.logger.Debug("Preparing device for operations", "device_id", dev.GetID())

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return err
	}

	// This would call platform-specific device preparation
	// TODO: Use existing prepareDevice function
	// legacyDevice := &types.DiskUtilDevice{
	//	DeviceIdentifier: dev.GetID(),
	// }
	// return device.PrepareDevice(legacyDevice)
	return nil // Stub for now to avoid circular import
}

func (pdm *PlatformDeviceManager) FormatDevice(ctx context.Context, device Device, filesystem string) error {
	pdm.logger.Debug("Formatting device", "device_id", device.GetID(), "filesystem", filesystem)

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return err
	}

	// This would call platform-specific formatting
	// Implementation depends on filesystem type and platform
	return nil
}

// DefaultISOManager implements ISOManager
type DefaultISOManager struct {
	logger    *logger.Logger
	validator *security.SecurityValidator
}

// NewDefaultISOManager creates a new ISO manager
func NewDefaultISOManager() *DefaultISOManager {
	return &DefaultISOManager{
		logger:    logger.GetLogger().WithContext("component", "iso_manager"),
		validator: security.GetSecurityValidator(),
	}
}

func (im *DefaultISOManager) OpenISO(ctx context.Context, file io.ReadSeeker) (ISO, error) {
	im.logger.Debug("Opening ISO file")

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Validate file if it's an *os.File
	if osFile, ok := file.(*os.File); ok {
		if err := im.validator.ValidateISOPath(osFile.Name()); err != nil {
			return nil, err
		}
	}

	// This would use the existing OpenWindowsISOWithContext function
	// For now, return a basic implementation
	return &ISOImpl{
		info: &types.ISOInfo{},
		file: nil, // Would store file reference
	}, nil
}

func (im *DefaultISOManager) ValidateISO(ctx context.Context, file io.ReadSeeker) (*types.ISOInfo, error) {
	im.logger.Debug("Validating ISO file")

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// This would use the existing ValidateWindowsISOWithContext function
	if osFile, ok := file.(*os.File); ok {
		return iso.ValidateWindowsISOWithContext(ctx, osFile)
	}

	return nil, fmt.Errorf("unsupported file type for validation")
}

func (im *DefaultISOManager) ExtractISO(ctx context.Context, iso ISO, destination string, progress ISOProgressCallback) error {
	im.logger.Debug("Extracting ISO contents", "destination", destination)

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return err
	}

	// This would implement ISO extraction with progress reporting
	return nil
}

// DefaultFlashingManager implements FlashingManager
type DefaultFlashingManager struct {
	logger        *logger.Logger
	deviceManager DeviceManager
	isoManager    ISOManager
}

// NewDefaultFlashingManager creates a new flashing manager
func NewDefaultFlashingManager(deviceManager DeviceManager, isoManager ISOManager) *DefaultFlashingManager {
	return &DefaultFlashingManager{
		logger:        logger.GetLogger().WithContext("component", "flashing_manager"),
		deviceManager: deviceManager,
		isoManager:    isoManager,
	}
}

func (fm *DefaultFlashingManager) CreateBootableUSB(ctx context.Context, iso ISO, device Device, options FlashingOptions) error {
	fm.logger.Info("Creating bootable USB",
		"device_id", device.GetID(),
		"use_gpt", options.UseGPT,
		"partition_label", options.PartitionLabel)

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return err
	}

	// Step 1: Validate device
	if !options.SkipValidation {
		if err := fm.deviceManager.ValidateDevice(ctx, device.GetID()); err != nil {
			return fmt.Errorf("device validation failed: %w", err)
		}
	}

	// Step 2: Prepare device
	if err := fm.deviceManager.PrepareDevice(ctx, device); err != nil {
		return fmt.Errorf("device preparation failed: %w", err)
	}

	// Step 3: Format device
	if err := fm.deviceManager.FormatDevice(ctx, device, "exfat"); err != nil {
		return fmt.Errorf("device formatting failed: %w", err)
	}

	// Step 4: Extract ISO contents
	// This would be implemented using the ISO manager

	// Step 5: Install bootloader
	if err := fm.InstallBootloader(ctx, device, "uefi-ntfs"); err != nil {
		return fmt.Errorf("bootloader installation failed: %w", err)
	}

	fm.logger.Info("Bootable USB creation completed successfully")
	return nil
}

func (fm *DefaultFlashingManager) InstallBootloader(ctx context.Context, dev Device, bootloaderType string) error {
	fm.logger.Debug("Installing bootloader", "device_id", dev.GetID(), "type", bootloaderType)

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return err
	}

	// TODO: Call the existing installBootloader function
	// legacyDevice := &types.DiskUtilDevice{
	//	DeviceIdentifier: dev.GetID(),
	// }
	// return device.InstallBootloader(legacyDevice)
	return nil // Stub for now to avoid circular import
}

// Application contains all managers
type Application struct {
	DeviceManager   DeviceManager
	ISOManager      ISOManager
	FlashingManager FlashingManager
	Logger          *logger.Logger
}

// NewApplication creates a new application with all managers
func NewApplication() *Application {
	deviceManager := NewPlatformDeviceManager()
	isoManager := NewDefaultISOManager()
	flashingManager := NewDefaultFlashingManager(deviceManager, isoManager)

	return &Application{
		DeviceManager:   deviceManager,
		ISOManager:      isoManager,
		FlashingManager: flashingManager,
		Logger:          logger.GetLogger().WithContext("component", "application"),
	}
}

// CreateBootableUSBWithContext creates a bootable USB with full context support
func (app *Application) CreateBootableUSBWithContext(ctx context.Context, isoPath, deviceID string, options FlashingOptions) error {
	// Set up timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	app.Logger.Info("Starting bootable USB creation",
		"iso_path", app.sanitizePath(isoPath),
		"device_id", deviceID,
		"options", options)

	// Open ISO
	file, err := os.Open(isoPath)
	if err != nil {
		return fmt.Errorf("failed to open ISO file: %w", err)
	}
	defer file.Close()

	iso, err := app.ISOManager.OpenISO(timeoutCtx, file)
	if err != nil {
		return fmt.Errorf("failed to open ISO: %w", err)
	}
	defer iso.Close()

	// Create device
	device := &DeviceImpl{
		ID: deviceID,
	}

	// Create bootable USB
	return app.FlashingManager.CreateBootableUSB(timeoutCtx, iso, device, options)
}

func (app *Application) sanitizePath(path string) string {
	return security.GetSecurityValidator().SanitizeForLogging(path)
}
