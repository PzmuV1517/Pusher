package cmd

import (
	"fmt"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/dash"
)

// A deploy puts the code's values back, whether it installs an APK or reloads
// team code. Anything tuned in the dashboard and not written down is gone, and
// there is nothing afterwards to say what it was.
//
// So the reading is taken before the deploy and reported after it, which is the
// only order in which the answer still exists.

// dashWatch is a reading taken before a deploy.
type dashWatch struct {
	serial   string
	live     dash.Values
	previous dash.Values
}

// beginDashWatch reads the dashboard before a deploy, when that is turned on.
//
// Failure is silent by design. The dashboard not running is ordinary, and a
// deploy must not fail because of a report about it.
func beginDashWatch(serial string) *dashWatch {
	if !config.GetDashWatch() || serial == "" {
		return nil
	}

	live, _, err := dash.Read(serial)
	if err != nil || len(live) == 0 {
		return nil
	}

	previous, _ := dash.Load(dash.SnapshotPath(config.Dir(), serial))

	return &dashWatch{serial: serial, live: live, previous: previous}
}

// report says what the deploy just overwrote.
func (w *dashWatch) report(projectRoot string) {
	if w == nil {
		return
	}

	code := dash.FromProject(projectRoot)
	result := dash.Compare(w.live, code, w.previous)

	if result.Any() {
		fmt.Print(result.Report())
	} else if result.Untouched > 0 {
		fmt.Printf("\n[=] Dashboard tuning: nothing to save, %d values matched your code.\n",
			result.Untouched)
	}

	// Recorded after the report, so the next deploy can tell what moved since
	// this one rather than comparing against something older.
	_ = dash.Save(dash.SnapshotPath(config.Dir(), w.serial), w.live)
}
