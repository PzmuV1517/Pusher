package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("246"))
)

// Stage represents a build stage
type Stage string

const (
	StageInit         Stage = "init"
	StageSaveWiFi     Stage = "save_wifi"
	StageConnectWiFi  Stage = "connect_wifi"
	StageConnectADB   Stage = "connect_adb"
	StageDetectGradle Stage = "detect_gradle"
	StageBuild        Stage = "build"
	StageComplete     Stage = "complete"
	StageFailed       Stage = "failed"
)

type spinner struct {
	frames []string
	index  int
}

func newSpinner() spinner {
	return spinner{
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		index:  0,
	}
}

func (s *spinner) next() string {
	frame := s.frames[s.index]
	s.index = (s.index + 1) % len(s.frames)
	return frame
}

type tickMsg time.Time
type doneMsg struct{ err error }
type outputMsg string
type stageMsg Stage

// Model represents the TUI state
type Model struct {
	stage       Stage
	spinner     spinner
	err         error
	output      []string
	maxOutput   int
	currentSSID string
	targetSSID  string
	gradlePath  string
	done        bool
	statusMsg   string
}

// NewModel creates a new TUI model
func NewModel() Model {
	return Model{
		stage:     StageInit,
		spinner:   newSpinner(),
		output:    []string{},
		maxOutput: 10,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

	case tickMsg:
		if !m.done {
			return m, tickCmd()
		}

	case stageMsg:
		m.stage = Stage(msg)

	case outputMsg:
		m.output = append(m.output, string(msg))
		if len(m.output) > m.maxOutput {
			m.output = m.output[len(m.output)-m.maxOutput:]
		}

	case doneMsg:
		m.done = true
		m.err = msg.err
		if msg.err != nil {
			m.stage = StageFailed
		} else {
			m.stage = StageComplete
		}
		return m, tea.Quit
	}

	return m, nil
}

// View renders the UI
func (m Model) View() string {
	if m.done && m.stage == StageComplete {
		return m.renderComplete()
	}

	if m.done && m.stage == StageFailed {
		return m.renderError()
	}

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Pusher - FTC Robot Deployer"))
	b.WriteString("\n\n")

	// Current stage with spinner
	b.WriteString(m.renderStage())
	b.WriteString("\n\n")

	// Status message
	if m.statusMsg != "" {
		b.WriteString(infoStyle.Render(m.statusMsg))
		b.WriteString("\n\n")
	}

	// Output (for Gradle build)
	if len(m.output) > 0 && m.stage == StageBuild {
		b.WriteString("Build output:\n")
		for _, line := range m.output {
			b.WriteString(infoStyle.Render("  " + line))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) renderStage() string {
	var icon string
	var status string
	var spin string

	if !m.done {
		spin = m.spinner.next() + " "
	}

	switch m.stage {
	case StageInit:
		icon = "[*]"
		status = "Initializing..."
	case StageSaveWiFi:
		icon = "[~]"
		status = fmt.Sprintf("Saving current Wi-Fi: %s", m.currentSSID)
	case StageConnectWiFi:
		icon = "[>]"
		status = fmt.Sprintf("Connecting to robot Wi-Fi: %s", m.targetSSID)
	case StageConnectADB:
		icon = "[+]"
		status = "Connecting to robot via ADB..."
	case StageDetectGradle:
		icon = "[*]"
		status = "Detecting Gradle wrapper..."
	case StageBuild:
		icon = "[#]"
		status = "Building and deploying..."
	case StageComplete:
		icon = "[OK]"
		status = "Deployment complete!"
		spin = ""
	case StageFailed:
		icon = "[X]"
		status = "Deployment failed"
		spin = ""
	}

	return fmt.Sprintf("%s%s %s", spin, icon, status)
}

func (m Model) renderComplete() string {
	return successStyle.Render("[OK] Deployment complete!\n\nYour app has been successfully built and deployed to the robot.\n")
}

func (m Model) renderError() string {
	var b strings.Builder
	b.WriteString(errorStyle.Render("[ERROR] Deployment failed\n\n"))
	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s\n", m.err.Error())))
	}
	return b.String()
}

// SetStage updates the current stage
func (m *Model) SetStage(stage Stage) {
	m.stage = stage
}

// SetCurrentSSID sets the current Wi-Fi SSID
func (m *Model) SetCurrentSSID(ssid string) {
	m.currentSSID = ssid
}

// SetTargetSSID sets the target Wi-Fi SSID
func (m *Model) SetTargetSSID(ssid string) {
	m.targetSSID = ssid
}

// SetGradlePath sets the Gradle wrapper path
func (m *Model) SetGradlePath(path string) {
	m.gradlePath = path
}

// SetStatusMsg sets a status message
func (m *Model) SetStatusMsg(msg string) {
	m.statusMsg = msg
}

// AddOutput adds a line to the output
func (m *Model) AddOutput(line string) {
	m.output = append(m.output, line)
	if len(m.output) > m.maxOutput {
		m.output = m.output[len(m.output)-m.maxOutput:]
	}
}

// SetError sets an error
func (m *Model) SetError(err error) {
	m.err = err
	m.stage = StageFailed
	m.done = true
}

// SetComplete marks as complete
func (m *Model) SetComplete() {
	m.stage = StageComplete
	m.done = true
}
