package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/feature"
	"github.com/andreibanu/pusher/internal/ftcproject"
	"github.com/andreibanu/pusher/internal/notify"
	"github.com/andreibanu/pusher/internal/pathtrace"
	"github.com/andreibanu/pusher/internal/telemetry"
	"github.com/andreibanu/pusher/internal/wifi"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	valueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	unsetStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	scrollStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	cursorOn    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
)

type screen int

const (
	screenMain screen = iota
	screenProfiles
	screenAddProfile
	screenHomeNetwork
	screenThreads
	screenBlob
	screenBlobRuns
	screenBlobToken
	screenUpdate
	screenDeploy
	screenExtreme
)

type addStep int

const (
	stepName addStep = iota
	stepSSID
	stepPassword
)

const defaultHeight = 24

const minVisibleRows = 3

// SettingsModel is the settings menu.
type SettingsModel struct {
	screen screen
	cursor int

	offset int

	height int
	width  int

	cfg      *config.Config
	profiles []string
	networks []string

	step               addStep
	newName            string
	newSSID            string
	input              string
	maskInput          bool
	confirmDeleteIndex int

	blob     blobState
	extreme  extremeState
	root     string
	gateStep int
	update   updateState

	status string
	err    error
	quit   bool
}

func (m *SettingsModel) projectRoot() string {
	if m.root == "" {
		m.root, _ = os.Getwd()
	}
	return m.root
}

// NewSettingsModel builds the settings menu.
func NewSettingsModel() (*SettingsModel, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	m := &SettingsModel{cfg: cfg, confirmDeleteIndex: -1, height: defaultHeight, width: defaultWidth}
	m.blob.limits = pathtrace.DefaultLimits()
	m.refreshProfiles()
	m.refreshBlob()

	// Read once here as well as on the way into its own screen, so the main
	// menu can say whether the project is still set up without going to disk
	// on every keystroke.
	m.refreshExtreme()

	return m, nil
}

// RunSettings opens the settings menu.
func RunSettings() error {
	model, err := NewSettingsModel()
	if err != nil {
		return err
	}

	_, err = tea.NewProgram(model).Run()
	return err
}

func (m *SettingsModel) refreshProfiles() {
	cfg, err := config.Load()
	if err != nil {
		m.err = err
		return
	}
	m.cfg = cfg

	m.profiles = m.profiles[:0]
	for name := range cfg.Profiles {
		m.profiles = append(m.profiles, name)
	}

	sortStrings(m.profiles)
}

func sortStrings(items []string) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j] < items[j-1]; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// Init satisfies tea.Model.
func (m *SettingsModel) Init() tea.Cmd { return nil }

var mainItems = []string{
	"Robot profiles",
	"Home Wi-Fi network",
	"Return to previous Wi-Fi",
	"Prefer USB when attached",
	"Slim APK before every push",
	"Send only changed parts",
	"Gradle threads",
	"blob library",
	"Deploy speed",
	"Pusher Extreme",
	"Dashboard tuning check",
	"Update pusher",
	"Exit",
	"Count this device",
	"Tell me about updates",
}

// Update satisfies tea.Model.
func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.height = size.Height
		m.width = size.Width

		m.offset = clampOffset(m.offset, m.cursor, m.visibleRows(), m.listLength())
		return m, nil
	}

	switch msg := msg.(type) {
	case releaseFoundMsg:
		m.update.checking = false
		m.update.release = msg.release
		m.update.err = msg.err
		return m, nil

	case updateAppliedMsg:
		m.update.busy = false
		m.update.done = msg.err == nil
		m.update.result = msg.result
		m.update.err = msg.err
		return m, nil

	case blobAuthMsg:
		m.blob.checking = false
		m.blob.busy = false
		m.blob.auth = msg.status
		m.blob.creds = msg.creds
		m.cursor, m.offset = 0, 0
		return m, nil

	case blobOpMsg:
		m.blob.busy = false
		m.err = msg.err
		m.status = msg.status
		m.refreshBlob()
		return m, nil
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if key.Type == tea.KeyCtrlC {
		m.quit = true
		return m, tea.Quit
	}

	switch m.screen {
	case screenMain:
		return m.updateMain(key)
	case screenProfiles:
		return m.updateProfiles(key)
	case screenAddProfile:
		return m.updateAddProfile(key)
	case screenHomeNetwork:
		return m.updateHomeNetwork(key)
	case screenThreads:
		return m.updateThreads(key)
	case screenBlob:
		return m.updateBlob(key)
	case screenBlobRuns:
		return m.updateBlobRuns(key)
	case screenBlobToken:
		return m.updateBlobToken(key)
	case screenUpdate:
		return m.updateUpdate(key)
	case screenDeploy:
		return m.updateDeploy(key)
	case screenExtreme:
		return m.updateExtreme(key)
	}

	return m, nil
}

