package tui

import (
	"strings"
	"testing"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/hubwifi"
	tea "github.com/charmbracelet/bubbletea"
)

// The idea is somebody else's and the credit is part of the feature, not a
// decoration on it. A refactor that tidies the screen and loses the line is a
// refactor that took the credit off.
func TestTheRelayScreenCreditsWhoseIdeaItWas(t *testing.T) {
	m := &SettingsModel{height: 30, width: 90, confirmDeleteIndex: -1, screen: screenRelay}

	view := stripANSI(m.View())
	for _, want := range []string{"Dhruv", "32001L"} {
		if !strings.Contains(view, want) {
			t.Errorf("the relay screen does not credit %q:\n%s", want, view)
		}
	}
}

// A Control Hub cannot join a network itself, its radio being its own access
// point, and somebody turning this on needs to know that before they go looking
// for a setting that does not exist.
func TestTheRelayScreenSaysWhatGetsARobotOntoYourNetwork(t *testing.T) {
	m := &SettingsModel{height: 30, width: 90, confirmDeleteIndex: -1, screen: screenRelay}

	view := stripANSI(m.View())
	for _, want := range []string{"cannot join", "Ethernet"} {
		if !strings.Contains(view, want) {
			t.Errorf("the screen does not explain %q:\n%s", want, view)
		}
	}
}

// Every remembered address is somewhere to switch back to, so they have to be
// on the screen and selectable rather than only in the config file.
func TestRememberedAddressesAreListedAndSelectable(t *testing.T) {
	m := &SettingsModel{height: 30, width: 90, confirmDeleteIndex: -1, screen: screenRelay}
	m.relay.spots = []config.Spot{
		{Network: "ICHB-Robotics", Address: "10.0.0.42:5555"},
		{Network: "venue", Address: "192.168.1.183:5555"},
	}

	view := stripANSI(m.View())
	for _, want := range []string{"ICHB-Robotics", "10.0.0.42:5555", "venue"} {
		if !strings.Contains(view, want) {
			t.Errorf("the screen does not list %q:\n%s", want, view)
		}
	}

	if got := m.relayLength(); got != len(relayItems)+2 {
		t.Errorf("the list is %d long, so the addresses cannot all be reached", got)
	}

	// And the cursor reaches them.
	for i := 0; i < m.relayLength()-1; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.cursor != m.relayLength()-1 {
		t.Errorf("the cursor stopped at %d of %d", m.cursor, m.relayLength()-1)
	}
}

// The relay is its own thing, not a footnote under getting to the robot: it
// changes what network the robot itself is on, which nothing else in pusher
// does.
func TestTheRelayHasItsOwnCategory(t *testing.T) {
	m := &SettingsModel{height: 40, width: 90, confirmDeleteIndex: -1, screen: screenMain}

	view := stripANSI(m.View())
	if !strings.Contains(view, "Reaching the robot over your own network") {
		t.Errorf("no category of its own on the main menu:\n%s", view)
	}
}

// A passphrase typed into a menu in a room full of people should not be on the
// screen.
func TestTheNetworkPasswordIsMasked(t *testing.T) {
	m := &SettingsModel{height: 30, width: 90, confirmDeleteIndex: -1, screen: screenRelayNetwork}
	m.relay.askingPassword = true
	m.relay.ssid = "ASUS"
	m.input = "hunter2"

	view := stripANSI(m.View())
	if strings.Contains(view, "hunter2") {
		t.Error("the passphrase is on screen in plain text")
	}
	if !strings.Contains(view, "*******") {
		t.Errorf("nothing shown in its place:\n%s", view)
	}
	if !strings.Contains(view, "ASUS") {
		t.Error("the network being joined is not shown")
	}
}

// Loading a driver and joining a network takes a minute or two. A menu that
// says nothing for that long is one somebody quits out of, which leaves a
// half-loaded driver and a supplicant nobody is managing.
func TestTheSetupSaysWhatItIsDoing(t *testing.T) {
	m := &SettingsModel{height: 40, width: 90, confirmDeleteIndex: -1, screen: screenRelay}
	m.relay.busy = true
	m.relay.setup = "working"

	m.Update(relayStepMsg{line: "[*] Loading the driver"})
	m.Update(relayStepMsg{line: "[OK] wlan1 came up"})

	view := stripANSI(m.View())
	for _, want := range []string{"Loading the driver", "wlan1 came up", "leave this open"} {
		if !strings.Contains(view, want) {
			t.Errorf("the live view does not show %q:\n%s", want, view)
		}
	}
}

// And it cannot be walked out of while it runs.
func TestTheSetupCannotBeAbandonedHalfway(t *testing.T) {
	m := &SettingsModel{height: 40, width: 90, confirmDeleteIndex: -1, screen: screenRelay}
	m.relay.busy = true

	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenRelay {
		t.Error("esc left the screen while the setup was running")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if m.quit {
		t.Error("q quit pusher while the setup was running")
	}

	// And once it is done, it lets go again.
	m.relay.busy = false
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen == screenRelay {
		t.Error("still stuck on the screen after the setup finished")
	}
}

// A step arriving after the end belongs to nothing on screen.
func TestStepsAfterTheEndAreDropped(t *testing.T) {
	m := &SettingsModel{height: 40, width: 90, confirmDeleteIndex: -1, screen: screenRelay}

	m.Update(relayStepMsg{line: "[*] something"})
	if len(m.relay.log) != 0 {
		t.Error("kept a step from a setup that is not running")
	}
}

// The relay screen needs the robot as much as any other, so it makes the same
// offer rather than reporting a failure somebody has to interpret.
func TestTheRelayScreenOffersToConnect(t *testing.T) {
	withADB(t)

	m := &SettingsModel{height: 40, width: 90, confirmDeleteIndex: -1, screen: screenRelay}
	m.Update(relayScanMsg{err: noRobot()})

	if !m.relay.connect.open {
		t.Fatal("nothing attached is exactly when the offer is worth making")
	}
	if !strings.Contains(stripANSI(m.View()), "Press c") {
		t.Errorf("the offer is not on screen:\n%s", stripANSI(m.View()))
	}
}

// What the robot can hear is the only opinion that matters, and it has to be
// selectable rather than merely printed.
func TestWhatTheRobotHeardIsListedAndSelectable(t *testing.T) {
	m := &SettingsModel{height: 40, width: 100, confirmDeleteIndex: -1, screen: screenRelay}
	m.Update(relayScanMsg{seen: []hubwifi.AP{
		{SSID: "ASUS", Signal: -63, Band: "2.4GHz", Secured: true},
		{SSID: "VenueGuest", Signal: -40, Band: "5GHz"},
	}})

	view := stripANSI(m.View())
	for _, want := range []string{"What the robot can hear", "ASUS", "VenueGuest", "open"} {
		if !strings.Contains(view, want) {
			t.Errorf("the scan does not show %q:\n%s", want, view)
		}
	}

	if got := m.relayLength(); got < len(relayItems)+2 {
		t.Errorf("the list is %d long, so the networks cannot be reached", got)
	}
}
