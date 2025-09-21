package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/entro314-labs/ember/internal/device"
	"github.com/entro314-labs/ember/internal/filesystem"
	"github.com/entro314-labs/ember/internal/iso"
	"github.com/entro314-labs/ember/internal/types"
)


// AdvancedTUIModel represents the advanced TUI model
type AdvancedTUIModel struct {
	// Basic TUI fields
	state        appState
	isoPath      string
	deviceID     string
	err          error
	quitting     bool

	// Advanced features
	advancedMode     bool
	analysis         *types.FilesystemAnalysis
	creator          *device.AdvancedUSBCreator
	progressTracker  *device.ProgressTracker

	// TUI components
	list            list.Model
	progressBar     progress.Model
	spinner         spinner.Model

	// Configuration options
	forceFilesystem types.FilesystemType
	customLabel     string
	enableUEFI      bool
	enableLegacy    bool
	skipAnalysis    bool
	quickFormat     bool
	performanceMode bool
	allowLargeFiles bool

	// Progress tracking
	currentOperation string
	overallProgress  float64
	currentFile      string
	operationStatus  map[string]types.OperationStatus
	completionSummary map[string]interface{}
}

// Advanced menu items
type advancedMenuItem struct {
	title       string
	description string
	action      string
}

func (i advancedMenuItem) Title() string       { return i.title }
func (i advancedMenuItem) Description() string { return i.description }
func (i advancedMenuItem) FilterValue() string { return i.title }

// NewAdvancedTUI creates a new advanced TUI
func NewAdvancedTUI() {
	// Initialize advanced TUI model
	model := AdvancedTUIModel{
		state:           stateMenu,
		advancedMode:    true,
		progressBar:     progress.New(progress.WithDefaultGradient()),
		spinner:         spinner.New(),
		enableUEFI:      true,
		enableLegacy:    true,
		customLabel:     "Windows USB",
		quickFormat:     true,  // Default to quick format
		performanceMode: false, // Default to compatibility mode
		allowLargeFiles: true,  // Default to allowing large files
		operationStatus: make(map[string]types.OperationStatus),
	}

	// Set up spinner
	model.spinner.Spinner = spinner.Dot
	model.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Set up main menu
	model.setupAdvancedMenu()

	// Start the TUI
	p := tea.NewProgram(&model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
	}
}

// setupAdvancedMenu sets up the advanced menu options
func (m *AdvancedTUIModel) setupAdvancedMenu() {
	items := []list.Item{
		advancedMenuItem{
			title:       "🔍 Select Windows ISO",
			description: "Choose a Windows installation ISO file",
			action:      "select_iso",
		},
		advancedMenuItem{
			title:       "💿 Select Target Device",
			description: "Choose the USB device to create bootable media",
			action:      "select_device",
		},
		advancedMenuItem{
			title:       "⚙️  Advanced Options",
			description: "Configure filesystem, UEFI/Legacy boot, and other settings",
			action:      "advanced_options",
		},
		advancedMenuItem{
			title:       "🔬 Analyze Source Media",
			description: "Perform comprehensive analysis of the selected ISO",
			action:      "analyze_media",
		},
		advancedMenuItem{
			title:       "🚀 Create Bootable USB",
			description: "Start the advanced USB creation process",
			action:      "create_usb",
		},
		advancedMenuItem{
			title:       "📊 System Information",
			description: "View system capabilities and dependencies",
			action:      "system_info",
		},
		advancedMenuItem{
			title:       "❌ Exit",
			description: "Exit the application",
			action:      "exit",
		},
	}

	const defaultWidth = 20
	l := list.New(items, list.NewDefaultDelegate(), defaultWidth, 14)
	l.Title = "🫗 Ember Advanced Mode"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	m.list = l
}

