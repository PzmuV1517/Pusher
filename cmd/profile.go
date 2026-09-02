package cmd

import (
	"fmt"

	"github.com/andreibanu/pusher/internal/gradle"
	"github.com/andreibanu/pusher/internal/profile"
	"github.com/spf13/cobra"
)

var (
	profileClear bool
	profileText  bool
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Show what ate the loop time on the last run",
	Long: `Samples what the robot is in the middle of while an OpMode runs and draws a
flame chart of it, so the answer to what is eating your loop time is a
measurement rather than a suspicion.

Turn it on in ` + "`pusher settings`" + ` -> Loop profiler, deploy, run an OpMode, then
run this. It costs loop time and is not for use in a match.`,
	RunE: runProfile,
}

func init() {
	profileCmd.Flags().BoolVar(&profileClear, "clear", false, "Delete the recordings from the robot")
	profileCmd.Flags().BoolVar(&profileText, "text", false, "Print the numbers instead of opening a page")
}

func runProfile(cmd *cobra.Command, args []string) error {
	serial, err := requireRobot()
	if err != nil {
		return err
	}

	if profileClear {
		if err := profile.Clear(serial); err != nil {
			return err
		}
		fmt.Println("[OK] Recordings cleared.")
		return nil
	}

	recordings, err := profile.List(serial)
	if err != nil {
		return err
	}

	if len(recordings) == 0 {
		return fmt.Errorf("no profiles on the robot%s", profileHint(serial))
	}

	report, err := profile.Read(serial, recordings[0])
	if err != nil {
		return err
	}

	if profileText {
		fmt.Printf("\n%s\n", report.Title())
		fmt.Println("─────────────────────────────────────────")
		for _, line := range report.Lines() {
			fmt.Println("  " + line)
		}
		return nil
	}

	page, err := report.Render("")
	if err != nil {
		return err
	}
	profile.Open(page)

	// A line in the terminal as well as the page. Somebody who ran this to see
	// one number should not have to look at a browser to get it.
	fmt.Printf("\n[OK] %s\n", report.Title())
	if hot := report.Hottest(1); len(hot) > 0 {
		fmt.Printf("     %s took the most, %.2fs of %.1fs\n",
			hot[0].Name, report.Seconds(hot[0].Self), report.Duration.Seconds())
	}
	fmt.Printf("     %s\n", page)

	if len(recordings) > 1 {
		fmt.Printf("     %d profiles on the robot; this is the newest.\n", len(recordings))
		fmt.Println("     `pusher settings` -> Loop profiles opens any of them.")
	}

	return nil
}

// profileHint says how to get a recording, and says it differently depending on
// whether the profiler is even installed. "Nothing here" is not useful when the
// reason is that nothing was ever going to be written.
func profileHint(serial string) string {
	wrapper, err := gradle.DetectWrapper()
	if err != nil {
		return ".\n    Turn the profiler on in `pusher settings` -> Loop profiler, then deploy."
	}

	if !profile.Installed(gradle.ProjectDir(wrapper)) {
		return ".\n    The loop profiler is not installed in this project.\n" +
			"    Turn it on in `pusher settings` -> Loop profiler, then deploy and run an OpMode."
	}

	// Installed in the project and installed on the robot are different things,
	// and the robot is the one that matters. It says so in its own log.
	if serial != "" && profile.Attached(serial) {
		return ".\n    The profiler is running on the robot, so it is waiting for a run:\n" +
			"    start an OpMode and stop it. The profile is written on stop."
	}

	return ".\n    The profiler is in this project but has not announced itself on the robot,\n" +
		"    so the robot is probably still running an APK from before you turned it on.\n" +
		"    Deploy, then run an OpMode and stop it."
}

// warnLoopProfiler says the profiler is on, every single deploy.
//
// Not once, and not only when it is turned on. Somebody turns it on during
// practice, forgets, and deploys before a match: the point of saying it here is
// that the last thing before the robot gets the code is a reminder that this
// build is slower than it should be.
func warnLoopProfiler(root string) {
	if !profile.Installed(root) {
		return
	}

	fmt.Println("\n[!] The loop profiler is installed, so this build samples the OpMode's")
	fmt.Println("    thread while every OpMode runs. Reading a thread's stack stops it for")
	fmt.Println("    as long as the walk takes, so that costs loop time.")
	fmt.Println("    Turn it off in `pusher settings` before an official match.")
}