// Position in mainItems of the entry an install only shows once turned on.
// rows() exists because the indexes stop lining up otherwise.
const optionalRow = 7

// mainSections group the settings by what somebody came to change. The order
// here is the order on screen; the numbers are positions in mainItems, so
// regrouping cannot change what an entry does.
var mainSections = []menuSection{
	{"Getting to the robot", []int{0, 1, 2, 3}},
	{"Building and deploying", []int{6, 4, 5, 8}},
	{"Reloading instead of installing", []int{9, 10}},
	{"Extras", []int{optionalRow}},
	{"Pusher itself", []int{11, 14, 13}},
	{"", []int{12}},
}

func (m *SettingsModel) layout() menuLayout {
	enabled := feature.Revealed()

	return arrange(mainSections, func(i int) bool {
		return i != optionalRow || enabled
	})
}

func (m *SettingsModel) rows() []int { return m.layout().Rows }

func (m *SettingsModel) checkGate(key tea.KeyMsg) bool {
	if feature.Revealed() {
		return false
	}

	name := key.String()

	next, done := feature.Match(m.gateStep, name)
	m.gateStep = next

	switch {
	case done:
		m.gateStep = 0
		m.err = feature.Grant()
		m.cursor, m.offset = 0, 0
		return true

	case next > 0 && name == "right":
		return true
	}

	return false
}

func (m *SettingsModel) updateMain(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.checkGate(key) {
		return m, nil
	}

	rows := m.rows()

	switch key.String() {
	case "q", "esc":
		m.quit = true
		return m, tea.Quit

	case "up", "k":
		m.moveCursor(-1, len(rows))
	case "down", "j":
		m.moveCursor(1, len(rows))

	case "enter", " ", "right", "l":
		m.status = ""
		m.err = nil

		switch rows[m.cursor] {
		case 0:
			m.confirmDeleteIndex = -1
			m.refreshProfiles()
			m.goTo(screenProfiles, 0)
		case 1:
			m.loadNetworks()
			m.goTo(screenHomeNetwork, 0)
		case 2:
			m.setStatus(config.SetSwitchBack(!config.GetSwitchBack()), "Return-to-Wi-Fi updated")
		case 3:
			m.setStatus(config.SetPreferUSB(!config.GetPreferUSB()), "USB preference updated")
		case 4:
			m.toggleAutoSlim()
		case 5:
			m.setStatus(config.SetDeltaTransfer(!config.GetDeltaTransfer()), "Delta transfer updated")
		case 6:
			m.input = strconv.Itoa(config.GetThreads())
			m.goTo(screenThreads, 0)
		case 7:
			return m, m.enterBlob()
		case 8:
			m.goTo(screenDeploy, 0)
		case 9:
			m.refreshExtreme()
			m.goTo(screenExtreme, 0)
		case 10:
			m.toggleDashWatch()
		case 11:
			return m, m.enterUpdate()
		case 12:
			m.quit = true
			return m, tea.Quit
		case 13:
			m.toggleTelemetry()
		case 14:
			m.toggleUpdateNotify()
		}
	}

	return m, nil
}

