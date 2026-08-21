package blobrel

import "testing"

// The tag is the only place the branch is recorded, so reading it is the whole
// of the contract between blob's release workflow and this.
func TestBranchComesOutOfTheTag(t *testing.T) {
	for _, tc := range []struct{ tag, want string }{
		{"v1.7.0", MainBranch},
		{"v1.8.0-feedforward.1", "feedforward"},
		{"v1.8.0-feedforward.12", "feedforward"},
		{"v1.8.0-RSTController.1", "RSTController"},

		// A label with no build number is still a label.
		{"v1.8.0-rc", "rc"},

		// Nothing after the hyphen is not a label.
		{"v1.8.0-", MainBranch},
		{"", MainBranch},
	} {
		if got := Branch(tc.tag); got != tc.want {
			t.Errorf("Branch(%q) = %q, want %q", tc.tag, got, tc.want)
		}
	}
}

func releases(tags ...string) []Release {
	out := make([]Release, 0, len(tags))
	for _, tag := range tags {
		out = append(out, Release{Tag: tag, Branch: Branch(tag), Prerelease: Branch(tag) != MainBranch})
	}
	return out
}

func TestBranchesAreListedMainFirst(t *testing.T) {
	// Newest first, as GitHub returns them, with branch work on top.
	got := branchesIn(releases(
		"v1.8.0-feedforward.2",
		"v1.8.0-RSTController.1",
		"v1.8.0-feedforward.1",
		"v1.7.0",
		"v1.6.0",
	))

	want := []string{MainBranch, "feedforward", "RSTController"}

	if len(got) != len(want) {
		t.Fatalf("branches = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("branch %d = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}

func TestBranchesWithNoStableReleaseYet(t *testing.T) {
	got := branchesIn(releases("v1.8.0-feedforward.1"))

	if len(got) != 1 || got[0] != "feedforward" {
		t.Errorf("branches = %v, want just the branch that has one", got)
	}
}

// The newest release on a branch is what switching to it lands on, and a stable
// tag must never be mistaken for branch work or the other way round.
func TestLatestOnPicksTheNewestFromThatBranchOnly(t *testing.T) {
	all := releases(
		"v1.9.0-feedforward.2",
		"v1.9.0-feedforward.1",
		"v1.7.0",
		"v1.6.0-RSTController.3",
	)

	for _, tc := range []struct{ branch, want string }{
		{MainBranch, "v1.7.0"},
		{"feedforward", "v1.9.0-feedforward.2"},
		{"RSTController", "v1.6.0-RSTController.3"},
		{"", "v1.7.0"},
	} {
		got, err := latestOn(all, tc.branch)
		if err != nil {
			t.Errorf("%s: %v", tc.branch, err)
			continue
		}
		if got != tc.want {
			t.Errorf("latest on %q = %q, want %q", tc.branch, got, tc.want)
		}
	}

	if _, err := latestOn(all, "nothing-here"); err == nil {
		t.Error("a branch with no releases was reported as having one")
	}
}
