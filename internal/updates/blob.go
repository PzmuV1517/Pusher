package updates

import (
	"time"

	"github.com/andreibanu/pusher/internal/blobdep"
	"github.com/andreibanu/pusher/internal/blobrel"
	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/ghauth"
)

// Blob is what the project uses and what has been published since.
type Blob struct {
	Current string
	Latest  string

	// Branch is the work these came from, which is worth saying when it is not
	// main: an update from a branch is not the same news as a stable release.
	Branch string
}

// Newer reports whether there is something to update to.
//
// Both versions are release tags rather than numbers, so this asks whether they
// differ rather than which is larger: a project pinned to an older tag on
// purpose is still worth telling once, and pusher does not get to decide that
// v1.4.0 is better than whatever somebody chose.
func (b Blob) Newer() bool {
	return b.Current != "" && b.Latest != "" && b.Current != b.Latest
}

// BlobCheck is one background look for a newer blob.
type BlobCheck struct {
	done   chan struct{}
	result Blob
}

// WatchBlob looks for a newer blob release for the project at root.
//
// Started at the top of a deploy so it runs alongside the build rather than in
// front of it. A project that does not use blob, or a machine with no access to
// the repository, does nothing and says nothing.
func WatchBlob(root string) *BlobCheck {
	c := &BlobCheck{done: make(chan struct{})}

	// All of it in the goroutine, resolving the GitHub credentials included:
	// that alone takes over a second, and doing it here would put the wait in
	// front of the deploy rather than beside it.
	go func() {
		defer close(c.done)

		dep, err := blobdep.Detect(root)
		if err != nil || dep == nil || dep.Version == "" {
			return
		}
		c.result.Current = dep.Version

		status, creds := ghauth.Resolve()
		if !status.OK() {
			return
		}

		// The branch somebody chose, not whatever is newest overall. Following a
		// branch and being told about main would be an update that undoes the
		// choice rather than one that follows it.
		branch := config.GetBlobBranch()
		c.result.Branch = branch

		if tag, err := blobrel.LatestOn(creds.Secret(), branch); err == nil {
			c.result.Latest = tag
		}
	}()

	return c
}

// Result waits briefly for the answer, and reports whether there is one worth
// saying out loud.
//
// Safe to call more than once, which is the point: a deploy says this at the
// start and again at the end, and the second call is free because the answer
// arrived long ago.
func (c *BlobCheck) Result(wait time.Duration) (Blob, bool) {
	if c == nil {
		return Blob{}, false
	}

	select {
	case <-c.done:
	case <-time.After(wait):
		return Blob{}, false
	}

	return c.result, c.result.Newer()
}
