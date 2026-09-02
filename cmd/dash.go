package cmd

import (
	"fmt"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/dash"
	"github.com/andreibanu/pusher/internal/gradle"
	"github.com/spf13/cobra"
)

var dashNoSave bool

var dashCmd = &cobra.Command{
	Use:   "dash",
	Short: "Compare the robot's tuning against your code",
	Long: `Reads what the robot's dashboard is currently holding and compares it
with the @Config or @Configurable fields your source declares.

Tuning lives on the robot until it is written into the source, and the next
deploy puts the code's values back. This says what you would lose.`,
}

var dashDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show what the robot holds that your code does not",
	Long: `Reads the robot's dashboard over its WebSocket and compares every value
with the initialiser its field has in your source.

Works with FtcDashboard and with Panels. A team runs one or the other, and
pusher asks both rather than making you say which.

Values that differ are tuning you have not written down yet. Values that agree
now but differed the last time this ran are tuning you have already saved.
Telling those apart needs a previous reading, so each run records one.

Works over USB, by forwarding a port, and over Wi-Fi.`,
	RunE: runDashDiff,
}

func init() {
	dashDiffCmd.Flags().BoolVar(&dashNoSave, "no-save", false, "Do not record this reading for the next comparison")
	dashCmd.AddCommand(dashDiffCmd)
}

func runDashDiff(cmd *cobra.Command, args []string) error {
	serial, err := requireRobot()
	if err != nil {
		return err
	}

	root, err := gradle.DetectWrapper()
	if err != nil {
		return fmt.Errorf("run this from your FTC project: %w", err)
	}
	project := gradle.ProjectDir(root)

	fmt.Printf("[*] Reading the dashboard on %s\n", serial)

	live, from, err := dash.Read(serial)
	if err != nil {
		return err
	}

	code := dash.FromProject(project)
	fmt.Printf("[*] %d tunables on the robot per %s, %d declared in %s\n",
		len(live), from, len(code), project)

	path := dash.SnapshotPath(config.Dir(), serial)
	previous, taken := dash.Load(path)

	result := dash.Compare(live, code, previous)
	result.Snapshot = taken

	fmt.Print(result.Report())

	if previous == nil {
		fmt.Println("\n    First reading for this robot, so nothing can be reported as")
		fmt.Println("    already saved yet. Run this again after you tune something.")
	} else if !taken.IsZero() {
		fmt.Printf("\n    Compared against the reading from %s.\n", taken.Format("2 Jan 15:04"))
	}

	if !dashNoSave {
		if err := dash.Save(path, live); err != nil {
			fmt.Printf("\n[!] Could not record this reading: %v\n", err)
		}
	}

	return nil
}
