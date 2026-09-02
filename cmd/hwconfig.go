package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/andreibanu/pusher/internal/gradle"
	"github.com/andreibanu/pusher/internal/robotcfg"
	"github.com/andreibanu/pusher/internal/tui"
	"github.com/spf13/cobra"
)

var (
	hwDir      string
	hwForce    bool
	hwNoBackup bool
	hwRestart  bool
	hwYes      bool
	hwRaw      bool
)

var hwconfigCmd = &cobra.Command{
	Use:     "hwconfig",
	Aliases: []string{"hw"},
	Short:   "Pull, edit and push the robot's hardware configuration",
	Long: `Keeps the robot's hardware configurations in your project so they can be read,
edited and versioned alongside the code that names the devices.

A configuration is one XML file in ` + robotcfg.HubDir + ` on the robot. The Driver
Station writes it, and nothing on the robot cares where a file came from, so one
edited on a laptop and copied back behaves exactly like one made on the Driver
Station.

Run on its own it opens a menu covering all of it, with an editor that knows
what a port is: pick a device type from the ones that exist, land on a free
port, and see a clash the moment it is typed.

  pusher hwconfig                open the menu
  pusher hwconfig list           print what the robot and the project have
  pusher hwconfig pull           copy every configuration into the project
  pusher hwconfig view comp      show what is wired where
  pusher hwconfig push comp      copy it back to the robot

Files move byte for byte in both directions. Pusher parses them to check and
describe them, and rewrites one only when you edit it - and then in the same
format the Driver Station uses, so the diff is the change and nothing else.`,
	RunE: runHWMenu,
}

var hwListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "Print what the robot and the project have",
	RunE:    runHWList,
}

var hwPullCmd = &cobra.Command{
	Use:   "pull [name...]",
	Short: "Copy configurations off the robot into the project",
	Long: `Copies configurations from the robot into the project.

With no names, everything on the robot is pulled.`,
	RunE: runHWPull,
}

var hwPushCmd = &cobra.Command{
	Use:   "push [name...]",
	Short: "Copy configurations from the project to the robot",
	Long: `Copies configurations from the project to the robot.

With no names, every configuration in the project is pushed.

Each file is checked first, and anything the robot controller would reject
stops the push. Use --force to push anyway.

The robot's own copy of anything about to be overwritten is saved into the
project first, because it may have been changed on the Driver Station since it
was pulled.`,
	RunE: runHWPush,
}

var hwViewCmd = &cobra.Command{
	Use:   "view <name>",
	Args:  cobra.ExactArgs(1),
	Short: "Show what a configuration wires where",
	RunE:  runHWView,
}

var hwEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Args:  cobra.ExactArgs(1),
	Short: "Open a configuration in your editor, then check it",
	Long: `Opens a configuration in $EDITOR, checks it when you save, and offers to push
it to the robot.

If the configuration is not in the project yet it is pulled from the robot
first.`,
	RunE: runHWEdit,
}

var hwDiffCmd = &cobra.Command{
	Use:   "diff [name...]",
	Short: "Compare the project's configurations with the robot's",
	Long: `Compares configurations in the project with the ones on the robot, in terms of
devices rather than lines: a file that was reformatted but wires the same things
to the same ports reports no change.`,
	RunE: runHWDiff,
}

var hwCheckCmd = &cobra.Command{
	Use:   "check [name...]",
	Short: "Check configurations for what the robot would reject",
	RunE:  runHWCheck,
}

var hwRemoveCmd = &cobra.Command{
	Use:     "rm <name>",
	Aliases: []string{"remove", "delete"},
	Args:    cobra.ExactArgs(1),
	Short:   "Delete a configuration from the robot",
	RunE:    runHWRemove,
}

