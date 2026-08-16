package wifi

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeRadio replaces the one call that needs hardware, and speeds the pacing up
// so a test that exercises several scans finishes in milliseconds.
func fakeRadio(t *testing.T, answer func(n int) (bool, error)) *int32Counter {
	t.Helper()

	counter := &int32Counter{}

	oldScan, oldGap, oldSeen := scanNow, scanGap, seenGap
	scanNow = func(_ *Manager, _ string) (bool, error) {
		return answer(counter.next())
	}
	scanGap = time.Millisecond
	seenGap = time.Millisecond

	scanMu.Lock()
	scanEnd = time.Time{}
	scanMu.Unlock()

	t.Cleanup(func() {
		scanNow, scanGap, seenGap = oldScan, oldGap, oldSeen

		scanMu.Lock()
		scanEnd = time.Time{}
		scanMu.Unlock()
	})

	return counter
}

type int32Counter struct {
	mu sync.Mutex
	n  int
}

func (c *int32Counter) next() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	return c.n
}

func (c *int32Counter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func TestWatchFindsNetwork(t *testing.T) {
	fakeRadio(t, func(int) (bool, error) { return true, nil })

	w := NewManager().Watch("14270-RC")
	defer w.Stop()

	if !w.WaitFor(2 * time.Second) {
		t.Fatalf("never saw a network that is always there: %+v", w.Last())
	}
	if last := w.Last(); !last.Seen() || !last.Present {
		t.Errorf("sighting says it was not found: %+v", last)
	}
}

func TestWatchMissingNetwork(t *testing.T) {
	fakeRadio(t, func(int) (bool, error) { return false, nil })

	w := NewManager().Watch("14270-RC")
	defer w.Stop()

	if w.WaitFor(50 * time.Millisecond) {
		t.Fatal("claimed to find a network that is not there")
	}

	last := w.Last()
	if last.Seen() {
		t.Errorf("recorded a sighting that never happened: %+v", last)
	}
	if !last.Answered() {
		t.Errorf("a scan that found nothing still answered the question: %+v", last)
	}
}

// The distinction that matters most: a scan the radio refused is not a network
// that is absent. Reading it as absent would send everyone hunting for a hub
// that was switched on the whole time.
func TestBusyRadioIsNotAnAnswer(t *testing.T) {
	busy := errors.New("Resource busy")
	fakeRadio(t, func(int) (bool, error) { return false, busy })

	w := NewManager().Watch("14270-RC")
	defer w.Stop()

	waitForScans(t, w, 1)

	last := w.Last()
	if last.Present {
		t.Error("a failed scan reported the network as present")
	}
	if last.Answered() {
		t.Errorf("a failed scan claimed to have answered: %+v", last)
	}
	if !errors.Is(last.Err, busy) {
		t.Errorf("lost the reason the scan failed: %+v", last)
	}
}

// Misses is what lets a caller stop waiting, so a refused scan must never
// count as one: three of those would end the wait having looked at nothing.
func TestMissesCountOnlyCleanScans(t *testing.T) {
	busy := errors.New("Resource busy")
	fakeRadio(t, func(n int) (bool, error) {
		if n%2 == 0 {
			return false, busy
		}
		return false, nil
	})

	w := NewManager().Watch("14270-RC")
	defer w.Stop()

	waitForScans(t, w, 6)

	// One snapshot, so the two counts are from the same moment: every odd scan
	// looked properly and found nothing, and every even one was refused.
	last := w.Last()

	if want := (last.Scans + 1) / 2; last.Misses != want {
		t.Errorf("counted %d misses in %d scans, want %d: refused scans are being "+
			"counted as an empty sky", last.Misses, last.Scans, want)
	}
}

// A hub that is switched on halfway through the build still gets found.
func TestWatchKeepsLooking(t *testing.T) {
	fakeRadio(t, func(n int) (bool, error) { return n >= 3, nil })

	w := NewManager().Watch("14270-RC")
	defer w.Stop()

	if !w.WaitFor(2 * time.Second) {
		t.Fatalf("gave up before the network appeared: %+v", w.Last())
	}
	if last := w.Last(); last.Scans < 3 {
		t.Errorf("found it in %d scans, which is fewer than it took", last.Scans)
	}
}

// Once found, the watcher must stop hammering the radio: a scan costs the
// connection several seconds of throughput, and Gradle is using it.
func TestWatchBacksOffOnceFound(t *testing.T) {
	oldSeen := seenGap
	counter := fakeRadio(t, func(int) (bool, error) { return true, nil })
	seenGap = time.Hour
	t.Cleanup(func() { seenGap = oldSeen })

	w := NewManager().Watch("14270-RC")
	defer w.Stop()

	w.WaitFor(2 * time.Second)
	time.Sleep(50 * time.Millisecond)

	if n := counter.count(); n != 1 {
		t.Errorf("scanned %d times after already finding it, want 1", n)
	}
}

func TestStopWaitsForTheScanInFlight(t *testing.T) {
	running := make(chan struct{})
	release := make(chan struct{})
	var started, freed sync.Once

	fakeRadio(t, func(int) (bool, error) {
		started.Do(func() { close(running) })
		<-release
		return false, nil
	})

	// Freed on the way out too, and registered after the fake so it is undone
	// first: a scan left blocked holds the one lock every other test needs, so a
	// failure here would hang the package rather than report itself.
	t.Cleanup(func() { freed.Do(func() { close(release) }) })

	w := NewManager().Watch("14270-RC")

	<-running
	stopped := make(chan struct{})
	go func() {
		w.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		t.Fatal("Stop returned while a scan still had the radio")
	case <-time.After(50 * time.Millisecond):
	}

	freed.Do(func() { close(release) })

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop never returned")
	}
}

