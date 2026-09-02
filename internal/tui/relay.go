package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/hubwifi"
	"github.com/andreibanu/pusher/internal/robot"
	tea "github.com/charmbracelet/bubbletea"
)

// Switching networks to reach the robot is the thing everybody puts up with and
// nobody wants. When the robot is already somewhere the laptop can reach, none
// of it is necessary: adb does not care which network a device is on as long as
// it can open a socket to it.
//
// Inspired by Dhruv, FTC 32001L, whose ADB relay put a Linux box on the
// robot's access point and bridged adb to the local network. This is a
// different way at the same want: the robot itself joins the network instead.

type relayState struct {
	busy  bool
	err   error
	found string
	setup string
	spots []config.Spot

	// log is what the setup has said so far, newest last. Shown while it runs,
	// because it takes a minute or two and a menu that sits there saying
	// nothing is a menu somebody quits out of halfway through loading a driver.
	log []string

	// Typing the network in: the name first, then the passphrase, which is
	// masked because somebody's home Wi-Fi password should not sit on a screen
	// in a room full of people.
	askingPassword bool
	ssid           string

	// What the robot can hear from where it is sitting, which is the only
	// opinion that matters and not one a laptop can give.
	scanning bool
	seen     []hubwifi.AP

	connect connectOffer
}

type relayFoundMsg struct {
	found robot.Found
	err   error
}

// relaySetupMsg is the outcome of putting the robot on a network.
type relaySetupMsg struct {
	network string
	address string
	err     error
}

// relayScanMsg is what the robot heard.
type relayScanMsg struct {
	seen []hubwifi.AP
	err  error
}

// relayConnectedMsg is the outcome of going to get the robot.
type relayConnectedMsg struct{ err error }

// relayStepMsg is one line of what the setup is doing.
type relayStepMsg struct{ line string }

// relaySteps carries progress back to the menu. Buffered and dropped when full:
// saying what is happening must never hold up the thing that is happening.
var relaySteps = make(chan string, 64)

func waitForRelayStep() tea.Msg { return relayStepMsg{line: <-relaySteps} }

// stepWriter turns what Setup writes into messages for the menu.
type stepWriter struct{ partial string }

func (w *stepWriter) Write(p []byte) (int, error) {
	w.partial += string(p)

	for {
		i := strings.IndexByte(w.partial, '\n')
		if i < 0 {
			break
		}

		line := strings.TrimSpace(w.partial[:i])
		w.partial = w.partial[i+1:]

		if line != "" {
			select {
			case relaySteps <- line:
			default:
			}
		}
	}

	return len(p), nil
}

func (m *SettingsModel) enterRelay() {
	m.relay.spots = config.GetRobotSpots()
	m.relay.err = nil
	m.goTo(screenRelay, 0)
}

// relayItems are the things on the screen that are not a remembered address.
var relayItems = []string{
	"ADB relay",
	"Put the robot on your Wi-Fi",
	"Look for networks near the robot",
	"Find the robot now",
	"Forget every address",
}

func (m *SettingsModel) relayLength() int {
	return len(relayItems) + len(m.relay.networks()) + len(m.relay.seen) + len(m.relay.spots)
}

// networks is what the robot already knows how to join.
func (s relayState) networks() []config.HubNetwork { return config.GetHubNetworks() }

