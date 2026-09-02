package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreibanu/pusher/internal/adb"
	tea "github.com/charmbracelet/bubbletea"
)

// withADB puts something called adb on the path, so the offer is decided by the
// test rather than by whether the machine running it has platform tools.
func withADB(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "adb"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// withoutADB is a machine with no platform tools, where connecting is not
// something pusher could offer to do.
func withoutADB(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

// noRobot is what adb.Target returns when nothing is attached.
func noRobot() error {
	return fmt.Errorf("%w - plug in USB or run `pusher connect`", adb.ErrNoRobot)
}

// stubConnect replaces the connect flow, which otherwise drops the machine
// running the tests onto a robot's network.
func stubConnect(t *testing.T, err error) *int {
	t.Helper()

	calls := 0
	old := connectNow
	connectNow = func() error {
		calls++
		return err
	}
	t.Cleanup(func() { connectNow = old })

	return &calls
}

func TestAMenuOffersToConnectRatherThanSayingNo(t *testing.T) {
	withADB(t)

	m := hwModelIn(t)
	m.Update(hwLoadedMsg{err: noRobot()})

	if !m.connect.open {
		t.Fatal("nothing attached and adb present is exactly when the offer is worth making")
	}
	if !strings.Contains(m.View(), "Press c") {
		t.Errorf("the offer has to be visible to be taken:\n%s", m.View())
	}
}

// The offer is only honest for the one failure connecting fixes. A robot that
// is attached and refusing to answer is not helped by joining a network, and
// neither is a laptop with no adb on it.
func TestAMenuOnlyOffersWhenConnectingWouldHelp(t *testing.T) {
	withADB(t)

	m := hwModelIn(t)
	m.Update(hwLoadedMsg{err: errors.New("the robot refused the listing")})

	if m.connect.open {
		t.Error("offered to connect a robot that is already connected")
	}
	if !strings.Contains(m.View(), "refused") {
		t.Errorf("swallowed the real failure:\n%s", m.View())
	}

	withoutADB(t)

	m = hwModelIn(t)
	m.Update(hwLoadedMsg{err: noRobot()})

	if m.connect.open {
		t.Error("offered to connect with no adb to connect with")
	}
}

func TestTakingTheOfferConnects(t *testing.T) {
	withADB(t)
	calls := stubConnect(t, nil)

	m := hwModelIn(t)
	m.Update(hwLoadedMsg{err: noRobot()})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if cmd == nil {
		t.Fatal("c did nothing")
	}
	if m.busy == "" {
		t.Error("a menu that has stopped answering for most of a minute has to say why")
	}

	msg := cmd()
	if *calls != 1 {
		t.Fatalf("ran the connect flow %d times, want 1", *calls)
	}
	if _, ok := msg.(hwConnectedMsg); !ok {
		t.Fatalf("got %T back from connecting", msg)
	}

	m.Update(msg)
	if m.connect.open {
		t.Error("still offering to connect after connecting")
	}
	if !m.loading {
		t.Error("connected without going back for what the menu could not read")
	}
}

func TestAFailedConnectSaysSoAndLeavesTheOfferUp(t *testing.T) {
	withADB(t)
	stubConnect(t, errors.New("the hub is not broadcasting"))

	m := hwModelIn(t)
	m.Update(hwLoadedMsg{err: noRobot()})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m.Update(cmd())

	if m.busy != "" {
		t.Error("left the menu looking busy after the attempt finished")
	}
	if m.err == nil || !strings.Contains(m.err.Error(), "broadcasting") {
		t.Errorf("did not say why it failed: %v", m.err)
	}
	if !m.connect.open {
		t.Error("a failed attempt is the one time somebody most wants to try again")
	}
}

// c is a letter. Screens that read typing get typed into, and a shortcut that
// outranks the alphabet turns naming a configuration into a Wi-Fi switch.
func TestTypingCIsNotARequestToConnect(t *testing.T) {
	withADB(t)
	calls := stubConnect(t, nil)

	m := hwModelIn(t)
	m.Update(hwLoadedMsg{err: noRobot()})

	press(t, m, "enter")
	m.prompt = hwPrompt{title: "Name for the new configuration", action: "new"}
	m.goTo(hwScreenPrompt, 0)

	typeIn(t, m, "comp")

	if *calls != 0 {
		t.Fatal("typing a name connected to the robot instead")
	}
	if m.prompt.value != "comp" {
		t.Errorf("the c never reached the field: %q", m.prompt.value)
	}
}

// The power menu is the other place somebody arrives with no robot attached,
// and it reads its list through the same call, so it gets the same offer.
func TestThePowerMenuOffersToConnectTooAndTakesIt(t *testing.T) {
	withADB(t)
	calls := stubConnect(t, nil)

	m := modelIn(t, "", false)
	m.screen = screenPower
	m.Update(powerListMsg{err: noRobot()})

	if !m.power.connect.open {
		t.Fatal("the power menu had nothing to show and did not say what would fix it")
	}

	view := m.viewPower()
	if !strings.Contains(view, "c connect") {
		t.Errorf("the offer is not reachable from the power menu:\n%s", view)
	}

	_, cmd := m.updatePower(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if cmd == nil {
		t.Fatal("c did nothing in the power menu")
	}

	msg := cmd()
	if *calls != 1 {
		t.Fatalf("ran the connect flow %d times, want 1", *calls)
	}

	m.Update(msg)
	if m.power.connect.open {
		t.Error("still offering to connect after connecting")
	}
	if !m.power.busy {
		t.Error("connected without going back for the recordings it could not read")
	}
}

// While the radio is busy the menu cannot be typed at for most of a minute, so
// it has to say that it is joining a network rather than reading a robot.
func TestThePowerMenuSaysWhatItIsWaitingOn(t *testing.T) {
	withADB(t)
	stubConnect(t, nil)

	m := modelIn(t, "", false)
	m.screen = screenPower
	m.Update(powerListMsg{err: noRobot()})
	m.updatePower(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})

	if view := m.viewPower(); !strings.Contains(view, "onnecting") {
		t.Errorf("a frozen menu that says nothing is a hung menu:\n%s", view)
	}
}

// A menu that tells somebody to quit it and run `pusher connect` knows
// perfectly well how to do that itself. Every entry in the developer menu needs
// the robot, so all of them point at the key instead.
func TestTheDevMenuPointsAtTheKeyNotAnotherCommand(t *testing.T) {
	withADB(t)
	stubConnect(t, nil)

	m := &devModel{height: 40, apk: "app.apk"}
	m.connect.consider(noRobot())

	if !strings.Contains(m.View(), "press c") {
		t.Errorf("no way in from the menu itself:\n%s", m.View())
	}

	m.run(true, false)
	if m.err == nil || !strings.Contains(m.err.Error(), "press c") {
		t.Errorf("running an entry with no robot said %v", m.err)
	}

	// And when connecting is not on offer, it still says the old thing rather
	// than pointing at a key that does nothing.
	off := &devModel{height: 40, apk: "app.apk"}
	off.run(true, false)

	if off.err == nil || !strings.Contains(off.err.Error(), "pusher connect") {
		t.Errorf("with no offer to make it should still say what to do, said %v", off.err)
	}
}
