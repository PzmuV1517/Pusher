// Package updates asks, in the background, whether something newer exists.
//
// Nothing here can fail a command. A check that cannot reach GitHub, an
// announcement that cannot be delivered and a robot with no network are all the
// same outcome: the deploy goes ahead and nobody is told anything.
package updates

import (
	"time"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/notify"
	"github.com/andreibanu/pusher/internal/selfupdate"
)

// checkEvery is how often pusher asks whether it is out of date. Often enough
// to hear about a release the week it happens, rarely enough that GitHub is not
// asked on every deploy.
const checkEvery = 24 * time.Hour

// Check is one background look for a newer pusher.
type Check struct {
	done chan struct{}

	// ran records that a look actually happened. Without it, a check that was
	// skipped still wrote down the time, so the next run was not due either and
	// the window slid forward forever: pusher would have checked once and never
	// again.
	ran bool

	// Found is the newer release, empty when there is not one. Read after the
	// check has finished.
	Found string

	// Notified records that the desktop was told, so the terminal need not
	// repeat it.
	Notified bool
}

// Watch looks for a newer pusher, unless it has looked recently.
//
// The config is read here and written in Finish, both on the caller's
// goroutine, because the settings screen writes that same file and viper has no
// idea two writers exist. In between, this only speaks HTTP.
func Watch() *Check {
	c := &Check{done: make(chan struct{})}

	if !Enabled() || !due(config.GetLastUpdateCheck(), time.Now()) {
		close(c.done)
		return c
	}

	c.ran = true

	go func() {
		defer close(c.done)

		release, err := selfupdate.Latest()
		if err != nil || !release.Newer() {
			return
		}
		c.Found = release.Version()
	}()

	return c
}

// Enabled reports whether pusher will look for newer versions.
func Enabled() bool {
	return config.GetUpdateNotify()
}

// Finish waits briefly for the answer and announces it, once per version.
//
// Announced rather than printed: the point of checking in the background is to
// reach somebody who has stopped reading the terminal, and a line that scrolls
// past the end of a deploy is a line nobody sees. The same version is never
// announced twice, so leaving it uninstalled costs one notification, not one a
// day forever.
func (c *Check) Finish(wait time.Duration) {
	if c == nil || !c.ran {
		return
	}

	select {
	case <-c.done:
	case <-time.After(wait):
		return
	}

	// Recorded whatever the answer, including "already the newest", so a
	// machine that is up to date does not ask GitHub on every command.
	_ = config.SetLastUpdateCheck(time.Now().Format(time.RFC3339))

	if c.Found == "" || c.Found == config.GetNotifiedVersion() {
		return
	}

	c.Notified = notify.Send(notify.Title,
		"Pusher "+c.Found+" is available. Run `pusher update` to install it.")

	if c.Notified {
		_ = config.SetNotifiedVersion(c.Found)
	}
}

// due reports whether enough time has passed since the last look.
//
// Anything unreadable counts as due: the cost of being wrong is one HTTP
// request, and the alternative is a machine that never checks again because of
// a stray character in a config file.
func due(last string, now time.Time) bool {
	if last == "" {
		return true
	}

	at, err := time.Parse(time.RFC3339, last)
	if err != nil {
		return true
	}
	if at.After(now) {
		return true
	}

	return now.Sub(at) >= checkEvery
}
