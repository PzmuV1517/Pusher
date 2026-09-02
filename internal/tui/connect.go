package tui

import (
	"errors"
	"fmt"
	"io"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/robot"
	tea "github.com/charmbracelet/bubbletea"
)

// A menu cannot ask the question the command line asks. Bubbletea owns the
// screen and the keyboard, so there is nowhere to print "connect now?" and
// nothing to read the answer with. What it can do is say the offer is there and
// give it a key, which is the same bargain: nobody's Wi-Fi is dropped without
// them asking for it.

// connectOffer is a screen's standing offer to go and get the robot.
type connectOffer struct {
	open bool
	busy bool
}

// consider decides whether an offer is worth making, from whatever failed.
//
// Only ErrNoRobot: a robot that is attached and refusing to answer is a
// different problem, and offering to join a network it is already on would be
// pusher confidently doing nothing.
func (c *connectOffer) consider(err error) {
	c.open = errors.Is(err, adb.ErrNoRobot) && robot.Possible()
}

// hint is the line a screen shows in place of the error.
func (c connectOffer) hint() string {
	if !c.open {
		return ""
	}
	if ssid := robot.Network(); ssid != "" {
		return fmt.Sprintf("No robot connected. Press c to join %s and connect.", ssid)
	}
	return "No robot connected. Press c to connect."
}

// working is what the screen says while the radio is busy, which is most of a
// minute of a menu that cannot be typed at.
func (c connectOffer) working() string {
	if ssid := robot.Network(); ssid != "" {
		return fmt.Sprintf("Joining %s and connecting...", ssid)
	}
	return "Connecting to the robot..."
}

// tracesConnectedMsg is the outcome of the run picker going and getting the
// robot.
type tracesConnectedMsg struct {
	err error
}

// connectNow is the connect flow itself, replaced in tests because the real one
// takes the laptop off whatever network it is on.
//
// Its own account of itself is thrown away on purpose: the six lines it prints
// would be painted over the menu and stay there for the rest of the session.
// Everything that matters when it fails is in the error.
var connectNow = func() error { return robot.Connect(io.Discard) }

// connect runs the connect flow and hands the outcome back as a message.
func connect(done func(error) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return done(connectProblem(connectNow()))
	}
}

// noRobotHere is what an entry says when it needs the robot and there is none.
//
// Pointing at the key rather than at another command: a menu that tells you to
// quit it and run `pusher connect` knows perfectly well how to do that itself.
func noRobotHere(offer connectOffer) error {
	if offer.open {
		return errors.New("no robot connected - press c to connect")
	}
	return errors.New("no robot connected - plug in USB or run `pusher connect`")
}

// connectProblem points at the fix, in a menu where the fix is a few keys away.
func connectProblem(err error) error {
	if errors.Is(err, robot.ErrNoProfile) {
		return errors.New("no robot network saved yet - add one under Wi-Fi profiles")
	}
	return err
}
