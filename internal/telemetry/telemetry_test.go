package telemetry

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/spf13/viper"
)

// counter is a stand-in for the Worker that remembers what it was sent.
type counter struct {
	mu     sync.Mutex
	bodies []map[string]any

	status int
	delay  time.Duration
}

func (c *counter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}

	raw, _ := io.ReadAll(r.Body)

	var body map[string]any
	_ = json.Unmarshal(raw, &body)

	c.mu.Lock()
	c.bodies = append(c.bodies, body)
	c.mu.Unlock()

	status := c.status
	if status == 0 {
		status = http.StatusNoContent
	}
	w.WriteHeader(status)
}

func (c *counter) pings() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.bodies)
}

func (c *counter) last() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.bodies) == 0 {
		return nil
	}
	return c.bodies[len(c.bodies)-1]
}

// setup gives the test its own config file and its own counter to talk to.
func setup(t *testing.T) *counter {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())
	viper.Reset()

	if err := config.Initialize(); err != nil {
		t.Fatalf("config.Initialize: %v", err)
	}

	c := &counter{}
	server := httptest.NewServer(c)

	oldEndpoint, oldNotice := endpoint, noticeOut
	endpoint = server.URL
	noticeOut = io.Discard

	t.Cleanup(func() {
		server.Close()
		endpoint, noticeOut = oldEndpoint, oldNotice
		os.Setenv("HOME", home)
		os.Unsetenv("PUSHER_NO_TELEMETRY")
		viper.Reset()
	})

	return c
}

// run is one whole invocation of pusher, start to finish.
func run(t *testing.T, version string) {
	t.Helper()
	Start(version).Finish(2 * time.Second)
}

func TestCountsTheDevice(t *testing.T) {
	c := setup(t)

	run(t, "1.2.3")

	if c.pings() != 1 {
		t.Fatalf("sent %d pings, want 1", c.pings())
	}

	body := c.last()
	if got := body["version"]; got != "1.2.3" {
		t.Errorf("version = %v, want 1.2.3", got)
	}
	if got, _ := body["platform"].(string); got == "" {
		t.Error("no platform sent")
	}
}

// The payload is the promise made to users in the notice and the README, so it
// is worth a test that fails the moment anything else is added to it.
func TestSendsNothingElse(t *testing.T) {
	c := setup(t)

	run(t, "1.2.3")

	body := c.last()
	for key := range body {
		switch key {
		case "id", "version", "platform":
		default:
			t.Errorf("sent %q, which is not one of the three fields users were promised", key)
		}
	}

	id, _ := body["id"].(string)
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(id) {
		t.Errorf("id = %q, want 32 hex characters of randomness", id)
	}
	if id == config.GetDeviceID() && id == "" {
		t.Error("no device id was stored")
	}
}

func TestCountsOncePerDay(t *testing.T) {
	c := setup(t)

	run(t, "1.2.3")
	run(t, "1.2.3")
	run(t, "1.2.3")

	if c.pings() != 1 {
		t.Errorf("sent %d pings for three runs in a row, want 1", c.pings())
	}
}

func TestCountsAgainTheNextDay(t *testing.T) {
	c := setup(t)

	run(t, "1.2.3")

	if err := config.SetLastPing(time.Now().Add(-25 * time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	run(t, "1.2.3")

	if c.pings() != 2 {
		t.Errorf("sent %d pings across two days, want 2", c.pings())
	}
}

// A device that was offline must not be marked as counted, or it disappears
// for a day every time it fails.
func TestAFailedPingIsNotRecorded(t *testing.T) {
	c := setup(t)
	c.status = http.StatusInternalServerError

	run(t, "1.2.3")

	if config.GetLastPing() != "" {
		t.Error("recorded a ping the counter rejected")
	}

	c.status = http.StatusNoContent
	run(t, "1.2.3")

	if c.pings() != 2 {
		t.Errorf("sent %d pings, want a retry on the run after the failure", c.pings())
	}
	if config.GetLastPing() == "" {
		t.Error("did not record the ping that succeeded")
	}
}

func TestTheDeviceIDIsStable(t *testing.T) {
	c := setup(t)

	run(t, "1.2.3")
	first := config.GetDeviceID()

	if err := config.SetLastPing(""); err != nil {
		t.Fatal(err)
	}
	run(t, "1.2.3")

	if got := config.GetDeviceID(); got != first {
		t.Errorf("device id changed from %q to %q, so one device counts as two", first, got)
	}
	if id, _ := c.last()["id"].(string); id != first {
		t.Errorf("sent %q but stored %q", id, first)
	}
}

func TestTheEnvironmentOptOutWins(t *testing.T) {
	c := setup(t)
	os.Setenv("PUSHER_NO_TELEMETRY", "1")

	run(t, "1.2.3")

	if c.pings() != 0 {
		t.Errorf("sent %d pings with PUSHER_NO_TELEMETRY set", c.pings())
	}
	if config.GetDeviceID() != "" {
		t.Error("generated a device id anyway, which is a record that should not exist")
	}
}

func TestTheSettingOptOutWins(t *testing.T) {
	c := setup(t)

	if err := config.SetTelemetry(false); err != nil {
		t.Fatal(err)
	}
	run(t, "1.2.3")

	if c.pings() != 0 {
		t.Errorf("sent %d pings with the setting off", c.pings())
	}
}

// An unconfigured build must be inert, not merely broken: no ID, no request,
// nothing written down.
func TestAnUnconfiguredBuildIsSilent(t *testing.T) {
	c := setup(t)
	endpoint = ""

	run(t, "1.2.3")

	if c.pings() != 0 {
		t.Errorf("sent %d pings with no endpoint configured", c.pings())
	}
	if config.GetDeviceID() != "" {
		t.Error("generated a device id with nowhere to send it")
	}
	if Enabled() || Configured() {
		t.Error("reports itself as on with no endpoint")
	}
}

// Nothing here is worth making somebody wait for.
func TestFinishGivesUpQuickly(t *testing.T) {
	c := setup(t)
	c.delay = 700 * time.Millisecond

	start := time.Now()
	Start("1.2.3").Finish(50 * time.Millisecond)

	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Errorf("held the exit for %s waiting on a slow counter", elapsed)
	}
	if config.GetLastPing() != "" {
		t.Error("recorded a ping that had not finished")
	}
}

func TestDue(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name string
		last string
		want bool
	}{
		{"never counted", "", true},
		{"counted an hour ago", now.Add(-time.Hour).Format(time.RFC3339), false},
		{"counted a day ago", now.Add(-25 * time.Hour).Format(time.RFC3339), true},
		{"counted exactly a day ago", now.Add(-24 * time.Hour).Format(time.RFC3339), true},
		{"nonsense in the config", "last tuesday", true},
		{"clock went backwards", now.Add(time.Hour).Format(time.RFC3339), true},
	} {
		if got := due(tc.last, now); got != tc.want {
			t.Errorf("%s: due = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Every command path calls Finish, including the ones that never started.
func TestFinishOnNothing(t *testing.T) {
	var r *Reporter
	r.Finish(time.Second)
}