func (m *SettingsModel) updateProfiles(key tea.KeyMsg) (tea.Model, tea.Cmd) {

	if m.confirmDeleteIndex >= 0 {
		if key.String() == "y" {
			name := m.profiles[m.confirmDeleteIndex]
			m.setStatus(config.DeleteProfile(name), fmt.Sprintf("Deleted %q", name))
			m.refreshProfiles()
			if m.cursor >= len(m.profiles) && m.cursor > 0 {
				m.cursor = len(m.profiles) - 1
			}
		} else {
			m.status = "Delete cancelled"
		}
		m.confirmDeleteIndex = -1
		return m, nil
	}

	switch key.String() {
	case "esc", "q", "left", "h":
		m.goTo(screenMain, 0)
		m.status = ""

	case "up", "k":
		m.moveCursor(-1, len(m.profiles))
	case "down", "j":
		m.moveCursor(1, len(m.profiles))

	case "a":
		m.step = stepName
		m.input = ""
		m.maskInput = false
		m.newName, m.newSSID = "", ""
		m.goTo(screenAddProfile, 0)
		m.status = ""

	case "d":
		if len(m.profiles) > 0 {
			m.confirmDeleteIndex = m.cursor
		}

	case "enter", " ":
		if len(m.profiles) > 0 {
			name := m.profiles[m.cursor]
			m.setStatus(config.SetDefaultProfile(name), fmt.Sprintf("%q is now the default robot", name))
			m.refreshProfiles()
		}
	}

	return m, nil
}

func (m *SettingsModel) updateAddProfile(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		m.goTo(screenProfiles, 0)
		m.input = ""
		m.status = "Cancelled"
		return m, nil

	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil

	case tea.KeyEnter:
		value := strings.TrimSpace(m.input)

		switch m.step {
		case stepName:
			if value == "" {
				m.err = fmt.Errorf("profile name cannot be empty")
				return m, nil
			}
			m.newName = value
			m.step = stepSSID
			m.input = ""
			m.err = nil

		case stepSSID:
			if value == "" {
				m.err = fmt.Errorf("SSID cannot be empty")
				return m, nil
			}
			m.newSSID = value
			m.step = stepPassword
			m.input = ""
			m.maskInput = true
			m.err = nil

		case stepPassword:

			m.setStatus(
				config.AddProfile(m.newName, m.newSSID, m.input),
				fmt.Sprintf("Added profile %q", m.newName),
			)
			m.input = ""
			m.maskInput = false
			m.refreshProfiles()
			m.goTo(screenProfiles, 0)
		}
		return m, nil

	case tea.KeySpace:
		m.input += " "
		return m, nil

	case tea.KeyRunes:
		m.input += string(key.Runes)
		return m, nil
	}

	return m, nil
}

func (m *SettingsModel) updateHomeNetwork(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "q", "left", "h":
		m.goTo(screenMain, 0)

	case "up", "k":
		m.moveCursor(-1, len(m.networks)+1)
	case "down", "j":
		m.moveCursor(1, len(m.networks)+1)

	case "enter", " ":

		if m.cursor == 0 {
			m.setStatus(config.SetHomeSSID(""), "Home network cleared")
		} else if m.cursor-1 < len(m.networks) {
			ssid := m.networks[m.cursor-1]
			m.setStatus(config.SetHomeSSID(ssid), fmt.Sprintf("Home network set to %q", ssid))
		}
		m.goTo(screenMain, 1)
	}

	return m, nil
}

func (m *SettingsModel) updateThreads(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		m.goTo(screenMain, 6)
		m.input = ""
		return m, nil

	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil

	case tea.KeyEnter:
		count, err := strconv.Atoi(strings.TrimSpace(m.input))
		if err != nil || count < 1 {
			m.err = fmt.Errorf("enter a positive whole number")
			return m, nil
		}
		m.err = nil
		m.setStatus(config.SetThreads(count), fmt.Sprintf("Gradle threads set to %d", count))
		m.goTo(screenMain, 6)
		return m, nil

	case tea.KeyRunes:
		for _, r := range key.Runes {
			if r >= '0' && r <= '9' {
				m.input += string(r)
			}
		}
		return m, nil
	}

	return m, nil
}

