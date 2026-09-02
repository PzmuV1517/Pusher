// Package robot attaches the laptop to the hub: joining its network, and
// getting adb onto it once there.
//
// This lives outside cmd because both halves of pusher have to be able to offer
// it. A command can print and ask a question; a menu can do neither while
// bubbletea owns the screen, so everything here reports to a writer the caller
// chooses rather than to stdout.
package robot

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/wifi"
)

// ErrNoProfile means there is no robot network on file, so there is nothing to
// join. Separate from a failed join: one is answered in settings, the other by
// switching the hub on.
var ErrNoProfile = errors.New("no robot profile configured")

// joinTimeout is how long a join is given before it counts as failed.
const joinTimeout = 45 * time.Second

// apWaitTimeout is how long pusher keeps looking for the hub before joining
// anyway. A scan takes about four seconds and the radio refuses another for ten,
// so this is room for three or four attempts.
const apWaitTimeout = 45 * time.Second

// enoughMisses is how many clean scans that found nothing settle the matter
// without waiting for more.
const enoughMisses = 3

// Connect gets from wherever the laptop is to talking to the robot.
//
// USB first, the way deploying decides it, then the network. Being on the robot
// network already is common enough to be worth not undoing: somebody who ran
// `pusher connect` an hour ago and let adb time out does not need their Wi-Fi
// touched again.
func Connect(out io.Writer) error {
	if !adb.IsInstalled() {
		return fmt.Errorf("%w - please install Android SDK Platform-Tools", adb.ErrNoADB)
	}

	if device, ok := adb.FindUSBDevice(); ok {
		fmt.Fprintf(out, "[OK] Hub already attached over USB: %s\n", device.Label())
		return nil
	}

	// Before anything touches the radio. A robot that is already reachable is
	// one nobody has to leave their own network for, which is the whole point.
	if config.GetRelay() {
		if found, err := Locate(out, false); err == nil {
			Remember(found)
			fmt.Fprintf(out, "[OK] Robot at %s, on the network you were already on\n", found.Addr)
			return nil
		}
	}

	manager := wifi.NewManager()

	onRobot, err := manager.IsOnRobotNetwork()
	if err != nil {
		return fmt.Errorf("failed to check the current network: %w", err)
	}

	if onRobot {
		fmt.Fprintln(out, "[OK] Already on the robot network")
	} else if err := join(out, manager); err != nil {
		return err
	}

	fmt.Fprintln(out, "\n[+] Connecting to robot via ADB...")
	if err := adb.ConnectTo(out); err != nil {
		return fmt.Errorf("failed to connect via ADB: %w", err)
	}

	fmt.Fprintln(out, "[OK] Connected via ADB")
	return nil
}

// join moves the laptop onto the robot's network.
func join(out io.Writer, manager *wifi.Manager) error {
	profile, err := config.GetDefaultProfile()
	if err != nil {
		return ErrNoProfile
	}

	// Started before the questions below, so the scan overlaps them rather than
	// following them.
	watcher := manager.Watch(profile.SSID)
	defer watcher.Stop()

	ssid, ssidErr := manager.CurrentSSID()
	switch {
	case ssidErr == nil && ssid != "":
		fmt.Fprintf(out, "[OK] Currently on: %s\n", ssid)
	case errors.Is(ssidErr, wifi.ErrSSIDUnavailable):
		if inferred, err := manager.MostRecentNetwork(SSIDs()...); err == nil && inferred != "" {
			fmt.Fprintf(out, "[*] The network name is hidden; assuming you are on %q\n", inferred)
		}
	}

	fmt.Fprintf(out, "\n[>] Joining robot Wi-Fi: %s\n", profile.SSID)

	ip, err := Join(out, manager, watcher, profile)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "[OK] On the robot network (%s)\n", ip)
	return nil
}

// Possible reports whether connecting is something pusher could attempt from
// here, so a caller knows whether an offer is worth making.
//
// Only adb being missing says no. Having no profile is not that case: somebody
// already sitting on the robot's network needs no profile to reach it.
func Possible() bool {
	return adb.IsInstalled()
}

// Network is the robot network pusher would join, or empty when it has none on
// file and would be relying on the laptop already being on the hub's.
//
// For prompts, which read better naming the network they are about to take the
// laptop off its own onto.
func Network() string {
	profile, err := config.GetDefaultProfile()
	if err != nil {
		return ""
	}
	return profile.SSID
}

// SSIDs is every robot network pusher has been told about, which is the set a
// network worth leaving is drawn from.
func SSIDs() []string {
	cfg, err := config.Load()
	if err != nil {
		return nil
	}

	ssids := make([]string, 0, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		if profile != nil && profile.SSID != "" {
			ssids = append(ssids, profile.SSID)
		}
	}

	return ssids
}
