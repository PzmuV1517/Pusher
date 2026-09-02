package tui

import (
	"sort"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/robotcfg"
	tea "github.com/charmbracelet/bubbletea"
)

type hwScreen int

const (
	hwScreenList hwScreen = iota
	hwScreenActions
	hwScreenDevices
	hwScreenDevice
	hwScreenPrompt
	hwScreenConfirm
	hwScreenSummary
)

type hwEntry struct {
	Name    string
	InLocal bool
	OnRobot bool
	Same    bool
	Known   bool
	Active  bool
}

func (e hwEntry) where() string {
	switch {
	case e.InLocal && e.OnRobot:
		return "project + robot"
	case e.InLocal:
		return "project only"
	default:
		return "robot only"
	}
}

func (e hwEntry) status() string {
	switch {
	case e.InLocal && e.OnRobot && e.Known && e.Same:
		return "same"
	case e.InLocal && e.OnRobot && e.Known:
		return "differs"
	case e.InLocal && e.OnRobot:
		return "on both"
	case e.InLocal:
		return "not on the robot"
	default:
		return "not pulled"
	}
}

type hwRowKind int

const (
	hwRowPortal hwRowKind = iota
	hwRowModule
	hwRowDevice
	hwRowAddDevice
	hwRowAddModule
)

type hwRow struct {
	Kind   hwRowKind
	Label  string
	Detail string
	Slot   robotcfg.Slot
	Portal int
	Module int
	Issue  robotcfg.Level
	HasIss bool
}

func (r hwRow) selectable() bool {
	return r.Kind != hwRowPortal
}

type hwField int

const (
	hwFieldType hwField = iota
	hwFieldName
	hwFieldPort
	hwFieldBus
)

type hwForm struct {
	field   hwField
	adding  bool
	slot    robotcfg.Slot
	portal  int
	module  int
	typed   string
	name    string
	port    string
	bus     string
	suggest []string
	pick    int
	problem string
}

type hwPrompt struct {
	title  string
	value  string
	action string
}

type hwConfirm struct {
	title  string
	detail string
	action string
	name   string
}

type hwModel struct {
	screen hwScreen
	cursor int
	offset int
	height int
	width  int

	store  *robotcfg.Store
	serial string

	robot  []string
	local  []string
	hashes map[string]string
	active string

	entries []hwEntry

	sel    string
	cfg    *robotcfg.Config
	saved  string
	dirty  bool
	issues robotcfg.Issues
	rows   []hwRow

	form    hwForm
	prompt  hwPrompt
	confirm hwConfirm

	loading bool
	connect connectOffer
	busy    string
	status  string
	err     error
	quit    bool
}

type hwLoadedMsg struct {
	serial string
	names  []string
	hashes map[string]string
	active string
	err    error
}

// hwConnectedMsg is the outcome of the menu going and getting the robot.
type hwConnectedMsg struct {
	err error
}

type hwOpMsg struct {
	status string
	err    error
	reload bool
}

// newHWModel builds the hardware configuration menu.
//
// Deliberately with no size on it: nothing is drawn until the terminal has said
// how big it is, and a default here is a full height frame painted into a
// window that may be half that.
func newHWModel(dir string) *hwModel {
	m := &hwModel{store: robotcfg.NewStore(dir)}

	m.refreshLocal()
	m.rebuildEntries()
	m.loading = true

	return m
}

// RunHWConfig opens the hardware configuration menu.
func RunHWConfig(dir string) error {
	_, err := tea.NewProgram(newHWModel(dir)).Run()
	return err
}

// Init satisfies tea.Model.
func (m *hwModel) Init() tea.Cmd { return hwLoad }

func hwLoad() tea.Msg {
	serial, err := adb.Target()
	if err != nil {
		return hwLoadedMsg{err: err}
	}

	names, err := robotcfg.List(serial)
	if err != nil {
		return hwLoadedMsg{serial: serial, err: err}
	}

	return hwLoadedMsg{
		serial: serial,
		names:  names,
		hashes: robotcfg.Hashes(serial),
		active: robotcfg.ActiveConfig(serial),
	}
}

func (m *hwModel) refreshLocal() {
	names, err := m.store.Names()
	if err != nil {
		m.err = err
		return
	}
	m.local = names
}