func init() {
	hwconfigCmd.PersistentFlags().StringVar(&hwDir, "dir", "",
		"Where configurations live (default: configs/ in the FTC project)")

	hwPushCmd.Flags().BoolVar(&hwForce, "force", false, "Push even if a configuration has errors")
	hwPushCmd.Flags().BoolVar(&hwNoBackup, "no-backup", false, "Do not save the robot's copy before overwriting it")
	hwPushCmd.Flags().BoolVar(&hwRestart, "restart", false, "Restart the robot controller afterwards, so the list is rebuilt")
	hwEditCmd.Flags().BoolVar(&hwYes, "yes", false, "Push when the edit checks out, without asking")
	hwRemoveCmd.Flags().BoolVarP(&hwYes, "yes", "y", false, "Delete without asking")
	hwViewCmd.Flags().BoolVar(&hwRaw, "raw", false, "Print the file instead of a summary")

	hwconfigCmd.AddCommand(hwListCmd, hwPullCmd, hwPushCmd, hwViewCmd, hwEditCmd,
		hwDiffCmd, hwCheckCmd, hwRemoveCmd)
}

func runHWMenu(cmd *cobra.Command, args []string) error {
	local, err := store()
	if err != nil {
		return err
	}

	return tui.RunHWConfig(local.Dir)
}

func store() (*robotcfg.Store, error) {
	if hwDir != "" {
		return robotcfg.NewStore(hwDir), nil
	}

	wrapper, err := gradle.DetectWrapper()
	if err != nil {
		return nil, fmt.Errorf("pusher cannot tell where to keep configurations: %w\n\n"+
			"Run this from your FTC project, or name a directory with --dir", err)
	}

	return robotcfg.NewStore(robotcfg.LocalDir(gradle.ProjectDir(wrapper))), nil
}

func runHWList(cmd *cobra.Command, args []string) error {
	local, err := store()
	if err != nil {
		return err
	}

	localNames, err := local.Names()
	if err != nil {
		return err
	}

	var (
		robotNames []string
		hashes     map[string]string
		active     string
		serial     string
	)
	if serial, err = requireRobot(); err == nil {
		robotNames, err = robotcfg.List(serial)
		if err != nil {
			fmt.Printf("[!] Could not read the robot's configurations: %v\n\n", err)
		}
		hashes = robotcfg.Hashes(serial)
		active = robotcfg.ActiveConfig(serial)
	} else {
		fmt.Printf("[*] No robot connected, showing only what the project has.\n")
		fmt.Printf("    (%v)\n\n", err)
	}

	if len(localNames) == 0 && len(robotNames) == 0 {
		fmt.Printf("[=] Nothing in %s and nothing on the robot.\n", local.Dir)
		if serial != "" {
			fmt.Println("    Make one on the Driver Station, then 'pusher hwconfig pull'.")
		}
		return nil
	}

	fmt.Printf("Project: %s\n\n", local.Dir)

	if serial == "" {
		for _, name := range localNames {
			fmt.Printf("  %s\n", name)
		}
		return nil
	}

	fmt.Printf("  %-32s %-24s %s\n", "CONFIGURATION", "WHERE", "STATUS")

	onRobot := set(robotNames)
	inProject := set(localNames)

	for _, name := range merged(localNames, robotNames) {
		where, status := "", ""

		switch {
		case !onRobot[name]:
			where, status = "project only", "not on the robot"
		case !inProject[name]:
			where, status = "robot only", "not pulled"
		default:
			where, status = "project + robot", compare(local, hashes, name)
		}

		if name == active {
			where += " (active)"
		}

		fmt.Printf("  %-32s %-24s %s\n", name, where, status)
	}

	fmt.Println()
	if active == "" && len(robotNames) > 0 {
		fmt.Println("[*] Pusher could not read which configuration is selected.")
		fmt.Println("    That needs privileged adb, which a phone robot controller does not give.")
	}

	return nil
}

func compare(local *robotcfg.Store, hashes map[string]string, name string) string {
	theirs, known := hashes[name]
	if !known {
		return "yes"
	}

	mine, err := local.Read(name)
	if err != nil {
		return "yes"
	}

	if robotcfg.Hash(mine) == theirs {
		return "same"
	}
	return "differs"
}

