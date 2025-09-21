package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/entro314-labs/ember/internal/device"
	"github.com/entro314-labs/ember/internal/iso"
	"github.com/entro314-labs/ember/internal/types"
)

// App states
type appState int

const (
	stateMenu appState = iota
	stateISODiscovery
	stateISOSelection
	stateDeviceSelection
	stateProgress
	stateComplete
	stateError
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color("#874BFD")).
			MarginBottom(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5F87")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			MarginTop(1)
)

// Model represents the state of the TUI
type model struct {
	state          appState
	list           list.Model
	filepicker     filepicker.Model
	progress       progress.Model
	spinner        spinner.Model
	selectedISO    string
	selectedDevice *types.DiskUtilDevice
	devices        []types.DiskUtilDevice
	discoveredISOs []*types.ISOInfo
	error          error
	isLoading      bool
	width          int
	height         int
}

// Messages
type errMsg error
type loadDevicesMsg struct {
	devices []types.DiskUtilDevice
	err     error
}

type discoverISOsMsg struct {
	isos []*types.ISOInfo
	err  error
}

// Flash progress messages
type flashProgressMsg device.FlashProgress
type flashCompleteMsg struct{}
type flashErrorMsg struct {
	err error
}

func initialModel() model {
	// Initialize list
	items := []list.Item{
		menuItem{title: "🔍 Smart ISO Discovery", desc: "Automatically find Windows ISO files on your system"},
		menuItem{title: "💿 Browse for ISO", desc: "Manually select a Windows ISO file"},
		menuItem{title: "🚀 Advanced Mode", desc: "Enable advanced features and analysis"},
		menuItem{title: "❌ Exit", desc: "Quit the application"},
	}

	l := list.New(items, newMenuDelegate(), 0, 0)
	l.Title = "Ember - Windows USB Creator"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	// Initialize filepicker
	fp := filepicker.New()
	fp.AllowedTypes = []string{".iso"}
	fp.CurrentDirectory, _ = os.Getwd()

	// Initialize progress bar
	prog := progress.New(progress.WithDefaultGradient())

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		state:      stateMenu,
		list:       l,
		filepicker: fp,
		progress:   prog,
		spinner:    s,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.filepicker.Init(),
		m.spinner.Tick,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-8)
		m.progress.Width = msg.Width - 4

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.state == stateMenu || m.state == stateComplete || m.state == stateError {
				return m, tea.Quit
			}
		case "esc":
			switch m.state {
			case stateISODiscovery:
				m.state = stateMenu
			case stateISOSelection:
				m.state = stateMenu
			case stateDeviceSelection:
				if len(m.discoveredISOs) > 0 {
					m.state = stateISODiscovery
				} else {
					m.state = stateISOSelection
				}
			case stateError:
				m.state = stateMenu
			}
		case "enter":
			if m.state == stateMenu {
				return m.handleMenuSelection()
			} else if m.state == stateISODiscovery {
				return m.handleISOSelection()
			} else if m.state == stateDeviceSelection {
				return m.handleDeviceSelection()
			} else if m.state == stateComplete {
				return m, tea.Quit
			}
		}

	case errMsg:
		m.error = msg
		m.state = stateError
		m.isLoading = false

	case flashProgressMsg:
		if m.state == stateProgress {
			progress := device.FlashProgress(msg)
			m.progress.SetPercent(progress.Progress)
		}

	case flashCompleteMsg:
		m.state = stateComplete
		m.isLoading = false

	case flashErrorMsg:
		m.error = msg.err
		m.state = stateError
		m.isLoading = false

	case loadDevicesMsg:
		m.isLoading = false
		if msg.err != nil {
			m.error = msg.err
			m.state = stateError
		} else {
			m.devices = msg.devices
			m.updateDeviceList()
		}

	case discoverISOsMsg:
		m.isLoading = false
		if msg.err != nil {
			m.error = msg.err
			m.state = stateError
		} else {
			m.discoveredISOs = msg.isos
			m.updateISOList()
		}

	case spinner.TickMsg:
		if m.isLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Update sub-models based on state
	switch m.state {
	case stateMenu, stateISODiscovery, stateDeviceSelection:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

	case stateISOSelection:
		var cmd tea.Cmd
		m.filepicker, cmd = m.filepicker.Update(msg)
		cmds = append(cmds, cmd)

		// Check if file was selected
		if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
			m.selectedISO = path
			m.state = stateDeviceSelection
			m.isLoading = true
			cmds = append(cmds, loadDevicesCmd())
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	var content string

	switch m.state {
	case stateMenu:
		content = m.list.View()

	case stateISODiscovery:
		if m.isLoading {
			content = headerStyle.Render("Discovering Windows ISOs...") + "\n\n" +
				m.spinner.View() + " Scanning common locations for Windows ISO files..."
		} else {
			content = m.list.View()
		}

	case stateISOSelection:
		content = headerStyle.Render("Select Windows ISO File") + "\n\n" +
			m.filepicker.View() + "\n\n" +
			helpStyle.Render("Press ESC to go back • Press Enter to select")

	case stateDeviceSelection:
		if m.isLoading {
			content = headerStyle.Render("Loading Devices...") + "\n\n" +
				m.spinner.View() + " Scanning for external devices..."
		} else {
			content = m.list.View()
		}

	case stateProgress:
		content = headerStyle.Render("Creating Bootable USB...") + "\n\n" +
			"ISO: " + filepath.Base(m.selectedISO) + "\n" +
			"Device: " + m.selectedDevice.DeviceIdentifier + "\n\n" +
			m.progress.View() + "\n\n" +
			helpStyle.Render("Please wait while the USB drive is being created...")

	case stateComplete:
		content = headerStyle.Render("Success!") + "\n\n" +
			successStyle.Render("✓ Bootable USB drive created successfully!") + "\n\n" +
			"Your Windows USB is ready to use.\n\n" +
			helpStyle.Render("Press Enter or Ctrl+C to exit")

	case stateError:
		content = headerStyle.Render("Error") + "\n\n" +
			errorStyle.Render("✗ "+m.error.Error()) + "\n\n" +
			helpStyle.Render("Press ESC to go back • Press Ctrl+C to exit")
	}

	// Wrap content with padding
	style := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width).
		Height(m.height)

	return style.Render(content)
}

