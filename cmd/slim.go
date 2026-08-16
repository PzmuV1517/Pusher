package cmd

import (
	"fmt"
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/extreme"
	"github.com/andreibanu/pusher/internal/ftcproject"
	"github.com/andreibanu/pusher/internal/gradle"
	"github.com/spf13/cobra"
)

var (
	slimUndo       bool
	slimABI        string
	slimSourceMaps bool
	slimStoreLibs  bool
)

var slimCmd = &cobra.Command{
	Use:   "slim",
	Short: "Shrink the APK that gets deployed",
	Long: `Trims the robot controller APK so every deploy transfers less data.

The APK is the deploy bottleneck, not the build. A stock FTC project packages
native libraries for two CPU architectures even though the hub only ever runs
one, so roughly a third of the native code is transferred and then discarded.

Changes are written to your FTC project's gradle files, with a backup of each
file kept alongside it. Run 'pusher slim --undo' to put everything back.`,
	RunE: runSlim,
}

func init() {
	slimCmd.Flags().BoolVar(&slimUndo, "undo", false, "Restore the gradle files pusher patched")
	slimCmd.Flags().StringVar(&slimABI, "abi", "", "ABI to keep (default: ask the connected hub)")
	slimCmd.Flags().BoolVar(&slimSourceMaps, "strip-source-maps", false, "Also exclude JavaScript source maps from assets")
	slimCmd.Flags().BoolVar(&slimStoreLibs, "store-libs", false, "Store native libraries uncompressed so the install does not extract them")
}

// warnSlimUnsupported stops a command that would appear to slim and do nothing.
//
// Reporting a slimmed deploy that packaged everything is worse than refusing:
// the whole point is the size of what gets transferred, and nobody checks.
func warnSlimUnsupported(root string) error {
	reason := ftcproject.Supported(root)
	if reason == nil {
		return nil
	}

	fmt.Printf("\n[!] %v\n", reason)
	fmt.Println("    Nothing gets patched. Every deploy would go on packaging")
	fmt.Println("    every architecture, at full size, while appearing to have")
	fmt.Println("    been slimmed.")

	if ignoreWarnings {
		fmt.Println("    Carrying on anyway because --ignore-warnings was passed.")
		return nil
	}

	return fmt.Errorf("stopped, because carrying on would achieve nothing.\n" +
		"    Turn slim off in `pusher settings`, or pass --ignore-warnings")
}

func runSlim(cmd *cobra.Command, args []string) error {
	wrapper, err := gradle.DetectWrapper()
	if err != nil {
		return fmt.Errorf("failed to detect Gradle wrapper: %w", err)
	}
	if err := warnSlimUnsupported(gradle.ProjectDir(wrapper)); err != nil {
		return err
	}

	project, err := detectFTCProject()
	if err != nil {
		return err
	}
	fmt.Printf("[OK] FTC project: %s\n", project.Root)

	if slimUndo {
		// Undo restores whole files from backups, and Pusher Extreme keeps a
		// block in one of them. A backup taken before that block was added
		// would silently remove it, turning extreme off without saying so.
		wasExcluded := extreme.Excluded(project.Root)

		restored, err := project.Undo()
		if err != nil {
			return err
		}

		if wasExcluded && !extreme.Excluded(project.Root) {
			if err := extreme.Exclude(project.Root); err != nil {
				return fmt.Errorf("restoring the Pusher Extreme block failed: %w", err)
			}
			fmt.Println("[*] Kept the Pusher Extreme block; undo it from `pusher settings`")
		}
		fmt.Printf("\n[OK] Restored: %s\n", strings.Join(restored, ", "))
		fmt.Println("    Your next build will package everything again.")
		return nil
	}

	analysis, err := project.Analyze()
	if err != nil {
		return err
	}

	fmt.Printf("[*] Currently packaging ABIs: %s\n", strings.Join(analysis.ABIs, ", "))

	abi, err := resolveTargetABI(analysis.ABIs)
	if err != nil {
		return err
	}
	fmt.Printf("[*] Keeping: %s\n", abi)

	changed := false

	abiChanged, err := project.SetABI(abi)
	if err != nil {
		return err
	}
	if abiChanged {
		changed = true
		fmt.Println("\n[OK] build.common.gradle now packages one ABI")
	} else {
		fmt.Println("\n[=] ABI filters already set to that, nothing to do")
	}

	if slimSourceMaps {
		mapsChanged, err := project.StripSourceMaps()
		if err != nil {
			return err
		}
		if mapsChanged {
			changed = true
			fmt.Println("[OK] TeamCode/build.gradle now excludes *.map source maps")
		} else {
			fmt.Println("[=] Source maps already excluded")
		}
	}

	if slimStoreLibs || config.GetStoreLibs() {
		libsChanged, err := project.StoreLibs(false)
		if err != nil {
			return err
		}
		if libsChanged {
			changed = true
			fmt.Println("[OK] Native libraries are now stored, not compressed")
			fmt.Println("    The APK gets bigger; the install stops extracting them.")
		} else {
			fmt.Println("[=] Native libraries already stored")
		}
	}

	if !changed {
		return nil
	}

	fmt.Println("\nRun 'pusher' to rebuild and deploy the slimmer APK.")
	fmt.Println("Undo any time with 'pusher slim --undo'.")

	return nil
}