func (m *SettingsModel) toggleAutoSlim() {
	enabling := !config.GetAutoSlim()

	// Said before it is turned on, since the alternative is finding out at the
	// next deploy that nothing was slimmed.
	if enabling {
		if reason := ftcproject.Supported(m.projectRoot()); reason != nil {
			m.err = reason
			return
		}
	}

	if err := config.SetAutoSlim(enabling); err != nil {
		m.err = err
		return
	}

	m.err = nil
	switch {
	case !enabling:
		m.status = "Pushes will package every architecture again"
	case config.GetHubABI() == "":
		m.status = "On, but connect the robot and run 'pusher slim' once first"
	default:
		m.status = fmt.Sprintf("On: pushes will package %s only", config.GetHubABI())
	}
}

func (m *SettingsModel) moveCursor(delta, length int) {
	if length <= 0 {
		m.cursor = 0
		m.offset = 0
		return
	}

	m.cursor = (m.cursor + delta + length) % length
	m.offset = clampOffset(m.offset, m.cursor, m.visibleRows(), length)
}

func (m *SettingsModel) goTo(target screen, cursor int) {
	m.screen = target
	m.cursor = cursor
	m.offset = clampOffset(0, cursor, m.visibleRows(), m.listLength())
}

func (m *SettingsModel) listLength() int {
	switch m.screen {
	case screenMain:
		return len(m.rows())
	case screenProfiles:
		return len(m.profiles)
	case screenHomeNetwork:

		return len(m.networks) + 1
	case screenBlob:
		return len(m.blobMenuItems())
	case screenBlobRuns:
		return len(m.blob.traces)
	case screenBlobToken:
		return 0
	case screenUpdate:
		return 0
	case screenDeploy:
		return len(deployItems)
	case screenExtreme:
		return len(extremeItems)
	}
	return 0
}

func (m *SettingsModel) visibleRows() int {
	chrome := 10

	// The headings are lines too, and not counting them pushes the last entry
	// off the bottom on a short terminal.
	if m.screen == screenMain {
		chrome += m.layout().Extra()
	}

	rows := m.height - chrome
	if rows < minVisibleRows {
		return minVisibleRows
	}
	return rows
}

func clampOffset(offset, cursor, visible, total int) int {
	if visible <= 0 || total <= visible {
		return 0
	}

	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+visible {
		offset = cursor - visible + 1
	}

	if max := total - visible; offset > max {
		offset = max
	}
	if offset < 0 {
		offset = 0
	}

	return offset
}

func (m *SettingsModel) setStatus(err error, success string) {
	if err != nil {
		m.err = err
		m.status = ""
		return
	}
	m.err = nil
	m.status = success
	m.refreshProfiles()
}

func (m *SettingsModel) loadNetworks() {
	networks, err := wifi.NewManager().PreferredNetworks()
	if err != nil {
		m.err = err
		m.networks = nil
		return
	}
	m.networks = networks
}