func runHWPull(cmd *cobra.Command, args []string) error {
	local, err := store()
	if err != nil {
		return err
	}

	serial, err := requireRobot()
	if err != nil {
		return err
	}

	names, err := robotcfg.List(serial)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("the robot has no configurations in %s", robotcfg.HubDir)
	}

	wanted, err := pick(args, names, "on the robot")
	if err != nil {
		return err
	}

	for _, name := range wanted {
		data, err := robotcfg.Fetch(serial, name)
		if err != nil {
			return err
		}

		if local.Has(name) {
			if existing, err := local.Read(name); err == nil && robotcfg.Same(existing, data) {
				fmt.Printf("[=] %s (unchanged)\n", name)
				continue
			}
		}

		if err := local.Write(name, data); err != nil {
			return err
		}
		fmt.Printf("[OK] %s -> %s\n", name, local.Path(name))

		report(name, data)
	}

	fmt.Printf("\n[*] Commit %s to keep the wiring with the code that uses it.\n", local.Dir)
	return nil
}

func runHWPush(cmd *cobra.Command, args []string) error {
	local, err := store()
	if err != nil {
		return err
	}

	names, err := local.Names()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("nothing in %s to push\n\nRun 'pusher hwconfig pull' first", local.Dir)
	}

	wanted, err := pick(args, names, "in "+local.Dir)
	if err != nil {
		return err
	}

	files := make(map[string][]byte, len(wanted))
	blocked := false

	for _, name := range wanted {
		data, err := local.Read(name)
		if err != nil {
			return err
		}
		files[name] = data

		if !report(name, data) && !hwForce {
			blocked = true
		}
	}

	if blocked {
		return fmt.Errorf("nothing was pushed: fix the errors above, or use --force to push anyway")
	}

	serial, err := requireRobot()
	if err != nil {
		return err
	}

	active := robotcfg.ActiveConfig(serial)
	replacedActive := false

	for _, name := range wanted {
		if !hwNoBackup {
			if current, err := robotcfg.Fetch(serial, name); err == nil && !robotcfg.Same(current, files[name]) {
				path, err := local.Backup(name, current)
				if err != nil {
					return err
				}
				fmt.Printf("[*] The robot's %s differed; saved it to %s\n", name, path)
			}
		}

		if err := robotcfg.Send(serial, name, files[name]); err != nil {
			return err
		}
		fmt.Printf("[OK] %s -> robot\n", name)

		if name == active {
			replacedActive = true
		}
	}

	fmt.Println()

	// Every file was read back off the robot before getting here, so it is
	// there and it is right. What is left is whether anything asked the robot
	// again, which is a question about the Driver Station rather than the file.
	if hwRestart {
		pkg := robotcfg.ControllerPackage(serial)
		if pkg == "" {
			fmt.Println("[!] Could not find the robot controller app to restart.")
		} else if err := robotcfg.Restart(serial, pkg); err != nil {
			fmt.Printf("[!] Could not restart the robot controller: %v\n", err)
		} else {
			fmt.Printf("[OK] Restarted %s, so it has rescanned the directory.\n\n", pkg)
		}
	}

	if replacedActive {
		fmt.Printf("[!] %q is the configuration the robot is running.\n", active)
		fmt.Println("    It keeps the old wiring until you re-select it:")
		fmt.Println("    Driver Station -> Configure Robot -> pick it -> Activate.")
	} else {
		fmt.Println("[*] Select it on the Driver Station to use it:")
		fmt.Println("    Configure Robot -> pick it -> Activate.")
	}

	if !hwRestart {
		fmt.Println()
		fmt.Println("[*] Not in the list? The Driver Station shows the list it was given when")
		fmt.Println("    it last asked, so one that appeared underneath it is not there yet.")
		fmt.Println("    Leave the config screen and open it again, or run this with --restart")
		fmt.Println("    to restart the robot controller and settle it.")
	}

	return nil
}

