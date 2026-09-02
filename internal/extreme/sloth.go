package extreme

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Sloth reloads team code onto the robot, which is what Pusher Extreme does,
// and the two cannot share a robot.
//
// Both install a class loader and both put a copy of team code on the SD card,
// so the robot ends up holding two copies and loading whichever it reaches
// first. Reported by a team and confirmed from their robot's own log: Sloth's
// loader picked up the dex pusher had left in the OnBotJava jars directory nine
// hours earlier, and every one of their fifteen classes failed to define,
// because the copy pusher ships carries team code only and the library those
// classes extend was not reachable from that loader. The Driver Station listed
// no OpModes at all, which reads from the driver's seat as code that will not
// download, and it survives uninstalling pusher: the dex is on the robot, not
// on the laptop.
//
// So this is a refusal rather than a warning. There is nothing to gain by
// setting it up anyway, Sloth already reloading team code faster than an
// install, and the way it fails is the way that costs a match.

// slothCoordinate is the group Sloth is published under.
//
// Matched together with the name rather than as one coordinate, because a
// version catalog splits them across fields and would otherwise slip through.
// Sinister on its own is deliberately not enough: Sloth brings it, but it is a
// scanner rather than a loader, and refusing on it would turn away projects
// that never had this problem.
var (
	slothCoordinate = regexp.MustCompile(`dev\.frozenmilk\.sinister`)
	slothName       = regexp.MustCompile(`\bSloth\b`)
	slothArchive    = regexp.MustCompile(`(?i)^sloth[-.].*\.(aar|jar)$`)
)

// UsesSloth reports whether the project has Sloth in it.
//
// Read from the build files rather than from the robot, because this has to be
// answerable at setup time, which is before anything has been deployed and
// often with no robot connected at all.
func UsesSloth(root string) bool {
	found := false

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || found {
			return nil
		}

		if info.IsDir() {
			switch info.Name() {
			case "build", ".git", ".gradle", ".idea":
				return filepath.SkipDir
			}
			return nil
		}

		if slothArchive.MatchString(info.Name()) {
			found = true
			return filepath.SkipDir
		}

		if !isBuildInput(info.Name()) || strings.HasSuffix(info.Name(), ".aar") ||
			strings.HasSuffix(info.Name(), ".jar") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if slothCoordinate.Match(content) && slothName.Match(content) {
			found = true
		}
		return nil
	})

	return found
}

// slothRefusal is what somebody sees instead of a setup.
const slothRefusal = `this project uses Sloth, which reloads team code the way Pusher Extreme does

Both put a copy of your team code on the robot and install their own class
loader for it, so the robot ends up holding two copies and loading whichever
it reaches first. On the robot this was reported from, every team class then
failed to load and the Driver Station listed no OpModes at all.

Sloth already does this job, so keep Sloth and leave Pusher Extreme off. If
you would rather use pusher, take Sloth out of your gradle file first.`