func TestStopIsRepeatable(t *testing.T) {
	fakeRadio(t, func(int) (bool, error) { return true, nil })

	w := NewManager().Watch("14270-RC")
	w.Stop()
	w.Stop()
}

// Both callers defer Stop and one of them also stops early, so a watcher that
// never ran must survive the same treatment.
func TestWatchWithoutAnSSID(t *testing.T) {
	fakeRadio(t, func(int) (bool, error) { return true, nil })

	w := NewManager().Watch("")
	defer w.Stop()

	if w.WaitFor(50 * time.Millisecond) {
		t.Error("claimed to find a network with no name")
	}
	if !errors.Is(w.Last().Err, ErrScanUnsupported) {
		t.Errorf("want ErrScanUnsupported, got %+v", w.Last())
	}
	w.Stop()
}

func TestScansArePaced(t *testing.T) {
	counter := fakeRadio(t, func(int) (bool, error) { return false, nil })
	scanGap = 80 * time.Millisecond

	mgr := NewManager()
	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := mgr.Visible("14270-RC"); err != nil {
			t.Fatalf("scan %d: %v", i, err)
		}
	}

	if elapsed := time.Since(start); elapsed < 160*time.Millisecond {
		t.Errorf("three scans took %s, which is faster than the radio allows", elapsed)
	}
	if counter.count() != 3 {
		t.Errorf("ran %d scans, want 3", counter.count())
	}
}

func waitForScans(t *testing.T, w *Watcher, want int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for w.Last().Scans < want {
		if time.Now().After(deadline) {
			t.Fatalf("only %d scans finished, want %d", w.Last().Scans, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestNmcliSawSSID(t *testing.T) {
	const output = "ICHB-Robotics\n\n14270-RC\nSSID\\:with\\:colons\n"

	for _, tc := range []struct {
		ssid string
		want bool
	}{
		{"14270-RC", true},
		{"ICHB-Robotics", true},
		{"SSID:with:colons", true},
		{"14270", false},
		{"14270-RC-2", false},
		{"", false},
	} {
		if got := nmcliSawSSID(output, tc.ssid); got != tc.want {
			t.Errorf("nmcliSawSSID(%q) = %v, want %v", tc.ssid, got, tc.want)
		}
	}
}

func TestNetshSawSSID(t *testing.T) {
	const output = "Interface name : Wi-Fi \r\nThere are 3 networks currently visible.\r\n" +
		"\r\nSSID 1 : ICHB-Robotics\r\n    Network type            : Infrastructure\r\n" +
		"    Authentication          : WPA2-Personal\r\n" +
		"\r\nSSID 2 : 14270-RC\r\n    Network type            : Infrastructure\r\n" +
		"\r\nSSID 3 : \r\n"

	for _, tc := range []struct {
		ssid string
		want bool
	}{
		{"14270-RC", true},
		{"ICHB-Robotics", true},
		{"Infrastructure", false},
		{"Wi-Fi", false},
		{"14270", false},
		{"", false},
	} {
		if got := netshSawSSID(output, tc.ssid); got != tc.want {
			t.Errorf("netshSawSSID(%q) = %v, want %v", tc.ssid, got, tc.want)
		}
	}
}
