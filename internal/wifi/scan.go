package wifi

import (
	"errors"
	"os"
	"sync"
	"time"
)

// ErrScanUnsupported means this platform has no way to look for a network
// without joining it.
var ErrScanUnsupported = errors.New("looking for networks is not supported on this platform")

// What the radio will actually agree to, measured on macOS 26.
//
// A scan sweeps every channel and takes about four seconds, and asking again
// too soon fails outright with "Resource busy": under two seconds always, five
// seconds sometimes, ten seconds never. So scanGap paces every scan pusher
// makes, and there is no faster mode to fall back on when it matters more.
// Variables rather than constants so the tests can run at a pace that is not
// four seconds per scan.
var (
	scanGap = 10 * time.Second

	// Once the network has been found, keep looking only often enough that the
	// driver's list does not go stale before the join.
	seenGap = 45 * time.Second
)

// Sighting is what the watcher knows about a network.
type Sighting struct {
	// Present is whether the last finished scan found it. False also covers
	// "the scan could not say", so read Err before treating it as absent.
	Present bool

	// At is when it was last seen. Zero if it never has been.
	At time.Time

	// Scans is how many scans have finished, failures included.
	Scans int

	// Misses is how many of those looked properly and found nothing. A caller
	// that has already collected a few of these has its answer and need not
	// wait around for more.
	Misses int

	// Err is why the last scan could not answer, if it could not.
	Err error
}

// Seen reports whether the network has been found at some point.
func (s Sighting) Seen() bool { return !s.At.IsZero() }

// Answered reports whether the last scan actually settled the question.
func (s Sighting) Answered() bool { return s.Scans > 0 && s.Err == nil }

// Watcher looks for one network in the background.
//
// Joining on macOS fails with "could not find network" when the driver's list
// of what is nearby does not have it, which is the normal state for a hub that
// was switched on a moment ago. Looking repeatedly both keeps that list fresh
// and lets pusher say whether the hub is broadcasting at all, rather than
// reporting a join failure and leaving the reason to guesswork.
type Watcher struct {
	mgr  *Manager
	ssid string

	mu       sync.Mutex
	sighting Sighting

	found  chan struct{}
	sealed bool

	stop chan struct{}
	done chan struct{}
}

// Watch starts looking for a network in the background. Stop it once the answer
// stops mattering: every scan costs the connection a few seconds of throughput.
func (m *Manager) Watch(ssid string) *Watcher {
	w := &Watcher{
		mgr:   m,
		ssid:  ssid,
		found: make(chan struct{}),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}

	if ssid == "" || !ScanningEnabled() {
		w.sighting.Err = ErrScanUnsupported
		close(w.done)
		return w
	}

	go w.run()
	return w
}

// ScanningEnabled reports whether pusher will look for networks at all.
//
// The escape hatch is for a machine where scanning upsets the connection;
// without it the only way out would be to stop using pusher's Wi-Fi handling.
func ScanningEnabled() bool {
	if os.Getenv("PUSHER_NO_SCAN") != "" {
		return false
	}
	return scanSupported
}

// The pace is set inside Visible, so the loop only decides how long to rest on
// top of it: nothing while the network is still missing, a while once it isn't.
func (w *Watcher) run() {
	defer close(w.done)

	for {
		if !w.once() {
			return
		}

		rest := time.Duration(0)
		if w.Last().Present {
			rest = seenGap
		}

		select {
		case <-w.stop:
			return
		case <-time.After(rest):
		}
	}
}

// once records one scan, and reports whether the watcher should carry on.
func (w *Watcher) once() bool {
	present, err := w.mgr.scan(w.ssid, w.stop)
	if errors.Is(err, errScanStopped) {
		return false
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.sighting.Scans++
	w.sighting.Err = err
	w.sighting.Present = present && err == nil

	switch {
	case w.sighting.Present:
		w.sighting.At = time.Now()
		if !w.sealed {
			w.sealed = true
			close(w.found)
		}
	case err == nil:
		w.sighting.Misses++
	}

	return true
}

// Last is what the most recent finished scan found.
func (w *Watcher) Last() Sighting {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.sighting
}

// WaitFor blocks until the network turns up, and reports whether it did.
//
// The waiting is all it does: the scanning happens on the watcher's own
// schedule, because two scans at once are two scans the radio rejects. False
// means the deadline passed, or that this platform cannot look at all.
func (w *Watcher) WaitFor(timeout time.Duration) bool {
	if w.Last().Present {
		return true
	}

	select {
	case <-w.found:
		return true
	case <-w.done:
		return w.Last().Present
	case <-time.After(timeout):
		return false
	}
}

// Stop ends the background scanning, and waits for any scan still in flight.
//
// Waiting is the point as much as stopping: a scan holds the radio across every
// channel, and a join attempted underneath one is a join that can fail for a
// reason that has nothing to do with the hub.
func (w *Watcher) Stop() {
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
	<-w.done
}

// One radio, one scan at a time, and not too often: overlapping or hurried
// scans are rejected, which would read as "the hub is not there".
var (
	scanMu  sync.Mutex
	scanEnd time.Time
)

var errScanStopped = errors.New("scan cancelled")

// Visible reports whether a network is broadcasting within range.
//
// It blocks for as long as the radio needs, which is seconds. The scan is
// filtered by name inside the OS, which is why this works on a macOS that will
// not tell pusher the name of the network it is already on: the names come back
// hidden, but the match is made before they are hidden.
func (m *Manager) Visible(ssid string) (bool, error) {
	return m.scan(ssid, nil)
}

// A nil stop channel never fires, so an uninterruptible caller waits it out.
func (m *Manager) scan(ssid string, stop <-chan struct{}) (bool, error) {
	if ssid == "" {
		return false, nil
	}
	if !ScanningEnabled() {
		return false, ErrScanUnsupported
	}

	scanMu.Lock()
	defer scanMu.Unlock()

	if rest := scanGap - time.Since(scanEnd); !scanEnd.IsZero() && rest > 0 {
		timer := time.NewTimer(rest)
		defer timer.Stop()

		select {
		case <-stop:
			return false, errScanStopped
		case <-timer.C:
		}
	}

	present, err := scanNow(m, ssid)
	scanEnd = time.Now()

	return present, err
}

// The one seam the tests need: everything above it is platform-independent, and
// everything below it needs a radio.
var scanNow = func(m *Manager, ssid string) (bool, error) {
	return m.visible(ssid)
}
