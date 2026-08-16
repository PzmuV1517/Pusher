package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/wifi"
)

// apWaitTimeout is how long pusher keeps looking for the hub before joining
// anyway. A scan takes about four seconds and the radio refuses another for ten,
// so this is room for three or four attempts.
const apWaitTimeout = 45 * time.Second

// enoughMisses is how many clean scans that found nothing settle the matter
// without waiting for more.
const enoughMisses = 3

// joinRobot joins the hub's network, having first made sure it is there.
//
// It takes over the watcher the caller started before the build: by now the hub
// is in the driver's list of what is nearby, which is the list the join
// consults, and that is what stops a join failing on a hub that is right there.
func joinRobot(mgr *wifi.Manager, watcher *wifi.Watcher, profile *config.Profile) (string, error) {
	seen := awaitRobotAP(watcher, profile.SSID)

	// Nothing else may touch the radio from here: a scan running underneath the
	// join is a join that fails for a reason that is not the hub's fault.
	if watcher != nil {
		watcher.Stop()
	}

	ip, err := mgr.JoinAndWait(profile.SSID, profile.Password, wifi.RobotSubnet, joinTimeout)
	if err == nil {
		return ip, nil
	}

	// The hub is there and the join still failed, which on macOS is usually the
	// driver having looked at the wrong moment. One more attempt is cheap, and
	// it is the difference between a deploy and a confusing error.
	if seen {
		fmt.Println("[*] The hub is broadcasting but the join did not take. Trying once more...")

		ip, retryErr := mgr.JoinAndWait(profile.SSID, profile.Password, wifi.RobotSubnet, joinTimeout)
		if retryErr == nil {
			return ip, nil
		}
		err = retryErr
	}

	return "", joinFailure(seen, err)
}

// awaitRobotAP waits for the hub to show up, and reports whether it did.
//
// It never blocks the deploy on its own answer. A scan can miss a network that
// is really there, so "I did not see it" is worth saying out loud and not worth
// refusing on: the join is the thing that actually knows.
func awaitRobotAP(watcher *wifi.Watcher, ssid string) bool {
	if watcher == nil {
		return false
	}

	last := watcher.Last()

	// A build long enough to hold several clean scans has already answered the
	// question, so waiting the timeout out again would be forty-five seconds
	// spent confirming what pusher was told three times over.
	if !last.Present && last.Misses < enoughMisses {
		if wifi.ScanningEnabled() {
			fmt.Printf("[*] Looking for %s...\n", ssid)
		}

		if watcher.WaitFor(apWaitTimeout) {
			return true
		}
		last = watcher.Last()
	}

	if last.Present {
		return true
	}

	switch {
	case errors.Is(last.Err, wifi.ErrScanUnsupported):

	// A scan that looked properly and found nothing outranks one the radio
	// refused afterwards: the refusal says nothing, the empty sky says plenty.
	case last.Misses > 0:
		fmt.Printf("[!] %s is not broadcasting.\n", ssid)
		fmt.Println("    Trying anyway, but check the hub is powered on and nearby.")

	case last.Err != nil:
		fmt.Printf("[!] Could not check whether %s is broadcasting: %v\n", ssid, last.Err)
	}

	return false
}

// joinFailure explains a failed join with what the scanning already established.
//
// The error it wraps already names the network and says what macOS said, so
// this only adds the half macOS cannot know.
func joinFailure(seen bool, err error) error {
	if seen {
		return fmt.Errorf("%w\n"+
			"    The hub is broadcasting, so it is not switched off or out of range.\n"+
			"    Check the password in `pusher settings`", err)
	}

	if !wifi.ScanningEnabled() {
		return err
	}

	return fmt.Errorf("%w\n"+
		"    pusher never saw that network while it was looking, so the hub is\n"+
		"    probably switched off, still starting up, or out of range", err)
}