func rememberHubABI(serial string) string {
	abis, err := adb.ABIList(serial)
	if err != nil || len(abis) == 0 {
		return ""
	}

	if abis[0] != config.GetHubABI() {
		_ = config.SetHubABI(abis[0])
	}

	return abis[0]
}

func applyAutoSlim() {
	abi := config.GetHubABI()
	if abi == "" {
		fmt.Println("\n[!] Slim-before-push is on, but pusher has not seen your hub yet.")
		fmt.Println("    Connect the robot and run 'pusher slim' once; after that")
		fmt.Println("    every push will slim automatically.")
		return
	}

	project, err := detectFTCProject()
	if err != nil {
		fmt.Printf("\n[!] Slim-before-push skipped: %v\n", err)
		return
	}

	changed, err := project.SetABI(abi)
	if err != nil {
		fmt.Printf("\n[!] Slim-before-push failed: %v\n", err)
		return
	}

	if changed {
		fmt.Printf("\n[OK] Slimmed: packaging %s only (undo with 'pusher slim --undo')\n", abi)
	}
}

func warnOnABIMismatch(patchedFor, actual string) {
	if patchedFor == "" || actual == "" || patchedFor == actual {
		return
	}

	fmt.Printf("\n[!] This APK was built for %s but the hub runs %s.\n", patchedFor, actual)
	fmt.Println("    pusher has corrected its records; rerun 'pusher' to rebuild.")
}

func detectFTCProject() (*ftcproject.Project, error) {
	wrapper, err := gradle.DetectWrapper()
	if err != nil {
		return nil, fmt.Errorf("failed to detect Gradle wrapper: %w", err)
	}

	return ftcproject.Detect(gradle.ProjectDir(wrapper))
}

func resolveTargetABI(projectABIs []string) (string, error) {
	if slimABI != "" {
		return slimABI, nil
	}

	device, ok := adb.FindUSBDevice()
	serial := ""
	if ok {
		serial = device.Serial
		fmt.Printf("[*] Asking %s which ABI it runs...\n", device.Label())
	} else if adb.IsConnected() {
		serial = adb.RobotAddr()
		fmt.Println("[*] Asking the robot which ABI it runs...")
	} else {
		return "", fmt.Errorf("no hub connected, so pusher cannot tell which ABI to keep\n\n" +
			"Either connect to the robot first (USB or 'pusher connect'),\n" +
			"or name it explicitly, e.g. 'pusher slim --abi armeabi-v7a'")
	}

	deviceABIs, err := adb.ABIList(serial)
	if err != nil {
		return "", fmt.Errorf("failed to read the hub's ABI: %w", err)
	}
	fmt.Printf("[OK] Hub supports: %s\n", strings.Join(deviceABIs, ", "))

	if deviceABIs[0] != config.GetHubABI() {
		_ = config.SetHubABI(deviceABIs[0])
	}

	return ftcproject.PickABI(deviceABIs, projectABIs)
}
