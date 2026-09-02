package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/robot"
	"golang.org/x/term"
)

// requireRobot returns the robot to talk to, and offers to go and get it when
// there is nothing attached.
//
// Every command that needs the robot used to stop at "no robot connected - run
// `pusher connect`", which is one command telling somebody to run a second
// command so they can run the first one again. There is nothing `pusher
// connect` knows that this does not, so it asks instead of instructing.
//
// Asked, not assumed. Connecting takes over the Wi-Fi radio and drops whatever
// network the laptop is on, which is not something to do to somebody who typed
// `pusher hwconfig` to look at a file in their project.
func requireRobot() (string, error) {
	serial, err := adb.Target()
	if err == nil {
		return serial, nil
	}

	// Missing platform tools is the one failure connecting cannot fix, and a
	// pipe is nobody to ask.
	if !errors.Is(err, adb.ErrNoRobot) || !robot.Possible() || !askable() {
		return "", err
	}

	fmt.Println("[!] No robot connected.")
	if !confirmDefaultYes("    " + connectQuestion()) {
		return "", err
	}
	fmt.Println()

	if err := connectNow(); err != nil {
		return "", err
	}

	return adb.Target()
}

// connectQuestion names the network it is about to take the laptop onto, which
// is the part somebody needs to see before saying yes.
func connectQuestion() string {
	if ssid := robot.Network(); ssid != "" {
		return fmt.Sprintf("Join %s and connect now?", ssid)
	}
	return "Connect to the robot now?"
}

// connectNow is the connect flow itself, replaced in tests because the real one
// takes the laptop off whatever network it is on.
var connectNow = connectRobot

// askable reports whether there is somebody at the keyboard to answer.
//
// A question written to a pipe is a program that hangs in somebody's script
// with no visible reason, so scripts get the error they always got.
func askable() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// confirmDefaultYes asks a question that assumes yes.
//
// The other way round for the other prompts, which are asking permission to
// change the robot. This one is asking permission to do the thing the command
// was already run to do, so enter means go on.
func confirmDefaultYes(question string) bool {
	fmt.Printf("%s [Y/n] ", question)

	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}

	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "" || answer == "y" || answer == "yes"
}