func (m *hwModel) rebuildEntries() {
	inLocal := map[string]bool{}
	for _, name := range m.local {
		inLocal[name] = true
	}
	onRobot := map[string]bool{}
	for _, name := range m.robot {
		onRobot[name] = true
	}

	seen := map[string]bool{}
	var all []string
	for _, list := range [][]string{m.local, m.robot} {
		for _, name := range list {
			if !seen[name] {
				seen[name] = true
				all = append(all, name)
			}
		}
	}
	sort.Strings(all)

	m.entries = m.entries[:0]
	for _, name := range all {
		entry := hwEntry{
			Name:    name,
			InLocal: inLocal[name],
			OnRobot: onRobot[name],
			Active:  name != "" && name == m.active,
		}

		if digest, known := m.hashes[name]; known && inLocal[name] {
			if data, err := m.store.Read(name); err == nil {
				entry.Known = true
				entry.Same = robotcfg.Hash(data) == digest
			}
		}

		m.entries = append(m.entries, entry)
	}
}

// Update satisfies tea.Model.
func (m *hwModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.offset = clampOffset(m.offset, m.cursor, m.visibleRows(), m.listLength())
		return m, nil

	case hwLoadedMsg:
		m.loading = false
		m.serial = msg.serial
		m.robot = msg.names
		m.hashes = msg.hashes
		m.active = msg.active

		// The offer is rendered on its own, so putting it in the status line
		// too would say it twice, once of them in the register of good news.
		m.connect.consider(msg.err)
		if msg.err != nil && !m.connect.open {
			m.status = "No robot connected. " + msg.err.Error()
		}

		m.rebuildEntries()
		return m, nil

	case hwConnectedMsg:
		m.connect.busy = false
		m.busy = ""

		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		// Connected, so everything this screen could not read is readable now.
		m.connect.open = false
		m.loading = true
		return m, hwLoad

	case hwOpMsg:
		m.busy = ""
		m.err = msg.err
		if msg.err == nil {
			m.status = msg.status
		}
		m.refreshLocal()
		if msg.reload {
			m.loading = true
			return m, hwLoad
		}
		m.rebuildEntries()
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

	if m.busy != "" {
		return m, nil
	}

	// Taken from any screen that is not reading typing, because a menu that
	// could not reach the robot got here with half of itself missing, and the
	// fix is the same wherever somebody has wandered to since.
	if key.String() == "c" && m.connect.open && m.screen != hwScreenPrompt && m.screen != hwScreenDevice {
		m.connect.busy = true
		m.busy = m.connect.working()
		m.err, m.status = nil, ""

		return m, connect(func(err error) tea.Msg { return hwConnectedMsg{err: err} })
	}

	switch m.screen {
	case hwScreenList:
		return m.updateHWList(key)
	case hwScreenActions:
		return m.updateHWActions(key)
	case hwScreenDevices:
		return m.updateHWDevices(key)
	case hwScreenDevice:
		return m.updateHWDevice(key)
	case hwScreenPrompt:
		return m.updateHWPrompt(key)
	case hwScreenConfirm:
		return m.updateHWConfirm(key)
	case hwScreenSummary:
		return m.updateHWSummary(key)
	}

	return m, nil
}

func (m *hwModel) goTo(target hwScreen, cursor int) {
	m.screen = target
	m.cursor = cursor
	m.offset = clampOffset(0, cursor, m.visibleRows(), m.listLength())
}

func (m *hwModel) move(delta, length int) {
	if length <= 0 {
		return
	}
	m.cursor = (m.cursor + delta + length) % length
	m.offset = clampOffset(m.offset, m.cursor, m.visibleRows(), length)
}

func (m *hwModel) visibleRows() int {
	const chrome = 12

	rows := m.height - chrome
	if rows < minVisibleRows {
		return minVisibleRows
	}
	return rows
}

func (m *hwModel) listLength() int {
	switch m.screen {
	case hwScreenList:
		return len(m.entries) + len(hwListExtras)
	case hwScreenActions:
		return len(m.actionItems())
	case hwScreenDevices:
		return len(m.rows)
	}
	return 0
}

func (m *hwModel) clear() {
	m.err = nil
	m.status = ""
}