func runHWView(cmd *cobra.Command, args []string) error {
	name := args[0]

	data, source, err := readAnywhere(name)
	if err != nil {
		return err
	}

	if hwRaw {
		fmt.Print(string(data))
		return nil
	}

	cfg, err := robotcfg.Parse(data)
	if err != nil {
		return fmt.Errorf("%s (%s): %w", name, source, err)
	}

	fmt.Printf("%s  (%s)\n\n", name, source)
	fmt.Print(robotcfg.Summary(cfg))

	names := cfg.Names()
	sort.Strings(names)
	fmt.Printf("\n%d device(s) an OpMode can look up: %s\n", len(names), strings.Join(names, ", "))

	printIssues(robotcfg.Validate(cfg))
	return nil
}

func runHWEdit(cmd *cobra.Command, args []string) error {
	name := args[0]

	local, err := store()
	if err != nil {
		return err
	}

	if !local.Has(name) {
		serial, err := requireRobot()
		if err != nil {
			return fmt.Errorf("%q is not in %s, and pusher cannot reach the robot to fetch it: %w",
				name, local.Dir, err)
		}

		data, err := robotcfg.Fetch(serial, name)
		if err != nil {
			return err
		}
		if err := local.Write(name, data); err != nil {
			return err
		}
		fmt.Printf("[OK] Pulled %s from the robot\n", name)
	}

	before, err := local.Read(name)
	if err != nil {
		return err
	}

	if err := openEditor(local.Path(name)); err != nil {
		return err
	}

	after, err := local.Read(name)
	if err != nil {
		return err
	}

	if robotcfg.Same(before, after) {
		fmt.Println("[=] Unchanged.")
		return nil
	}

	oldCfg, oldErr := robotcfg.Parse(before)
	newCfg, newErr := robotcfg.Parse(after)

	if newErr != nil {
		fmt.Printf("\n[!] %s no longer parses: %v\n", name, newErr)
		fmt.Printf("    The file is still at %s. Nothing was sent to the robot.\n", local.Path(name))
		return fmt.Errorf("%s is broken", name)
	}

	if oldErr == nil {
		if changes := robotcfg.Diff(oldCfg, newCfg); len(changes) > 0 {
			fmt.Println("\nChanged:")
			for _, change := range changes {
				fmt.Printf("  %s\n", change)
			}
		}
	}

	issues := robotcfg.Validate(newCfg)
	printIssues(issues)

	if issues.Errors() {
		fmt.Printf("\n[!] Not pushing %s. Fix the errors, or push it yourself with --force.\n", name)
		return fmt.Errorf("%s has errors", name)
	}

	if !hwYes && !confirm(fmt.Sprintf("\nPush %s to the robot?", name)) {
		fmt.Printf("[*] Left in %s. Push it later with 'pusher hwconfig push %s'.\n", local.Dir, name)
		return nil
	}

	return runHWPush(cmd, []string{name})
}

func runHWDiff(cmd *cobra.Command, args []string) error {
	local, err := store()
	if err != nil {
		return err
	}

	serial, err := requireRobot()
	if err != nil {
		return err
	}

	names, err := local.Names()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("nothing in %s to compare", local.Dir)
	}

	wanted, err := pick(args, names, "in "+local.Dir)
	if err != nil {
		return err
	}

	for _, name := range wanted {
		mine, err := local.Read(name)
		if err != nil {
			return err
		}

		theirs, err := robotcfg.Fetch(serial, name)
		if err != nil {
			fmt.Printf("[!] %s is not on the robot\n", name)
			continue
		}

		if robotcfg.Same(mine, theirs) {
			fmt.Printf("[=] %s is identical\n", name)
			continue
		}

		robotCfg, err := robotcfg.Parse(theirs)
		if err != nil {
			fmt.Printf("[!] the robot's %s does not parse: %v\n", name, err)
			continue
		}
		myCfg, err := robotcfg.Parse(mine)
		if err != nil {
			fmt.Printf("[!] %s does not parse: %v\n", name, err)
			continue
		}

		changes := robotcfg.Diff(robotCfg, myCfg)
		if len(changes) == 0 {
			fmt.Printf("[=] %s wires the same things; only the file differs\n", name)
			continue
		}

		fmt.Printf("[!] %s (project vs robot)\n", name)
		for _, change := range changes {
			fmt.Printf("      %s\n", change)
		}
	}

	return nil
}