func (m model) handleMenuSelection() (tea.Model, tea.Cmd) {
	selected := m.list.SelectedItem().(menuItem)
	if strings.Contains(selected.title, "Smart ISO Discovery") {
		m.state = stateISODiscovery
		m.isLoading = true
		return m, discoverISOsCmd()
	} else if strings.Contains(selected.title, "Browse for ISO") {
		m.state = stateISOSelection
		return m, nil
	} else if strings.Contains(selected.title, "Advanced") {
		// Switch to advanced TUI
		NewAdvancedTUI()
		return m, tea.Quit
	}
	return m, tea.Quit
}

func (m model) handleDeviceSelection() (tea.Model, tea.Cmd) {
	if len(m.devices) == 0 {
		return m, nil
	}

	selectedIdx := m.list.Index()
	if selectedIdx < len(m.devices) {
		m.selectedDevice = &m.devices[selectedIdx]
		m.state = stateProgress
		return m, startFlashingCmd(m.selectedISO, m.selectedDevice)
	}
	return m, nil
}

func (m *model) updateDeviceList() {
	if len(m.devices) == 0 {
		items := []list.Item{
			menuItem{
				title: "❌ No USB devices found",
				desc:  "Please connect a USB drive and try again",
			},
		}
		m.list.SetItems(items)
		m.list.Title = "No Devices Available"
		return
	}

	items := make([]list.Item, len(m.devices))
	for i, dev := range m.devices {
		// Enhanced device display with safety indicators
		safetyIcon := dev.GetSafetyIcon()
		speedInfo := ""
		if dev.USBSpeed != "" {
			speedInfo = fmt.Sprintf(" • %s", dev.USBSpeed)
		}

		items[i] = menuItem{
			title: fmt.Sprintf("%s %s%s", safetyIcon, dev.GetDisplayName(), speedInfo),
			desc:  fmt.Sprintf("%s • %s", dev.GetDeviceDescription(), dev.GetSafetyMessage()),
		}
	}
	m.list.SetItems(items)
	m.list.Title = "Select Target USB Device (🟢=Safe, 🟡=Caution, 🔴=Danger)"
}