// AdvancedTUI Update function
func (m *AdvancedTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			m.quitting = true
			if m.creator != nil {
				m.creator.Cleanup()
			}
			return m, tea.Quit

		case "enter":
			return m.handleMenuSelection()

		case "a":
			// Toggle advanced mode
			if m.state == stateMenu {
				m.advancedMode = !m.advancedMode
				if m.advancedMode {
					m.setupAdvancedMenu()
				} else {
					// Switch back to basic menu (would need to implement)
				}
			}
		}

	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case ProgressMsg:
		// Handle progress updates
		return m.handleProgressUpdate(msg)

	case ErrorMsg:
		m.state = stateError
		m.err = msg.err
		return m, nil

	case CompletionMsg:
		m.state = stateComplete
		return m, nil
	}

	// Update list
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// Handle menu selection in advanced mode
func (m *AdvancedTUIModel) handleMenuSelection() (tea.Model, tea.Cmd) {
	if item, ok := m.list.SelectedItem().(advancedMenuItem); ok {
		switch item.action {
		case "select_iso":
			return m, m.selectISOFile()
		case "select_device":
			return m, m.selectDevice()
		case "advanced_options":
			return m, m.showAdvancedOptions()
		case "analyze_media":
			return m, m.analyzeSourceMedia()
		case "create_usb":
			return m, m.createBootableUSB()
		case "system_info":
			return m, m.showSystemInfo()
		case "exit":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// Custom messages for advanced TUI
type ProgressMsg struct {
	operationID string
	progress    float64
	message     string
	status      types.OperationStatus
}

type ErrorMsg struct {
	err error
}

type CompletionMsg struct {
	summary map[string]interface{}
}

// Command functions for advanced TUI
func (m *AdvancedTUIModel) selectISOFile() tea.Cmd {
	return func() tea.Msg {
		// Use the smart ISO discovery system for file selection
		isos, err := iso.DiscoverWindowsISOs()
		if err != nil {
			return ErrorMsg{err: err}
		}

		if len(isos) > 0 {
			// Use the first found ISO as default
			m.isoPath = isos[0].Path
		}

		return tea.Msg(nil)
	}
}

func (m *AdvancedTUIModel) selectDevice() tea.Cmd {
	return func() tea.Msg {
		// Get available devices using the device management system
		devices, err := device.ListDevices()
		if err != nil {
			return ErrorMsg{err: err}
		}

		if len(devices) > 0 {
			// Use the first USB device as default
			for _, dev := range devices {
				if dev.RemovableMedia && !dev.Internal {
					m.deviceID = dev.DeviceIdentifier
					break
				}
			}
		}

		return tea.Msg(nil)
	}
}

func (m *AdvancedTUIModel) showAdvancedOptions() tea.Cmd {
	return func() tea.Msg {
		// Toggle advanced configuration options
		m.advancedMode = !m.advancedMode

		// Set sensible defaults for advanced options
		if m.advancedMode {
			m.enableUEFI = true
			m.enableLegacy = true
			m.customLabel = "Windows USB"
		}

		return tea.Msg(nil)
	}
}

func (m *AdvancedTUIModel) analyzeSourceMedia() tea.Cmd {
	return func() tea.Msg {
		if m.isoPath == "" {
			return ErrorMsg{err: fmt.Errorf("no ISO file selected")}
		}

		// Perform source media analysis
		analysis, err := filesystem.AnalyzeSourceMedia(m.isoPath)
		if err != nil {
			return ErrorMsg{err: err}
		}

		m.analysis = analysis
		return tea.Msg(nil) // Return to menu with analysis complete
	}
}

func (m *AdvancedTUIModel) createBootableUSB() tea.Cmd {
	return func() tea.Msg {
		if m.isoPath == "" || m.deviceID == "" {
			return ErrorMsg{err: fmt.Errorf("ISO file and device must be selected")}
		}

		// Initialize creator and progress tracking
		m.creator = device.NewAdvancedUSBCreator()

		// Create a new progress tracker for this operation
		m.progressTracker = device.NewProgressTracker()

		// Subscribe to progress updates
		tuiSubscriber := &TUIProgressSubscriber{model: m}
		m.progressTracker.Subscribe(tuiSubscriber)
		m.progressTracker.StartPeriodicUpdates(500 * time.Millisecond)

		// Configure creation options
		config := &device.USBCreationConfig{
			SourcePath:      m.isoPath,
			TargetDevice:    "/dev/" + m.deviceID,
			Label:           m.customLabel,
			ForceFilesystem: m.forceFilesystem,
			SkipAnalysis:    m.skipAnalysis,
			EnableUEFI:      m.enableUEFI,
			LegacyBIOS:      m.enableLegacy,
			QuickFormat:     m.quickFormat,
			PerformanceMode: m.performanceMode,
			AllowLargeFiles: m.allowLargeFiles,
			Verbose:         false, // TUI handles its own output
		}

		// Start creation process in goroutine with proper message channel
		go func() {
			// Create channels for progress tracking
			done := make(chan struct{})
			errorChan := make(chan error, 1)

			// Run USB creation in separate goroutine
			go func() {
				defer close(done)
				err := m.creator.CreateAdvancedWindowsUSB(config)
				if err != nil {
					errorChan <- err
				}
			}()

			// Monitor for completion or error
			select {
			case err := <-errorChan:
				// Send error message directly to the TUI
				m.err = err
				m.state = stateError
			case <-done:
				// Success - get creation summary
				summary := m.creator.GetCreationSummary()
				m.completionSummary = summary
				m.state = stateComplete
			}
		}()

		m.state = stateProgress
		return m.spinner.Tick
	}
}

func (m *AdvancedTUIModel) showSystemInfo() tea.Cmd {
	return func() tea.Msg {
		// Display comprehensive system information using dependency manager
		dm := device.NewDependencyManager()
		err := dm.CheckAllDependencies()
		if err != nil {
			return ErrorMsg{err: err}
		}

		// System info checked and displayed via logger
		return tea.Msg(nil)
	}
}

// Handle progress updates
func (m *AdvancedTUIModel) handleProgressUpdate(msg ProgressMsg) (tea.Model, tea.Cmd) {
	m.currentOperation = msg.operationID
	m.overallProgress = msg.progress
	m.operationStatus[msg.operationID] = msg.status

	// Update progress bar
	if msg.progress > 0 {
		m.progressBar.SetPercent(msg.progress / 100.0)
	}

	return m, nil
}

// AdvancedTUI View function
func (m *AdvancedTUIModel) View() string {
	if m.quitting {
		return "Goodbye! 👋\n"
	}

	switch m.state {
	case stateMenu:
		return m.renderAdvancedMenu()
	case stateProgress:
		return m.renderAdvancedProgress()
	case stateComplete:
		return m.renderCompletion()
	case stateError:
		return m.renderError()
	default:
		return m.renderAdvancedMenu()
	}
}

func (m *AdvancedTUIModel) renderAdvancedMenu() string {
	var status strings.Builder

	// Show current selection status
	if m.isoPath != "" {
		status.WriteString(fmt.Sprintf("📁 ISO: %s\n", m.isoPath))
	}
	if m.deviceID != "" {
		status.WriteString(fmt.Sprintf("💿 Device: %s\n", m.deviceID))
	}
	if m.analysis != nil {
		status.WriteString(fmt.Sprintf("🔍 Analysis: %s filesystem recommended\n", m.analysis.RecommendedFS))
	}

	if status.Len() > 0 {
		status.WriteString("\n")
	}

	// Show advanced settings
	settings := fmt.Sprintf(
		"⚙️  Settings: Label='%s', UEFI=%v, Legacy=%v\n\n",
		m.customLabel, m.enableUEFI, m.enableLegacy,
	)

	help := "\nControls: ↑/↓ navigate, Enter select, 'a' toggle advanced mode, 'q' quit"

	return headerStyle.Render("🫗 Ember Advanced USB Creator") + "\n\n" +
		status.String() +
		settings +
		m.list.View() +
		help
}

func (m *AdvancedTUIModel) renderAdvancedProgress() string {
	var progress strings.Builder

	progress.WriteString(headerStyle.Render("🚀 Creating Bootable USB"))
	progress.WriteString("\n\n")

	// Show overall progress
	progress.WriteString(fmt.Sprintf("Overall Progress: %.1f%%\n", m.overallProgress))
	progress.WriteString(m.progressBar.View())
	progress.WriteString("\n\n")

	// Show current operation
	if m.currentOperation != "" {
		progress.WriteString(fmt.Sprintf("%s Current: %s\n", m.spinner.View(), m.currentOperation))
	}

	// Show current file if available
	if m.currentFile != "" {
		progress.WriteString(fmt.Sprintf("📁 File: %s\n", m.currentFile))
	}

	// Show operation statuses
	progress.WriteString("\nOperations:\n")
	for opID, status := range m.operationStatus {
		statusIcon := "⏳"
		switch status {
		case types.StatusCompleted:
			statusIcon = "✅"
		case types.StatusFailed:
			statusIcon = "❌"
		case types.StatusRunning:
			statusIcon = "🔄"
		case types.StatusCanceled:
			statusIcon = "⚠️"
		}
		progress.WriteString(fmt.Sprintf("  %s %s\n", statusIcon, opID))
	}

	progress.WriteString("\n\nPress 'q' to cancel")

	return progress.String()
}

func (m *AdvancedTUIModel) renderCompletion() string {
	success := successStyle.Render("✅ USB Creation Completed Successfully!")

	var details strings.Builder
	details.WriteString("\n\n📊 Summary:\n")
	details.WriteString(fmt.Sprintf("  • Source: %s\n", m.isoPath))
	details.WriteString(fmt.Sprintf("  • Target: %s\n", m.deviceID))
	details.WriteString(fmt.Sprintf("  • Label: %s\n", m.customLabel))

	if m.analysis != nil {
		details.WriteString(fmt.Sprintf("  • Filesystem: %s\n", m.analysis.RecommendedFS))
		details.WriteString(fmt.Sprintf("  • Files copied: %d\n", m.analysis.FileCount))
		details.WriteString(fmt.Sprintf("  • Total size: %s\n", types.FormatBytes(m.analysis.TotalSize)))
	}

	instructions := "\n\n🎉 Your Windows USB is ready for use!\n" +
		"You can safely remove the device and use it to install Windows.\n\n" +
		"Press any key to return to menu or 'q' to quit."

	return success + details.String() + instructions
}

func (m *AdvancedTUIModel) renderError() string {
	errorText := errorStyle.Render(fmt.Sprintf("❌ Error: %v", m.err))
	help := "\n\nPress any key to return to menu or 'q' to quit."

	return errorText + help
}

// TUIProgressSubscriber implements progress updates for the TUI
type TUIProgressSubscriber struct {
	model *AdvancedTUIModel
}

func (tps *TUIProgressSubscriber) OnProgressUpdate(update device.ProgressUpdate) {
	// Update the model's progress tracking fields
	if tps.model != nil {
		tps.model.currentOperation = update.OperationName
		tps.model.overallProgress = update.Progress
		tps.model.currentFile = update.CurrentFile
		tps.model.operationStatus[update.OperationID] = types.OperationStatus(update.Status)

		// Update progress bar
		if update.Progress > 0 {
			tps.model.progressBar.SetPercent(update.Progress / 100.0)
		}
	}
}

// Init function for the TUI model
func (m *AdvancedTUIModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
	)
}