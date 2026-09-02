package cmd

import (
	"fmt"
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/feature"
	"github.com/andreibanu/pusher/internal/pathtrace"
	"github.com/andreibanu/pusher/internal/tui"
	"github.com/andreibanu/pusher/internal/visual"
	"github.com/spf13/cobra"
)

var (
	visFile     string
	visOut      string
	visNoOpen   bool
	visProject  string
	visTopSpeed float64
	visAccel    float64
	visDecel    float64
	visLatAccel float64
)

var visualiseCmd = &cobra.Command{
	Use:     "visualiser [OpMode class name]",
	Aliases: []string{"visualizer", "vis"},
	Short:   "Draw the path an autonomous actually drove, coloured by speed",
	Long: `Renders a blob path trace as an HTML page: the whole flow of the auto, every
curve coloured by modelled speed, and a duration estimate next to the measured
time.

Traces are recorded by the blob library when BlobParams.recordTrace is on, which
requires the blob-dev build. Competition builds cannot record at all. Manage the
build from ` + "`pusher settings` -> blob library" + `.

  pusher visualiser              pick from the runs on the robot
  pusher visualiser CloseBlue    newest run for that OpMode
  pusher visualiser --file t.json  a trace you already have`,
	RunE: runVisualise,
}

func runVisualise(cmd *cobra.Command, args []string) error {

	if !feature.Revealed() {
		return fmt.Errorf("unknown command %q for %q", "visualiser", "pusher")
	}

	if status, _ := feature.Authorized(); !status.OK() {
		return fmt.Errorf("the visualiser needs read access to the blob repository.\n" +
			"Set a GitHub token in `pusher settings` -> blob library -> GitHub token")
	}

	limits := pathtrace.DefaultLimits()
	if visTopSpeed > 0 {
		limits.TopSpeed = visTopSpeed
	}
	if visAccel > 0 {
		limits.Accel = visAccel
	}
	if visDecel > 0 {
		limits.Decel = visDecel
	}
	if visLatAccel > 0 {
		limits.LatAccel = visLatAccel
	}

	if visFile != "" {
		return render(func() (string, error) {
			return visual.RenderLocal(visFile, visProject, visOut, limits)
		})
	}

	if len(args) == 0 && visOut == "" && !visNoOpen {
		return tui.RunTracePicker(visProject, limits)
	}

	serial, err := requireRobot()
	if err != nil {
		return err
	}

	_, traces, err := visual.ListOn(serial)
	if err != nil {
		return err
	}

	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	hits := adb.MatchTraces(traces, name)
	if len(hits) == 0 {
		return fmt.Errorf("no trace for %q on the robot\navailable: %s",
			name, strings.Join(adb.OpModeNames(traces), ", "))
	}

	return render(func() (string, error) {
		return visual.Render(serial, hits[0], visProject, visOut, limits)
	})
}

func render(run func() (string, error)) error {
	out, err := run()
	if err != nil {
		return err
	}

	fmt.Println(out)
	if !visNoOpen {
		visual.Open(out)
	}
	return nil
}

func init() {
	visualiseCmd.Flags().StringVar(&visFile, "file", "", "Render a local trace instead of pulling one")
	visualiseCmd.Flags().StringVarP(&visOut, "out", "o", "", "Where to write the HTML")
	visualiseCmd.Flags().BoolVar(&visNoOpen, "no-open", false, "Do not open the result in a browser")
	visualiseCmd.Flags().StringVar(&visProject, "project", "", "Project root used to map segments to source lines")
	visualiseCmd.Flags().Float64Var(&visTopSpeed, "top-speed", 0, "Drivetrain top speed at full power, in/s")
	visualiseCmd.Flags().Float64Var(&visAccel, "accel", 0, "Acceleration limit, in/s^2")
	visualiseCmd.Flags().Float64Var(&visDecel, "decel", 0, "Deceleration limit, in/s^2")
	visualiseCmd.Flags().Float64Var(&visLatAccel, "lat-accel", 0, "Lateral grip limit, in/s^2")
}
