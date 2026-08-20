package updates

import (
	"testing"
	"time"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/spf13/viper"
)

func settings(t *testing.T) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())
	viper.Reset()

	if err := config.Initialize(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(viper.Reset)
}

func TestDue(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name string
		last string
		want bool
	}{
		{"never checked", "", true},
		{"checked an hour ago", now.Add(-time.Hour).Format(time.RFC3339), false},
		{"checked a day ago", now.Add(-25 * time.Hour).Format(time.RFC3339), true},
		{"checked exactly a day ago", now.Add(-24 * time.Hour).Format(time.RFC3339), true},
		{"nonsense in the config", "whenever", true},
		{"clock went backwards", now.Add(time.Hour).Format(time.RFC3339), true},
	} {
		if got := due(tc.last, now); got != tc.want {
			t.Errorf("%s: due = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Nobody wants the same version announced every morning until they install it.
func TestAVersionIsAnnouncedOnce(t *testing.T) {
	settings(t)

	if err := config.SetNotifiedVersion("1.3.0"); err != nil {
		t.Fatal(err)
	}

	c := &Check{done: make(chan struct{}), ran: true, Found: "1.3.0"}
	close(c.done)
	c.Finish(time.Second)

	if c.Notified {
		t.Error("announced a version somebody has already been told about")
	}
}

// Checking at all is what the setting turns off, not just the announcing: a
// machine that does not want to be told should not be asking GitHub either.
func TestTheSettingStopsTheCheck(t *testing.T) {
	settings(t)

	if err := config.SetUpdateNotify(false); err != nil {
		t.Fatal(err)
	}
	if Enabled() {
		t.Fatal("still enabled with the setting off")
	}

	c := Watch()
	c.Finish(time.Second)

	if c.Found != "" {
		t.Error("looked anyway")
	}
	if config.GetLastUpdateCheck() != "" {
		t.Error("recorded a check that never happened")
	}
}

// A check that found nothing still counts as a check, or an up-to-date machine
// asks GitHub on every single command.
func TestAnUneventfulCheckIsStillRecorded(t *testing.T) {
	settings(t)

	c := &Check{done: make(chan struct{}), ran: true}
	close(c.done)
	c.Finish(time.Second)

	if config.GetLastUpdateCheck() == "" {
		t.Error("a check that found nothing was not recorded")
	}
}

func TestFinishOnNothing(t *testing.T) {
	var c *Check
	c.Finish(time.Second)

	var b *BlobCheck
	if _, newer := b.Result(time.Second); newer {
		t.Error("a check that never ran reported an update")
	}
}

// Same tag, nothing to say. Different tag, worth one line.
func TestBlobNewerNeedsBothVersions(t *testing.T) {
	for _, tc := range []struct {
		blob Blob
		want bool
	}{
		{Blob{Current: "v1.4.0", Latest: "v1.5.0"}, true},
		{Blob{Current: "v1.5.0", Latest: "v1.5.0"}, false},
		{Blob{Current: "v1.4.0"}, false},
		{Blob{Latest: "v1.5.0"}, false},
		{Blob{}, false},
	} {
		if got := tc.blob.Newer(); got != tc.want {
			t.Errorf("%+v.Newer() = %v, want %v", tc.blob, got, tc.want)
		}
	}
}

// A project with no blob in it must not hold up a deploy waiting for GitHub.
func TestABlobCheckOnAProjectWithoutBlobIsInstant(t *testing.T) {
	settings(t)

	start := time.Now()
	c := WatchBlob(t.TempDir())

	if _, newer := c.Result(2 * time.Second); newer {
		t.Error("reported an update for a project that does not use blob")
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Errorf("took %s to decide there was nothing to check", elapsed)
	}
}

// A skipped check must not move the clock. Writing the time down whether or not
// a look happened means the next run is not due either, and the one after that,
// so pusher checks once on a fresh install and never again.
func TestASkippedCheckDoesNotMoveTheClock(t *testing.T) {
	settings(t)

	yesterday := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	if err := config.SetLastUpdateCheck(yesterday); err != nil {
		t.Fatal(err)
	}

	// Two hours ago is not due, so this one does nothing.
	Watch().Finish(time.Second)

	if got := config.GetLastUpdateCheck(); got != yesterday {
		t.Errorf("last check moved from %q to %q without a check happening", yesterday, got)
	}
}
