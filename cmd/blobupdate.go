package cmd

import (
	"fmt"
	"time"

	"github.com/andreibanu/pusher/internal/updates"
)

// A deploy is the moment somebody cares that the library moved, and the moment
// they are least likely to read anything. So it is said twice: once before the
// build, where it is still cheap to stop and take it, and once at the end,
// where it is the last thing on screen rather than something scrolled past by a
// minute of Gradle output.

const (
	// blobStartWait is how long the start of a deploy may wait for GitHub.
	// Measured against the real repository: resolving the credentials takes
	// about 1.2s and the release lookup another 0.5s, so anything shorter than
	// this would miss the answer on the way in and only ever say it on the way
	// out.
	blobStartWait = 2500 * time.Millisecond

	// blobEndWait is generous because by now the answer arrived long ago, and
	// waiting costs nothing when there is nothing to wait for.
	blobEndWait = 2 * time.Second
)

// announceBlob says that the library has moved on, if it has.
func announceBlob(check *updates.BlobCheck, wait time.Duration, leadingBlank bool) bool {
	blob, newer := check.Result(wait)
	if !newer {
		return false
	}

	if leadingBlank {
		fmt.Println()
	}

	fmt.Printf("[*] blob %s is available; this project uses %s\n", blob.Latest, blob.Current)
	fmt.Println("    Update it in `pusher settings` -> blob library.")

	return true
}