func (m *SettingsModel) updateRelay(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Nothing leaves this screen while the setup is running. Quitting halfway
	// through leaves a driver loaded, a supplicant nobody is managing and an
	// interface with no address, and the next attempt has to clean all of it up
	// before it can start.
	if m.relay.busy {
		switch key.String() {
		case "q", "esc", "left", "h":
			m.status = "Still working. It will say when it is done."
			return m, nil
		}
		return m, nil
	}

	switch key.String() {
	case "q":
		m.quit = true
		return m, tea.Quit

	case "esc", "left", "h":
		m.goTo(screenMain, 0)
		return m, nil

	case "up", "k":
		m.moveCursor(-1, m.relayLength())
	case "down", "j":
		m.moveCursor(1, m.relayLength())

	case "c":
		if !m.relay.connect.open {
			return m, nil
		}
		m.relay.connect.busy = true
		m.relay.busy = true
		m.relay.setup = m.relay.connect.working()
		m.err, m.status = nil, ""
		return m, connect(func(err error) tea.Msg { return relayConnectedMsg{err: err} })

	case "enter", " ", "right", "l":
		if m.relay.busy {
			return m, nil
		}
		m.err, m.status = nil, ""

		at := m.cursor - len(relayItems)

		// A network the robot already knows: join it.
		if saved := m.relay.networks(); at >= 0 && at < len(saved) {
			pick := saved[at]
			_ = config.RememberHubNetwork(pick.SSID, pick.Password)

			m.relay.busy = true
			m.relay.setup = "working"
			m.relay.log = nil
			return m, tea.Batch(m.setUpRelay(pick.SSID, pick.Password), waitForRelayStep)
		}
		at -= len(m.relay.networks())

		// One the robot just heard: ask for the passphrase, then join it.
		if at >= 0 && at < len(m.relay.seen) {
			m.relay.ssid = m.relay.seen[at].SSID
			m.relay.askingPassword = true
			m.input, m.maskInput = "", true
			m.goTo(screenRelayNetwork, 0)
			return m, nil
		}
		at -= len(m.relay.seen)

		// A remembered address, chosen to switch to.
		if at >= 0 && at < len(m.relay.spots) {
			return m, m.useSpot(m.relay.spots[at])
		}

		switch m.cursor {
		case 0:
			m.setStatus(config.SetRelay(!config.GetRelay()), "ADB relay updated")
		case 1:
			ssid, password := config.GetHubNetwork()
			if ssid == "" {
				// Nothing to join yet, so ask before doing anything.
				m.relay.askingPassword = false
				m.relay.ssid = ""
				m.input = ""
				m.maskInput = false
				m.goTo(screenRelayNetwork, 0)
				return m, nil
			}

			m.relay.busy = true
			m.relay.setup = "working"
			m.relay.log = nil
			return m, tea.Batch(m.setUpRelay(ssid, password), waitForRelayStep)
		case 2:
			m.relay.scanning = true
			m.relay.busy = true
			m.relay.setup = "asking the robot what it can hear"
			return m, m.scanNetworks()
		case 3:
			m.relay.busy = true
			m.relay.err = nil
			return m, m.findRobot()
		case 4:
			m.setStatus(config.ForgetSpots(), "Forgotten. The next find starts from nothing")
			m.relay.spots = config.GetRobotSpots()
			m.relay.found = ""
			adb.UseOwnAccessPoint()
			m.cursor = 0
		}
	}

	return m, nil
}

// findRobot sweeps the network. Slow enough to be a command rather than a call,
// so the menu says what it is doing instead of freezing.
func (m *SettingsModel) findRobot() tea.Cmd {
	return func() tea.Msg {
		found, err := robot.Locate(io.Discard, true)
		return relayFoundMsg{found: found, err: err}
	}
}

// useSpot switches to a remembered address, if the robot is still at it.
func (m *SettingsModel) useSpot(spot config.Spot) tea.Cmd {
	return func() tea.Msg {
		found, ok := robot.Try(spot.Address)
		if !ok {
			return relayFoundMsg{err: fmt.Errorf("nothing answered at %s", spot.Address)}
		}
		return relayFoundMsg{found: found}
	}
}

func (m *SettingsModel) viewRelay() string {
	var head strings.Builder

	head.WriteString(helpStyle.Render("  "+fit("ADB relay   reach the robot without leaving your own network", textWidth(m.width))) + "\n\n")

	hubNet, _ := config.GetHubNetwork()

	values := []string{
		onOff(config.GetRelay()),
		orUnset(hubNet, "not set"),
		"",
		"",
		"",
	}
	if m.relay.busy && m.relay.setup != "" {
		if m.relay.scanning {
			values[2] = m.relay.setup
		} else {
			values[1] = m.relay.setup
		}
	}

	rows := make([]string, 0, m.relayLength())
	for i, item := range relayItems {
		rows = append(rows, renderRow(i == m.cursor, item, values[i], 29, m.width))
	}

	at := len(relayItems)

	if saved := m.relay.networks(); len(saved) > 0 {
		rows = append(rows, "\n"+helpStyle.Render("  "+fit("Networks the robot knows", textWidth(m.width)))+"\n")

		current, _ := config.GetHubNetwork()
		for _, network := range saved {
			value := "join"
			if network.SSID == current {
				value = "current"
			}
			rows = append(rows, renderRow(at == m.cursor, network.SSID, value, 29, m.width))
			at++
		}
	}

	if len(m.relay.seen) > 0 {
		rows = append(rows, "\n"+helpStyle.Render("  "+fit("What the robot can hear", textWidth(m.width)))+"\n")

		for _, ap := range m.relay.seen {
			value := ap.Bars() + "  " + ap.Band
			if !ap.Secured {
				value += "  open"
			}
			rows = append(rows, renderRow(at == m.cursor, ap.SSID, value, 29, m.width))
			at++
		}
	}

	if len(m.relay.spots) > 0 {
		rows = append(rows, "\n"+helpStyle.Render("  "+fit("Where the robot has been found", textWidth(m.width)))+"\n")

		for _, spot := range m.relay.spots {
			value := spot.Address
			if spot.Address == adb.RobotAddr() {
				value += "  ← using"
			}
			rows = append(rows, renderRow(at == m.cursor, spot.Network, value, 29, m.width))
			at++
		}
	}

	var tail strings.Builder

	// What it is doing, live. The whole point: this takes a minute or two, and
	// somebody watching a still screen assumes it has hung and quits, which
	// leaves a half loaded driver and a supplicant nobody is managing.
	if m.relay.busy && len(m.relay.log) > 0 {
		tail.WriteString("\n")
		for _, line := range m.relay.log {
			tail.WriteString("  " + scrollStyle.Render(fit(line, textWidth(m.width))) + "\n")
		}
	}

	tail.WriteString("\n")
	for _, line := range wrap(relayNote, textWidth(m.width)) {
		tail.WriteString("  " + helpStyle.Render(line) + "\n")
	}
	tail.WriteString("\n" + helpStyle.Render("  "+fit("Inspired by Dhruv, FTC 32001L", textWidth(m.width))) + "\n")
	if m.relay.connect.open {
		tail.WriteString("\n" + helpStyle.Render("  "+fit(m.relay.connect.hint(), textWidth(m.width))) + "\n")
	}

	if m.relay.busy {
		tail.WriteString("\n" + helpStyle.Render("  "+fit("working, leave this open until it finishes", textWidth(m.width))) + "\n")
	} else {
		tail.WriteString("\n" + helpStyle.Render("  "+fit("enter choose · esc back", textWidth(m.width))) + "\n")
	}

	// The rows are already rendered, so the list is handed over as it is.
	block, start := pane(rows, m.width, m.budget(head.String(), tail.String()), m.offset, m.cursor)
	m.offset = start

	return head.String() + block + tail.String()
}

