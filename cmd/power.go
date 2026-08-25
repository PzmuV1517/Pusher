package cmd

import (
	"fmt"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/gradle"
	"github.com/andreibanu/pusher/internal/power"
	"github.com/spf13/cobra"
)

var (
	powerClear bool
	powerText  bool
)

var powerCmd = &cobra.Command{
	Use:   "power",
	Short: "Show what drew the most current on the last run",
	Long: `Reports how much current each motor and hub drew while an OpMode ran, so
the answer to what is flattening the battery is a measurement rather than an
argument.

Turn the monitor on in ` + "`pusher settings`" + ` -> Power monitor, deploy, drive, then
run this. It costs loop time and is not for use in a match.`,
	RunE: runPower,
}

func init() {
	powerCmd.Flags().BoolVar(&powerClear, "clear", false, "Delete the recordings from the robot")
	powerCmd.Flags().BoolVar(&powerText, "text", false, "Print the numbers instead of opening a page")
}

func runPower(cmd *cobra.Command, args []string) error {
	serial, err := adb.Target()
	if err != nil {
		return err
	}

	if powerClear {
		if err := power.Clear(serial); err != nil {
			return err
		}
		fmt.Println("[OK] Recordings cleared.")
		return nil
	}

	recordings, err := power.List(serial)
	if err != nil {
		return err
	}

	if len(recordings) == 0 {
		return fmt.Errorf("no recordings on the robot%s", powerHint(serial))
	}

	report, err := power.Read(serial, recordings[0])
	if err != nil {
		return err
	}

	if powerText {
		printPowerReport(report, len(recordings))
		return nil
	}

	page, err := report.Render("")
	if err != nil {
		return err
	}
	power.Open(page)

	// A line in the terminal as well as the page. The page is the answer, but
	// somebody who ran this to check one number should not have to look at a
	// browser to get it.
	fmt.Printf("\n[OK] %s\n", report.Title())
	if motors := report.Motors(); len(motors) > 0 {
		fmt.Printf("     %s peaked highest at %.2fA\n", motors[0].Name, motors[0].Peak)
	}
	fmt.Printf("     %s\n", page)

	if len(recordings) > 1 {
		fmt.Printf("     %d recordings on the robot; this is the newest.\n", len(recordings))
		fmt.Println("     `pusher settings` -> Power readings opens any of them.")
	}

	return nil
}

// powerHint says how to get a recording, and says it differently depending on
// whether the monitor is even installed. "Nothing here" is not useful when the
// reason is that nothing was ever going to be written.
func powerHint(serial string) string {
	wrapper, err := gradle.DetectWrapper()
	if err != nil {
		return ".\n    Turn the monitor on in `pusher settings` -> Power monitor, then deploy."
	}

	if !power.Installed(gradle.ProjectDir(wrapper)) {
		return ".\n    The power monitor is not installed in this project.\n" +
			"    Turn it on in `pusher settings` -> Power monitor, then deploy and drive."
	}

	// Installed in the project and installed on the robot are different things,
	// and the robot is the one that matters. It says so in its own log.
	if serial != "" && power.Attached(serial) {
		return ".\n    The monitor is running on the robot, so it is waiting for a run:\n" +
			"    start an OpMode, drive, and stop it. The recording is written on stop."
	}

	return ".\n    The monitor is in this project but has not announced itself on the robot,\n" +
		"    so the robot is probably still running an APK from before you turned it on.\n" +
		"    Deploy, then run an OpMode and stop it."
}

func printPowerReport(r power.Report, total int) {
	fmt.Printf("\n%s\n", r.Title())
	fmt.Println("─────────────────────────────────────────")

	for _, line := range r.Lines() {
		fmt.Println("  " + line)
	}

	if total > 1 {
		fmt.Printf("  %d recordings on the robot; this is the newest. `pusher power --clear` removes them.\n", total)
	}
}

// warnPowerMonitor says the monitor is on, every single deploy.
//
// Not once, and not only when it is turned on. Somebody turns it on during
// practice, forgets, and deploys before a match: the whole point of saying it
// here is that the last thing before the robot gets the code is a reminder
// that this build is slower than it should be.
func warnPowerMonitor(root string) {
	if !power.Installed(root) {
		return
	}

	fmt.Println("\n[!] The power monitor is installed, so this build reads motor current")
	fmt.Println("    while every OpMode runs. That costs loop time: a motor's current")
	fmt.Println("    cannot be bulk read, so each reading is its own trip over the bus.")
	fmt.Println("    Turn it off in `pusher settings` before an official match.")
}
