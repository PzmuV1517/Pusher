// Package telemetry counts the devices pusher runs on.
//
// One random ID per device, the version and the platform, sent at most once a
// day. Nothing about the project, the robot, the network or the person. The
// count exists to answer "how many teams is this reaching, and which versions
// are they on", which is the difference between changing something and guessing
// at whether anyone would notice.
package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/andreibanu/pusher/internal/config"
)

// endpoint is the deployed Worker, from worker/README.md step 5.
//
// Empty disables everything: no ID is generated, no notice is printed and no
// request is made. A build with no counter configured must be a build that does
// not phone anywhere, not one that fails quietly against a dead URL.
var endpoint = "https://pusher-count.quantum-robotics-9fc.workers.dev"

const (
	// pingEvery is the most often one device is counted.
	pingEvery = 24 * time.Hour

	// requestTimeout is generous for one small POST and still short enough that
	// a captive portal cannot hold the goroutine open.
	requestTimeout = 3 * time.Second
)

// Reporter counts one run.
//
// Split in two on purpose. Everything that touches the config file happens on
// the caller's goroutine, in Start and Finish, because the settings screen
// writes that same file and viper has no idea two writers exist. The goroutine
// in between only speaks HTTP.
type Reporter struct {
	done chan struct{}
	ok   bool
}

// Start counts this device, in the background, if it is due to be counted.
//
// It never returns an error and never blocks on the network. Call Finish before
// the process exits.
func Start(version string) *Reporter {
	r := &Reporter{done: make(chan struct{})}

	if !Enabled() {
		close(r.done)
		return r
	}

	id, fresh := deviceID()
	if id == "" {
		close(r.done)
		return r
	}

	// Said once, on the run that created the ID, before anything is sent.
	if fresh {
		printNotice()
	}

	if !due(config.GetLastPing(), time.Now()) {
		close(r.done)
		return r
	}

	go func() {
		defer close(r.done)
		r.ok = send(id, version) == nil
	}()

	return r
}

// Finish waits briefly for the ping and records it if it landed.
//
// Most commands take longer than the request did, so this usually returns
// immediately. A ping that has not finished by then is abandoned, and the day
// is simply not counted: there is nothing here worth making somebody wait for.
func (r *Reporter) Finish(wait time.Duration) {
	if r == nil {
		return
	}

	select {
	case <-r.done:
	case <-time.After(wait):
		return
	}

	// Only a ping that arrived counts as one. A device that spent the day
	// offline is counted the next time it manages to reach the Worker, rather
	// than being marked done and skipped for another day.
	if r.ok {
		_ = config.SetLastPing(time.Now().Format(time.RFC3339))
	}
}

// Enabled reports whether pusher will count this device.
func Enabled() bool {
	if endpoint == "" {
		return false
	}
	if os.Getenv("PUSHER_NO_TELEMETRY") != "" {
		return false
	}
	return config.GetTelemetry()
}

// Configured reports whether this build has a counter to talk to at all.
func Configured() bool { return endpoint != "" }

// due reports whether enough time has passed since the last counted run.
//
// An unparseable or missing timestamp counts as due: the worst case is one
// extra ping, and the alternative is a device that never reports because of a
// stray character in a config file.
func due(last string, now time.Time) bool {
	if last == "" {
		return true
	}

	at, err := time.Parse(time.RFC3339, last)
	if err != nil {
		return true
	}

	// A clock that has gone backwards would otherwise park the device in a
	// future it has to wait out.
	if at.After(now) {
		return true
	}

	return now.Sub(at) >= pingEvery
}

// deviceID is this device's random ID, created on first use.
//
// Random, not derived from anything: no MAC address, no hostname, no username,
// nothing that could tie the row back to a person or survive a reinstall.
func deviceID() (id string, fresh bool) {
	if existing := config.GetDeviceID(); existing != "" {
		return existing, false
	}

	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", false
	}

	id = hex.EncodeToString(raw)
	if err := config.SetDeviceID(id); err != nil {
		return "", false
	}

	return id, true
}

type ping struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
}

func send(id, version string) error {
	body, err := json.Marshal(ping{
		ID:       id,
		Version:  version,
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/ping", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("counter returned %s", resp.Status)
	}

	return nil
}

// noticeOut is stdout everywhere but the tests, which have nothing to say.
var noticeOut io.Writer = os.Stdout

func printNotice() {
	fmt.Fprintln(noticeOut, "[*] pusher counts how many devices use it: a random ID it just made up,")
	fmt.Fprintln(noticeOut, "    the version, and your operating system. Nothing about your project,")
	fmt.Fprintln(noticeOut, "    your robot or your network, and never more than once a day.")
	fmt.Fprintln(noticeOut, "    Turn it off in `pusher settings`, or set PUSHER_NO_TELEMETRY=1.")
	fmt.Fprintln(noticeOut)
}
