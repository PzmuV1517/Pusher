package extreme

import (
	"fmt"
	"time"

	"github.com/andreibanu/pusher/internal/dash"
)

// Pushing classes and having them registered are different things, and until
// now pusher only knew about the first. A reload that delivered every file and
// registered none of them printed the same success as one that worked.
//
// So it asks. FtcDashboard reports the robot's own OpMode list, straight out of
// RegisteredOpModes, which is what the Driver Station shows. Comparing that
// against what the team declared turns a silent disappearance into a line of
// output naming what went missing.

const (
	// settleWait is how long the robot is given between attempts. It picks the
	// reload up on its next event loop tick, which is fast, but registration
	// itself is not instant.
	settleWait = 750 * time.Millisecond

	// verifyAttempts is how many times the list is re-read while OpModes are
	// still appearing, so a slow robot is not reported as a broken one.
	verifyAttempts = 3
)

// Verify reports what the team declared and which of it the robot did not end
// up with.
//
// A nil error with nothing missing is the good case. An error means the
// question could not be asked at all, which is not a reload failure and must
// not be presented as one: a project without FtcDashboard is entitled to
// deploy.
//
// The first attempt is made immediately, so a robot with no dashboard on it
// costs a failed connection rather than a wait. Only a robot that answered and
// came up short is asked again, because that is the one case where waiting
// might change the answer.
func Verify(root, serial string) (declared, missing []OpMode, err error) {
	declared = DeclaredOpModes(root)
	if len(declared) == 0 {
		return nil, nil, nil
	}

	for attempt := 0; attempt < verifyAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(settleWait)
		}

		onRobot, err := dash.Registered(serial)
		if err != nil {
			return declared, nil, err
		}

		missing = MissingFrom(declared, dash.Names(onRobot))
		if len(missing) == 0 {
			return declared, nil, nil
		}
	}

	return declared, missing, nil
}

// verified turns the answer into the line a deploy prints, or nothing at all.
func verified(root, serial string) (step, warning string) {
	declared, missing, err := Verify(root, serial)
	if len(declared) == 0 {
		return "", ""
	}
	if err != nil {
		// Silence rather than noise. The dashboard is not required, and a
		// deploy that worked should not end with a paragraph about a library
		// the team chose not to use.
		return "", ""
	}

	if len(missing) == 0 {
		return fmt.Sprintf("the robot registered all %d OpModes", len(declared)), ""
	}

	return "", fmt.Sprintf("the robot registered %d of %d OpModes. Missing: %s\n"+
		"    They compiled and reached the robot, so this is registration rather\n"+
		"    than delivery. Usually the app never rescanned: restart the robot app,\n"+
		"    since the SDK attaches its reload watch when it starts. If they are\n"+
		"    still missing after that, the robot turned them down and said why:\n"+
		"    `pusher dev` -> Collect the robot's own logs.",
		len(declared)-len(missing), len(declared), Summary(missing, 5))
}
