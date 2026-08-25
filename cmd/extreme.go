package cmd

import (
	"fmt"
	"time"

	"github.com/andreibanu/pusher/internal/config"
	"github.com/andreibanu/pusher/internal/extreme"
	"github.com/andreibanu/pusher/internal/gradle"
)

// tryExtreme replaces the install with a reload when that is genuinely
// equivalent, and says why when it is not.
//
// The dangerous outcome here is not failing. It is reloading when the robot
// needed an install: everything reports success and the robot runs stale code,
// which at a competition is discovered by the robot doing last week's
// autonomous. So every doubt falls back to installing.
func tryExtreme(gradlePath, serial, apkPath string) (bool, error) {
	if !config.GetExtreme() {
		return false, nil
	}

	project, err := extreme.FindProject()
	if err != nil {
		return false, nil
	}

	// Asked before the signature, because this is the case the signature cannot
	// see: a library swapped in a menu with no robot connected leaves the robot
	// agreeing with a signature taken before the swap.
	if config.GetForceInstall() {
		fmt.Println("\n[*] Pusher Extreme is on, but installing this time: the library changed, which a reload cannot carry")
		return false, nil
	}

	state := extreme.Status(project.Root, serial, apkPath)
	if !state.Usable() {
		fmt.Printf("\n[*] Pusher Extreme is on, but installing this time: %s\n", state.Reason)
		return false, nil
	}

	fmt.Println("\n[>] Pusher Extreme: reloading team code, not installing")

	classpath, err := extreme.ResolveClasspath(project.Wrapper, extreme.Module)
	if err != nil {
		fmt.Printf("[!] Could not work out what to compile against: %v\n", err)
		fmt.Println("[*] Installing instead.")
		return false, nil
	}

	result, err := extreme.Reload(project, serial, classpath, extreme.Kept(project.Root))
	for _, step := range result.Steps {
		fmt.Printf("    %s\n", step)
	}
	if err != nil {
		// A failed reload leaves the robot with whatever it had, which may now
		// be a directory the SDK cannot read. Installing puts it back to a
		// state that certainly works.
		fmt.Printf("\n[!] Reload failed: %v\n", err)
		fmt.Println("[*] Falling back to a full install.")
		return false, nil
	}

	for _, warning := range result.Warnings {
		fmt.Printf("[!] %s\n", warning)
	}

	fmt.Printf("\n[OK] Reloaded %d classes in %.1fs, without installing\n",
		result.Classes, result.Total.Seconds())

	return true, nil
}

// apkCarriesTeamCode reports whether a build still packages the team's classes.
//
// Deliberately not a question about settings. The exclusion lives in the
// module's gradle file, so the file is what the build obeys, and the two
// disagree the moment somebody turns Pusher Extreme off without undoing the
// setup. Asking the setting instead produced a deploy that installed an APK
// with nothing in it and then went home.
func apkCarriesTeamCode(root string) bool {
	return !extreme.Excluded(root)
}

// recordExtremeState notes what the robot now holds, after an install that
// went through the ordinary path. Without it the next deploy cannot tell
// whether anything outside team code changed and installs again.
func recordExtremeState(serial string) {
	project, err := extreme.FindProject()
	if err != nil {
		return
	}

	// Off and on again regenerates an identical block, so an install that
	// packaged team code has to take the signature away rather than let the
	// robot keep agreeing with it. It would otherwise reload classes the APK
	// already has, and the SDK then registers no OpModes at all.
	//
	// Whether the APK packaged team code is a question about the gradle file,
	// which is why the setting is not consulted here either.
	if apkCarriesTeamCode(project.Root) {
		extreme.ForgetSignature(serial)
		return
	}

	if signature, err := extreme.Signature(project.Root); err == nil {
		extreme.RecordSignature(serial, signature)
	}

	// The install has happened, so whatever needed one has been carried.
	if config.GetForceInstall() {
		_ = config.SetForceInstall(false)
	}
}

// reloadAfterInstall puts team code onto the robot once an APK that no longer
// carries it has been installed.
//
// An install on its own leaves the robot with no OpModes at all: they are
// excluded from the APK, and the reload that supplies them has not happened.
// Reporting a successful deploy in that state is how somebody gets to a match
// with an empty OpMode list.
//
// The gradle block decides this, not the setting. They disagree whenever the
// setting is turned off without undoing the setup, and it is the file the build
// obeys: the APK comes out empty of team code either way. Gating this on the
// setting instead meant those deploys installed an empty APK and stopped, while
// the robot went on serving the dex left behind by the last reload. Everything
// written before that reload still appeared, so the robot looked fine, and
// anything written since was simply absent.
func reloadAfterInstall(serial string) error {
	project, err := extreme.FindProject()
	if err != nil || apkCarriesTeamCode(project.Root) {
		return nil
	}

	if !config.GetExtreme() {
		fmt.Println("\n[!] Pusher Extreme is turned off, but this project is still set up for it.")
		fmt.Println("    The APK that was just installed carries no team code, so it has to be")
		fmt.Println("    reloaded anyway. Undo the setup in `pusher settings` for ordinary APKs.")
	}

	stranded := func(err error) error {
		return fmt.Errorf("the APK is installed but carries no team code, so the robot "+
			"has no OpModes: %w\n"+
			"    Run `pusher` again, or undo the setup in `pusher settings`", err)
	}

	fmt.Println("\n[>] Pusher Extreme: that APK has no team code in it, reloading it now")

	classpath, err := extreme.ResolveClasspath(project.Wrapper, extreme.Module)
	if err != nil {
		return stranded(err)
	}

	result, err := extreme.Reload(project, serial, classpath, extreme.Kept(project.Root))
	for _, step := range result.Steps {
		fmt.Printf("    %s\n", step)
	}
	if err != nil {
		return stranded(err)
	}

	for _, warning := range result.Warnings {
		fmt.Printf("[!] %s\n", warning)
	}

	fmt.Printf("[OK] Reloaded %d classes, so the robot has its OpModes\n", result.Classes)
	return nil
}

// extremeDeploy is the deploy path when Pusher Extreme is set up.
//
// The APK is still built, because it is the only way to know whether anything
// outside team code changed, and with team code excluded that build has almost
// nothing to do.
func extremeDeploy(gradlePath, serial string) (bool, error) {
	apkPath, err := gradle.FindApk(gradle.ProjectDir(gradlePath))
	if err != nil {
		return false, nil
	}

	start := time.Now()

	done, err := tryExtreme(gradlePath, serial, apkPath)
	if err != nil || !done {
		return done, err
	}

	fmt.Printf("[OK] Deployed in %.1fs\n", time.Since(start).Seconds())
	return true, nil
}

// extremeReady reports whether the robot is set up for reloading, for the
// status line.
func extremeReady(serial string) (extreme.State, bool) {
	project, err := extreme.FindProject()
	if err != nil {
		return extreme.State{}, false
	}

	apkPath, _ := gradle.FindApk(project.Root)

	return extreme.Status(project.Root, serial, apkPath), true
}
