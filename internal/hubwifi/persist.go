package hubwifi

import (
	"fmt"
	"strings"

	"github.com/andreibanu/pusher/internal/adb"
)

// None of this survives a reboot on its own: the module lives in RAM, the
// supplicant dies with it, and the routes go with the interface. For a robot
// that gets powered down between matches that means doing it again every time,
// which is the difference between a thing you rely on and a thing you remember.
//
// So a boot hook. It is the most invasive thing pusher does to a robot, and it
// is written to be undone: one file in /system/etc/init, one directory under
// /data, and Undo removes both.

// BootScript is what runs on the hub at every boot.
const BootScript = Dir + "/boot.sh"

// InitScript is the hook init reads. Android 7 reads every .rc in this
// directory, so adding one is additive: nothing REV ships is edited.
const InitScript = "/system/etc/init/pusher-wifi.rc"

const initEntry = `# Written by pusher. Remove this file to stop the robot joining that network.
service pusher_wifi /system/bin/sh ` + BootScript + `
    class late_start
    user root
    group root
    disabled
    seclabel u:r:su:s0

on property:sys.boot_completed=1
    start pusher_wifi
`

// persist installs the boot hook.
func (r runner) persist(opt Options) error {
	r.say("[*] Making it come back after a reboot")

	// The module has to be on the hub whether or not this run needed to load
	// it. A hub whose interface was already up skips loading entirely, and
	// without this the boot script would find nothing to insmod and the whole
	// hook would be a file that runs and achieves nothing.
	if err := r.ensureModule(opt.Module); err != nil {
		return err
	}

	if err := r.push(r.bootScript(opt), BootScript); err != nil {
		return err
	}
	r.quiet("chmod", "755", BootScript)

	// /system is mounted read only and has to be let go of afterwards, whatever
	// happens in between: a hub left with a writable system partition is a hub
	// one bad shutdown from a corrupt one.
	if out, err := r.run("mount", "-o", "rw,remount", "/system"); err != nil {
		return fmt.Errorf("cannot make /system writable to install the hook: %s", strings.TrimSpace(out))
	}
	defer r.quiet("mount", "-o", "ro,remount", "/system")

	if err := r.push(initEntry, InitScript); err != nil {
		return err
	}
	r.quiet("chmod", "644", InitScript)

	r.say("[OK] It will rejoin on every boot, and put itself back if the adapter is unplugged")
	r.say("     and plugged in again. Undo with `pusher relay forget`")
	return nil
}

// ensureModule leaves a copy of the driver on the hub for the boot script.
func (r runner) ensureModule(module string) error {
	if _, err := r.run("ls", "-d", ModulePath); err == nil {
		return nil
	}

	if module == "" {
		var err error
		if module, err = r.carriedDriver(); err != nil {
			return fmt.Errorf("nothing to leave on the robot for the next boot: %w", err)
		}
	}

	return adb.Push(r.serial, module, ModulePath)
}

// Undo takes the boot hook back off, and leaves the hub as it was.
func Undo(serial string) error {
	prefix, err := rootPrefix(serial)
	if err != nil {
		return err
	}
	r := runner{serial: serial, root: prefix}

	if out, err := r.run("mount", "-o", "rw,remount", "/system"); err != nil {
		return fmt.Errorf("cannot make /system writable to remove the hook: %s", strings.TrimSpace(out))
	}

	r.quiet("rm", "-f", InitScript)
	r.quiet("mount", "-o", "ro,remount", "/system")
	r.quiet("rm", "-rf", Dir)

	return nil
}

// Persisted reports whether the boot hook is installed.
func Persisted(serial string) bool {
	prefix, err := rootPrefix(serial)
	if err != nil {
		return false
	}

	r := runner{serial: serial, root: prefix}
	out, err := r.run("ls", InitScript)

	return err == nil && strings.Contains(out, "pusher-wifi.rc")
}