func runHWCheck(cmd *cobra.Command, args []string) error {
	local, err := store()
	if err != nil {
		return err
	}

	names, err := local.Names()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("nothing in %s to check\n\nRun 'pusher hwconfig pull' first", local.Dir)
	}

	wanted, err := pick(args, names, "in "+local.Dir)
	if err != nil {
		return err
	}

	bad := 0
	for _, name := range wanted {
		data, err := local.Read(name)
		if err != nil {
			return err
		}
		if !report(name, data) {
			bad++
		}
	}

	if bad > 0 {
		return fmt.Errorf("%d configuration(s) the robot would reject", bad)
	}

	fmt.Printf("\n[OK] %d configuration(s) checked.\n", len(wanted))
	return nil
}

func runHWRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	serial, err := requireRobot()
	if err != nil {
		return err
	}

	if !robotcfg.Exists(serial, name) {
		return fmt.Errorf("the robot has no configuration called %q", name)
	}

	if name == robotcfg.ActiveConfig(serial) {
		fmt.Printf("[!] %q is the configuration the robot is running.\n", name)
	}

	if local, err := store(); err == nil {
		if data, err := robotcfg.Fetch(serial, name); err == nil {
			if path, err := local.Backup(name, data); err == nil {
				fmt.Printf("[*] Saved the robot's copy to %s\n", path)
			}
		}
	}

	if !hwYes && !confirm(fmt.Sprintf("Delete %q from the robot?", name)) {
		fmt.Println("[*] Left alone.")
		return nil
	}

	if err := robotcfg.Remove(serial, name); err != nil {
		return err
	}

	fmt.Printf("[OK] Deleted %s from the robot\n", name)
	return nil
}

func readAnywhere(name string) ([]byte, string, error) {
	if local, err := store(); err == nil && local.Has(name) {
		data, err := local.Read(name)
		return data, local.Path(name), err
	}

	serial, err := requireRobot()
	if err != nil {
		return nil, "", fmt.Errorf("%q is not in the project, and pusher cannot reach the robot: %w", name, err)
	}

	data, err := robotcfg.Fetch(serial, name)
	return data, "on the robot", err
}

func report(name string, data []byte) bool {
	cfg, err := robotcfg.Parse(data)
	if err != nil {
		fmt.Printf("\n[X] %s: %v\n", name, err)
		return false
	}

	issues := robotcfg.Validate(cfg)
	if len(issues) == 0 {
		return true
	}

	fmt.Printf("\n%s:\n", name)
	printIssues(issues)

	return !issues.Errors()
}

func printIssues(issues robotcfg.Issues) {
	for _, issue := range issues {
		marker := "[!]"
		if issue.Level == robotcfg.Error {
			marker = "[X]"
		}
		fmt.Printf("  %s %s\n", marker, issue)
	}
}

func pick(args, available []string, where string) ([]string, error) {
	if len(args) == 0 {
		return available, nil
	}

	have := set(available)
	for _, name := range args {
		if !have[name] {
			return nil, fmt.Errorf("no configuration called %q %s\n\navailable: %s",
				name, where, strings.Join(available, ", "))
		}
	}

	return args, nil
}

func set(names []string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, name := range names {
		m[name] = true
	}
	return m
}

func merged(a, b []string) []string {
	seen := map[string]bool{}
	var all []string

	for _, list := range [][]string{a, b} {
		for _, name := range list {
			if !seen[name] {
				seen[name] = true
				all = append(all, name)
			}
		}
	}

	sort.Strings(all)
	return all
}

func openEditor(path string) error {
	editor := firstSet("VISUAL", "EDITOR")
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "vi"
		}
	}

	parts := strings.Fields(editor)
	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %s failed: %w\n\nEdit %s yourself, then run 'pusher hwconfig push %s'",
			parts[0], err, path, strings.TrimSuffix(filepath.Base(path), robotcfg.Ext))
	}

	return nil
}

func firstSet(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func confirm(question string) bool {
	fmt.Printf("%s [y/N] ", question)

	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}

	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}