const relayNote = "Put the robot on a network this laptop is already on and pusher will " +
	"deploy to it there, without touching your Wi-Fi. A Control Hub cannot join a " +
	"network itself: its radio is its own access point. What gets it onto yours is " +
	"a USB Ethernet adapter, or a phone as the robot controller."

// setUpRelay does the whole thing: driver, association, address, routes, and
// the boot hook. Slow enough to be a command rather than a call, so the menu
// says what it is doing rather than freezing for a minute.
func (m *SettingsModel) setUpRelay(ssid, password string) tea.Cmd {
	return func() tea.Msg {
		serial, err := adb.Target()
		if err != nil {
			return relaySetupMsg{err: err}
		}

		found, err := hubwifi.Setup(&stepWriter{}, serial, hubwifi.Options{
			SSID: ssid, Password: password, Persist: true,
		})
		if err != nil {
			return relaySetupMsg{err: err}
		}

		robot.Remember(robot.Found{Addr: found.Address + ":5555", Model: "Control Hub"})
		_ = config.SetRelay(true)

		return relaySetupMsg{network: ssid, address: found.Address}
	}
}

// updateRelayNetwork takes the network name and then the passphrase.
func (m *SettingsModel) updateRelayNetwork(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		m.input, m.maskInput = "", false
		m.relay.askingPassword = false
		m.goTo(screenRelay, 1)
		return m, nil

	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil

	case tea.KeyEnter:
		value := strings.TrimSpace(m.input)

		if !m.relay.askingPassword {
			if value == "" {
				return m, nil
			}
			m.relay.ssid = value
			m.relay.askingPassword = true
			m.input, m.maskInput = "", true
			return m, nil
		}

		ssid, password := m.relay.ssid, m.input
		m.input, m.maskInput = "", false
		m.relay.askingPassword = false

		if err := config.RememberHubNetwork(ssid, password); err != nil {
			m.err = err
			m.goTo(screenRelay, 1)
			return m, nil
		}

		m.goTo(screenRelay, 1)
		m.relay.busy = true
		m.relay.setup = "loading the driver and joining " + ssid
		return m, m.setUpRelay(ssid, password)

	case tea.KeyRunes:
		m.input += string(key.Runes)
		return m, nil
	}

	return m, nil
}

func (m *SettingsModel) viewRelayNetwork() string {
	var b strings.Builder

	b.WriteString(helpStyle.Render("  "+fit("The network for the robot to join, not the one it serves.", textWidth(m.width))) + "\n")
	b.WriteString(helpStyle.Render("  "+fit("Its own access point carries on as it is, for the Driver Station.", textWidth(m.width))) + "\n\n")

	if m.relay.askingPassword {
		b.WriteString(fmt.Sprintf("  Network:  %s\n", m.relay.ssid))
		b.WriteString(fmt.Sprintf("  Password: %s\n", strings.Repeat("*", len(m.input))))
	} else {
		b.WriteString(fmt.Sprintf("  Network:  %s\n", m.input))
		b.WriteString(helpStyle.Render("  Password: ") + "\n")
	}

	b.WriteString("\n" + helpStyle.Render("  "+fit("Stored in pusher's config, and put on the robot so it can rejoin on boot.", textWidth(m.width))) + "\n")
	b.WriteString("\n" + helpStyle.Render("  "+fit("enter next · esc cancel", textWidth(m.width))) + "\n")

	return b.String()
}

// scanNetworks asks the robot what it can hear from where it is.
func (m *SettingsModel) scanNetworks() tea.Cmd {
	return func() tea.Msg {
		serial, err := adb.Target()
		if err != nil {
			return relayScanMsg{err: err}
		}

		seen, err := hubwifi.Scan(serial)
		return relayScanMsg{seen: seen, err: err}
	}
}