// View satisfies tea.Model.
func (m *SettingsModel) View() string {
	if m.quit {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Pusher Settings"))
	b.WriteString("\n\n")

	switch m.screen {
	case screenMain:
		b.WriteString(m.viewMain())
	case screenProfiles:
		b.WriteString(m.viewProfiles())
	case screenAddProfile:
		b.WriteString(m.viewAddProfile())
	case screenHomeNetwork:
		b.WriteString(m.viewHomeNetwork())
	case screenThreads:
		b.WriteString(m.viewThreads())
	case screenBlob:
		b.WriteString(m.viewBlob())
	case screenBlobRuns:
		b.WriteString(m.viewBlobRuns())
	case screenBlobToken:
		b.WriteString(m.viewBlobToken())
	case screenUpdate:
		b.WriteString(m.viewUpdate())
	case screenDeploy:
		b.WriteString(m.viewDeploy())
	case screenExtreme:
		b.WriteString(m.viewExtreme())
	}

	// Fitted, because fill() budgets exactly two lines for this. A message
	// wider than the terminal wraps onto a third, the view comes out taller
	// than the screen, and the previous frame's bottom rows are left behind.
	// That is what "toggling a setting breaks the menu" looks like.
	room := textWidth(m.width) - 2

	if m.err != nil {
		b.WriteString("\n" + errStyle.Render("  ! "+fit(m.err.Error(), room)) + "\n")
	} else if m.status != "" {
		b.WriteString("\n" + okStyle.Render("  ✓ "+fit(m.status, room)) + "\n")
	}

	return clamp(b.String(), m.width, m.height)
}

func (m *SettingsModel) toggleDashWatch() {
	enabling := !config.GetDashWatch()

	if err := config.SetDashWatch(enabling); err != nil {
		m.err = err
		return
	}

	m.err = nil
	if enabling {
		m.status = "On: every push says what dashboard tuning it overwrote"
		return
	}
	m.status = "Off: `pusher dash diff` still compares on demand"
}

func (m *SettingsModel) toggleTelemetry() {
	if !telemetry.Configured() {
		m.err = nil
		m.status = "This build has no counter to talk to, so nothing is sent"
		return
	}

	enabling := !config.GetTelemetry()

	if err := config.SetTelemetry(enabling); err != nil {
		m.err = err
		return
	}

	m.err = nil
	if enabling {
		m.status = "On: a random ID, the version and your OS, once a day"
		return
	}
	m.status = "Off: pusher sends nothing at all"
}

// telemetryLabel says what is actually happening, which is not always what the
// setting says: an unconfigured build sends nothing however this is set.
func (m *SettingsModel) telemetryLabel() string {
	if !telemetry.Configured() {
		return "not set up"
	}
	if !telemetry.Enabled() && config.GetTelemetry() {
		return "off (PUSHER_NO_TELEMETRY)"
	}
	return onOff(config.GetTelemetry())
}

func (m *SettingsModel) toggleUpdateNotify() {
	enabling := !config.GetUpdateNotify()

	if err := config.SetUpdateNotify(enabling); err != nil {
		m.err = err
		return
	}

	m.err = nil
	if enabling {
		m.status = "On: a desktop notification when a newer pusher is out"
		return
	}
	m.status = "Off: pusher will not check for newer versions"
}

// updateNotifyLabel says what will happen, which is not the setting alone: a
// machine with nowhere to deliver a notification is told so here rather than
// left wondering why one never arrives.
func (m *SettingsModel) updateNotifyLabel() string {
	if !config.GetUpdateNotify() {
		return "off"
	}
	if !notify.Enabled() {
		return "on (no notifications here)"
	}
	return "on"
}

func (m *SettingsModel) viewMain() string {
	values := []string{
		m.defaultProfileLabel(),
		orUnset(config.GetHomeSSID(), "not set"),
		onOff(config.GetSwitchBack()),
		onOff(config.GetPreferUSB()),
		m.autoSlimLabel(),
		onOff(config.GetDeltaTransfer()),
		strconv.Itoa(config.GetThreads()),
		m.blobLabel(),
		m.deployLabel(),
		m.extremeLabel(),
		onOff(config.GetDashWatch()),
		m.updateLabel(),
		"",
		m.telemetryLabel(),
		m.updateNotifyLabel(),
	}

	list := m.layout()

	return m.fill("", "\n"+helpStyle.Render("  "+fit("↑/↓ move · enter select · q quit", textWidth(m.width)))+"\n",
		len(list.Rows), func(i int) string {
			return list.render(i,
				renderRow(i == m.cursor, mainItems[list.Rows[i]], values[list.Rows[i]], 29, m.width),
				m.width)
		})
}

func (m *SettingsModel) autoSlimLabel() string {
	if !config.GetAutoSlim() {
		return "off"
	}
	if abi := config.GetHubABI(); abi != "" {
		return "on (" + abi + ")"
	}
	return "on (hub unknown)"
}

func (m *SettingsModel) defaultProfileLabel() string {
	if m.cfg == nil || m.cfg.DefaultProfile == "" {
		return "none"
	}
	if profile, ok := m.cfg.Profiles[m.cfg.DefaultProfile]; ok {
		return fmt.Sprintf("%s (%s)", m.cfg.DefaultProfile, profile.SSID)
	}
	return m.cfg.DefaultProfile
}

func (m *SettingsModel) viewProfiles() string {
	var b strings.Builder
	b.WriteString(helpStyle.Render("  Robot profiles") + "\n\n")

	if len(m.profiles) == 0 {
		b.WriteString(unsetStyle.Render("  "+fit("No profiles yet. Press 'a' to add one", textWidth(m.width))) + "\n")
	}

	head := b.String()

	return m.fill(head,
		"\n"+helpStyle.Render("  "+fit("enter set default · a add · d delete · esc back", textWidth(m.width)))+"\n",
		len(m.profiles), func(i int) string {
			name := m.profiles[i]

			if i == m.confirmDeleteIndex {
				return errStyle.Render(fmt.Sprintf("  > %s: delete? (y/n)", name)) + "\n"
			}

			marker := " "
			if m.cfg != nil && name == m.cfg.DefaultProfile {
				marker = "*"
			}

			ssid := ""
			if m.cfg != nil {
				if profile, ok := m.cfg.Profiles[name]; ok {
					ssid = profile.SSID
				}
			}

			return renderRow(i == m.cursor, fmt.Sprintf("%s %s", marker, name), ssid, 26, m.width)
		})
}

func (m *SettingsModel) viewAddProfile() string {
	labels := map[addStep]string{
		stepName:     "Profile name",
		stepSSID:     "Robot Wi-Fi SSID",
		stepPassword: "Robot Wi-Fi password",
	}

	shown := m.input
	if m.maskInput {
		shown = strings.Repeat("•", len(m.input))
	}

	var b strings.Builder
	b.WriteString(helpStyle.Render(fmt.Sprintf("  Add profile (step %d of 3)", int(m.step)+1)) + "\n\n")
	b.WriteString(fmt.Sprintf("  %s: %s\n", labels[m.step], valueStyle.Render(shown+"▌")))

	if m.step > stepName {
		b.WriteString("\n" + unsetStyle.Render("  name: "+m.newName) + "\n")
	}
	if m.step > stepSSID {
		b.WriteString(unsetStyle.Render("  ssid: "+m.newSSID) + "\n")
	}

	b.WriteString("\n" + helpStyle.Render("  enter next · esc cancel") + "\n")
	return b.String()
}

func (m *SettingsModel) viewHomeNetwork() string {
	var b strings.Builder
	b.WriteString(helpStyle.Render("  "+fit("Network to return to after deploying", textWidth(m.width))) + "\n\n")

	if len(m.networks) == 0 {
		b.WriteString(unsetStyle.Render("  No saved Wi-Fi networks found") + "\n")
	}

	current := config.GetHomeSSID()

	return m.fill(b.String(),
		"\n"+helpStyle.Render("  "+fit("↑/↓ move · enter select · esc back", textWidth(m.width)))+"\n",
		len(m.networks)+1, func(i int) string {
			if i == 0 {
				return renderRow(m.cursor == 0, "(none, stay on the robot)", "", 32, m.width)
			}

			ssid := m.networks[i-1]
			value := ""
			if ssid == current {
				value = "current"
			}
			return renderRow(m.cursor == i, ssid, value, 32, m.width)
		})
}

func (m *SettingsModel) viewThreads() string {
	var b strings.Builder
	b.WriteString(helpStyle.Render("  Gradle worker threads") + "\n\n")
	b.WriteString(fmt.Sprintf("  Threads: %s\n", valueStyle.Render(m.input+"▌")))
	b.WriteString("\n" + helpStyle.Render("  "+fit("enter save · esc cancel", textWidth(m.width))) + "\n")
	return b.String()
}

// helpBlock renders the note for the selected row at a fixed height.
//
// A screen whose height changes as the cursor moves leaves the taller frame's
// leftovers behind, which reads as the menu being broken while scrolling. Every
// note is padded to the same number of lines instead.

// fill lays a list into whatever room is left once everything else on the
// screen has been laid out.
//
// Measured rather than guessed at. The room a list has was a constant subtracted
// from the terminal height, which was too pessimistic on a short terminal, where
// it scrolled a list that would have fitted, and too crude on a tall one, where
// the extra lines went unused. Neither is something a person should have to
// work around by resizing.
//
// Rows are counted as the height they render to, so an entry carrying a group
// heading takes the two lines it really occupies.
func (m *SettingsModel) fill(before, after string, total int, row func(int) string) string {
	// The title and the blank under it, and the status line when there is one.
	// Measured in rows the terminal will actually use, not newlines.
	chrome := 2 + height(before, m.width) + height(after, m.width)
	if m.err != nil || m.status != "" {
		chrome += 2
	}

	budget := m.height - chrome
	if budget < minVisibleRows {
		budget = minVisibleRows
	}

	rendered := make([]string, total)
	tall := make([]int, total)
	for i := range rendered {
		rendered[i] = row(i)
		tall[i] = height(rendered[i], m.width)
	}

	start, end := window(tall, m.offset, m.cursor, budget)
	m.offset = start

	var b strings.Builder
	b.WriteString(before)

	if start > 0 {
		b.WriteString(scrollStyle.Render(fmt.Sprintf("   ↑ %d more above", start)) + "\n")
	}
	for i := start; i < end; i++ {
		b.WriteString(rendered[i])
	}
	if end < total {
		b.WriteString(scrollStyle.Render(fmt.Sprintf("   ↓ %d more below", total-end)) + "\n")
	}

	b.WriteString(after)
	return b.String()
}

// window picks the run of rows to show: as many as fit, keeping the cursor
// inside, and leaving room for the markers that say a list continues.
func window(tall []int, offset, cursor, budget int) (start, end int) {
	total := len(tall)
	if total == 0 {
		return 0, 0
	}

	fits := func(from int) int {
		used, to := 0, from
		if from > 0 {
			used++
		}
		for to < total {
			room := budget - used
			if to+1 < total {
				room--
			}
			// The first row goes in whatever the budget, so a terminal too
			// short for one entry and its markers shows the entry rather than
			// two arrows and nothing between them.
			if to > from && tall[to] > room {
				break
			}
			used += tall[to]
			to++
		}
		return to
	}

	start = offset
	if start > cursor {
		start = cursor
	}
	if start < 0 {
		start = 0
	}

	// Walk the top down until the cursor is on screen. Terminates: the cursor
	// is always inside a window that starts at it.
	for start < cursor && fits(start) <= cursor {
		start++
	}

	// And back up while there is room going spare, so the last screenful is
	// full rather than a few rows against the bottom.
	for start > 0 {
		if fits(start-1) <= cursor {
			break
		}
		start--
	}

	return start, fits(start)
}

func (m *SettingsModel) renderList(total int, row func(int) string) string {
	visible := m.visibleRows()

	start := m.offset
	if start > total-visible {
		start = total - visible
	}
	if start < 0 {
		start = 0
	}

	end := start + visible
	if end > total {
		end = total
	}

	var b strings.Builder

	if start > 0 {
		b.WriteString(scrollStyle.Render(fmt.Sprintf("   ↑ %d more above", start)) + "\n")
	}

	for i := start; i < end; i++ {
		b.WriteString(row(i))
	}

	if end < total {
		b.WriteString(scrollStyle.Render(fmt.Sprintf("   ↓ %d more below", total-end)) + "\n")
	}

	return b.String()
}

// renderRow draws one entry. column is where the values would like to line up
// and width is the terminal, which wins: a row wider than the terminal wraps,
// and a view built to a fixed number of lines then leaves the taller frame
// behind on screen.
//
// Both texts are cut before they are styled, since trimming a string that
// already carries colour escapes can cut one in half.
func renderRow(selected bool, label, value string, column, width int) string {
	// Only an unset width falls back to a default. Treating a narrow terminal
	// as eighty columns is what padded rows out past the edge of it.
	if width <= 0 {
		width = defaultWidth
	}

	// The value keeps what it needs, up to leaving the label something to live
	// in, and the label takes what is left.
	value = fit(value, width-rowPrefix-1-8)
	column = labelWidth(width, column, lipgloss.Width(value))
	label = fit(label, column)

	prefix := "   "
	if selected {
		prefix = cursorOn.Render(" > ")
		label = cursorOn.Render(label)
	}

	pad := column - lipgloss.Width(label)
	if pad < 1 {
		pad = 1
	}

	if value == "" {
		return prefix + label + "\n"
	}

	return prefix + label + strings.Repeat(" ", pad) + valueStyle.Render(value) + "\n"
}

func onOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func orUnset(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
