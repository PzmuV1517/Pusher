package bench

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andreibanu/pusher/internal/adb"
)

// Run is one measured deploy.
type Run struct {
	Name string

	What string

	Transfer time.Duration
	Install  time.Duration
	Bytes    int64
	Skipped  bool

	// Spread is the gap between the fastest and slowest sample. A deploy is
	// noisy enough that without it a few seconds of variance reads as a
	// finding.
	Spread  time.Duration
	Samples int

	Err error
}

// Total is the whole deploy, transfer and install together.
func (r Run) Total() time.Duration { return r.Transfer + r.Install }

// APK is what the project currently builds, measured rather than assumed.
type APK struct {
	Path string
	Size int64

	LibBytes      int64
	LibPacked     int64
	LibCompressed bool

	DexBytes int64
	DexFiles int

	// Stale means the APK is older than the gradle files that describe how to
	// build it, so it does not reflect the current settings.
	Stale bool
}

// Inspect reads the composition of an APK.
func Inspect(path string) (APK, error) {
	out := APK{Path: path}

	info, err := os.Stat(path)
	if err != nil {
		return out, err
	}
	out.Size = info.Size()
	out.Stale = olderThanGradle(path, info.ModTime())

	archive, err := zip.OpenReader(path)
	if err != nil {
		return out, fmt.Errorf("cannot read the APK: %w", err)
	}
	defer archive.Close()

	for _, entry := range archive.File {
		switch {
		case strings.HasPrefix(entry.Name, "lib/") && strings.HasSuffix(entry.Name, ".so"):
			out.LibBytes += int64(entry.UncompressedSize64)
			out.LibPacked += int64(entry.CompressedSize64)
			if entry.Method != zip.Store {
				out.LibCompressed = true
			}
		case strings.HasSuffix(entry.Name, ".dex"):
			out.DexBytes += int64(entry.UncompressedSize64)
			out.DexFiles++
		}
	}

	return out, nil
}

// Options controls what a benchmark run does.
type Options struct {
	Serial string
	APK    string

	Splits []string

	Repeat int

	// Timeout bounds one measured deploy. A hub that drops off mid-install
	// would otherwise stall the whole benchmark with nothing on screen.
	Timeout time.Duration

	Progress func(string)
}

// Deploy measures the configurations that differ at deploy time.
func Deploy(opt Options) []Run {
	if opt.Repeat < 1 {
		opt.Repeat = 1
	}
	if opt.Timeout <= 0 {
		opt.Timeout = 10 * time.Minute
	}

	report := func(msg string) {
		if opt.Progress != nil {
			opt.Progress(msg)
		}
	}

	configs := []struct {
		name string
		what string
		opts adb.Options
	}{
		{
			"Android Studio equivalent",
			"one streamed session install of the whole APK, no delta",
			adb.Options{Stream: true},
		},
		{
			"pusher, staged install",
			"push to a temporary file, then install from it",
			adb.Options{},
		},
		{
			"pusher, streamed install",
			"stream the APK into the install session",
			adb.Options{Stream: true},
		},
		{
			"pusher, delta transfer",
			"send only changed chunks, then install",
			adb.Options{Delta: true},
		},
		{
			"pusher, delta + streamed",
			"changed chunks, streamed into the session",
			adb.Options{Delta: true, Stream: true},
		},
	}

	var runs []Run

	for _, cfg := range configs {
		best := Run{Name: cfg.name, What: cfg.what}

		var slowest, fastest time.Duration

		for attempt := 0; attempt < opt.Repeat; attempt++ {
			report(fmt.Sprintf("%s (%d/%d)", cfg.name, attempt+1, opt.Repeat))

			adb.ForgetInstalled(opt.Serial)

			run := measure(opt.Serial, opt.APK, cfg.opts, opt.Timeout)
			run.Name, run.What = cfg.name, cfg.what

			if run.Err != nil {
				best = run
				break
			}

			total := run.Total()
			if attempt == 0 || total < fastest {
				fastest = total
			}
			if total > slowest {
				slowest = total
			}

			if attempt == 0 || total < best.Total() {
				keep := best
				best = run
				best.Name, best.What = keep.Name, keep.What
			}
		}

		if best.Err == nil {
			best.Spread = slowest - fastest
			best.Samples = opt.Repeat
		}

		runs = append(runs, best)
	}

	report("skip when unchanged")
	runs = append(runs, measureSkip(opt.Serial, opt.APK))

	if len(opt.Splits) > 1 {
		report("split install")
		runs = append(runs, measureSplit(opt.Serial, opt.APK, opt.Splits))
	}

	return runs
}

func measure(serial, apk string, opts adb.Options, limit time.Duration) Run {
	info, err := os.Stat(apk)
	if err != nil {
		return Run{Err: err}
	}

	// Buffered, so a run that outlives its deadline can still finish writing
	// rather than leaking a blocked goroutine.
	done := make(chan error, 1)

	start := time.Now()
	go func() {
		_, err := adb.InstallWith(serial, apk, opts)
		done <- err
	}()

	select {
	case err := <-done:
		return Run{Install: time.Since(start), Bytes: info.Size(), Err: err}
	case <-time.After(limit):
		return Run{Err: fmt.Errorf("gave up after %s; the robot stopped responding", limit)}
	}
}

func measureSkip(serial, apk string) Run {
	run := Run{
		Name: "pusher, nothing changed",
		What: "the robot already holds this exact build",
	}

	if _, err := adb.InstallWith(serial, apk, adb.Options{Stream: true, SkipUnchanged: true}); err != nil {
		run.Err = err
		return run
	}

	start := time.Now()
	plan, err := adb.InstallWith(serial, apk, adb.Options{Stream: true, SkipUnchanged: true})
	run.Install = time.Since(start)
	run.Err = err
	run.Skipped = plan.Skipped

	if !plan.Skipped && err == nil {
		run.Err = fmt.Errorf("the skip did not trigger, so this is a full install")
	}

	return run
}

func measureSplit(serial, apk string, splits []string) Run {
	run := Run{
		Name: "pusher, changed split only",
		What: "install only the feature module that changed",
	}

	pkg := adb.PackageName(apk)
	if pkg == "" {
		run.Err = fmt.Errorf("cannot read the package name, so splits cannot be installed")
		return run
	}

	start := time.Now()
	count, err := adb.SplitInstall(serial, pkg, splits)
	run.Install = time.Since(start)
	run.Err = err

	if err == nil && count == 0 {
		run.Skipped = true
	}

	return run
}

// olderThanGradle reports whether the APK predates any gradle file in the
// project, which means it was built under different settings.
//
// Both DSLs, since .gradle.kts does not match *.gradle and a measurement taken
// against an APK that no longer matches the build files is not a measurement of
// anything.
func olderThanGradle(apkPath string, built time.Time) bool {
	dir := filepath.Dir(apkPath)

	for i := 0; i < 6 && dir != "/" && dir != "."; i++ {
		for _, pattern := range []string{"*.gradle", "*.gradle.kts"} {
			matches, _ := filepath.Glob(filepath.Join(dir, pattern))
			for _, gradleFile := range matches {
				if info, err := os.Stat(gradleFile); err == nil && info.ModTime().After(built) {
					return true
				}
			}
		}
		dir = filepath.Dir(dir)
	}

	return false
}