func (m *model) updateISOList() {
	if len(m.discoveredISOs) == 0 {
		items := []list.Item{
			menuItem{
				title: "❌ No Windows ISOs found",
				desc:  "Try browsing manually or check Downloads/Desktop folders",
			},
		}
		m.list.SetItems(items)
		m.list.Title = "No ISOs Discovered"
		return
	}

	items := make([]list.Item, len(m.discoveredISOs))
	for i, iso := range m.discoveredISOs {
		// Enhanced ISO display with analysis info
		fsIcon := "💾"
		if iso.RecommendedFS == types.FilesystemNTFS {
			fsIcon = "🔧" // NTFS needed for large files
		}

		items[i] = menuItem{
			title: fmt.Sprintf("%s %s (%s)", fsIcon, filepath.Base(iso.Path), iso.Version),
			desc:  fmt.Sprintf("%s • %s • Found in: %s", iso.GetSizeString(), iso.RecommendedFS, iso.Source),
		}
	}
	m.list.SetItems(items)
	m.list.Title = "Discovered Windows ISOs (💾=FAT32, 🔧=NTFS Recommended)"
}

func (m model) handleISOSelection() (tea.Model, tea.Cmd) {
	if len(m.discoveredISOs) == 0 {
		return m, nil
	}

	selectedIdx := m.list.Index()
	if selectedIdx < len(m.discoveredISOs) {
		m.selectedISO = m.discoveredISOs[selectedIdx].Path
		m.state = stateDeviceSelection
		m.isLoading = true
		return m, loadDevicesCmd()
	}
	return m, nil
}

// Menu item type
type menuItem struct {
	title string
	desc  string
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

// Custom delegate for better styling
func newMenuDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.Styles.SelectedTitle = d.Styles.SelectedTitle.
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
		Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"})

	d.Styles.SelectedDesc = d.Styles.SelectedDesc.
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
		Foreground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"})

	return d
}

// Commands
func loadDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		devices, err := device.ListDevices()
		return loadDevicesMsg{devices: devices, err: err}
	}
}

func discoverISOsCmd() tea.Cmd {
	return func() tea.Msg {
		isos, err := iso.DiscoverWindowsISOs()
		return discoverISOsMsg{isos: isos, err: err}
	}
}

func startFlashingCmd(isoPath string, targetDevice *types.DiskUtilDevice) tea.Cmd {
	// Use the advanced USB creator for reliable USB creation
	return func() tea.Msg {
		// Create advanced USB creator for robust creation
		creator := device.NewAdvancedUSBCreator()
		defer creator.Cleanup()

		// Configure creation options with smart defaults
		config := &device.USBCreationConfig{
			SourcePath:      isoPath,
			TargetDevice:    "/dev/" + targetDevice.DeviceIdentifier,
			Label:           "Windows USB",
			ForceFilesystem: "", // Let analysis determine optimal filesystem
			SkipAnalysis:    false,
			EnableUEFI:      true,
			LegacyBIOS:      true,
			QuickFormat:     true,  // Default to quick format in TUI
			PerformanceMode: false, // Default to compatibility mode
			AllowLargeFiles: true,  // Default to allowing large files
			Verbose:         false,
		}

		// Perform the USB creation
		err := creator.CreateAdvancedWindowsUSB(config)
		if err != nil {
			return flashErrorMsg{err: err}
		}

		return flashCompleteMsg{}
	}
}

func NewTUI() {
	program := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
