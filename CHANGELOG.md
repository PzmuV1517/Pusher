# Changelog

Every push to `main` cuts a release: the series comes from `VERSION` and the
patch counts what has landed since that file last changed. So a version here is
one commit, and the numbers are the ones on the releases page rather than a
grouping invented afterwards.

Anything not listed is in `git log`, which is the complete record.

## Unreleased

- **`pusher power` shows what drew the most current.** Turn the monitor on in
  `pusher settings` -> Power monitor, deploy, drive, and it reports peak and
  average draw per motor, the hub's total, and how far the battery sagged,
  ranked worst first, with the moment each peak happened. The monitor is one
  generated file that attaches itself through the same `@OnCreateEventLoop`
  hook FtcDashboard uses, so no OpMode changes and nothing to remember before a
  practice run.

  It costs loop time. A motor's current cannot be read in a bulk transfer, so
  every reading is its own round trip over the bus, and pusher says so on every
  deploy while the monitor is installed. **Not for use in an official match.**

- **Power readings** in `pusher settings` lists the runs on the robot and shows
  any of them, rather than only the newest. Each run is its own recording, named
  by OpMode and time so two runs of the same OpMode are told apart.

- When the monitor records nothing it leaves a note on the robot saying why, and
  `pusher power` shows the note instead of an empty directory. The log would
  have done, if anybody could read it: it is a ring buffer on a robot somebody
  has to plug into, and by the time the question is asked the line has gone.

- A robot with no recordings yet is no longer reported as a failure to look.
  `ls` on a directory that does not exist exits non-zero, adb passes that back
  as its own exit code, and the result was that "you have nothing recorded yet"
  surfaced as `cannot list ... exit status 1`. The same fault was in the trace
  listing, so `pusher visualiser` reported it too rather than saying there were
  no traces.

## 1.2.23

- **The blob menu picks a release branch.** blob publishes branch work as a
  labelled tag, `v1.8.0-RSTController.1`, which GitHub marks as a pre-release;
  the label up to its first dot is the branch. **Release branch** lists the
  branches that have releases and moves the project onto the newest one from
  whichever you choose. **Version**, and the line a deploy prints, then follow
  that branch rather than main.

## 1.2.22

- **A newer pusher announces itself.** Once a day, in the background, pusher
  asks whether a newer release exists and says so with a desktop notification
  rather than a line that scrolls past the end of a deploy. macOS through
  osascript, Linux through notify-send, Windows through a WinRT toast. The same
  version is announced once, not once a day until it is installed. Turn it off
  in `pusher settings` -> Tell me about updates, or with `PUSHER_NO_NOTIFY=1`.
- **A deploy says when blob has moved on**, at the start and again at the end,
  because the start is when it is still cheap to act on and the end is what is
  still on screen. The check runs beside the deploy rather than in front of it.

## 1.2.21

- Menus lay themselves out to a small terminal instead of treating anything
  under thirty-two columns as unset and pretending it was eighty. Scrolling a
  narrow window showed duplicates and pieces of the previous frame, because
  every row was padded past the edge, wrapped, and the view ran off the bottom.

## 1.2.20

- The dashboard tuning check now saves. It was on the config struct, had a
  getter and a setter, and was never written to the file, so every toggle
  reported success and came back off.
- A status message is cut to one line, which is the two lines the layout
  budgets for it.

## 1.2.19

- What the APK carries is decided by the gradle file rather than the setting.
  Turning Pusher Extreme off without undoing the setup left every deploy
  installing an APK with no team code in it and skipping the reload that
  supplies it, so the robot went on serving the dex from the last reload.

## 1.2.18

- Pusher checks that the robot registered the OpModes it was sent, instead of
  reporting success for a reload that registered none of them.
- Fixed the dashboard client reading a `data` key the robot does not send, which
  made `pusher dash diff` find nothing on a real robot.

## 1.2.17

- Pusher Extreme compiles and reloads Kotlin sources, with the compiler the
  project already builds with.
- The APK exclusion covers the Kotlin compile tasks too. Without that, team
  Kotlin stayed in the APK while also being reloaded, and the packaged copy is
  the one that runs.

## 1.2.16

- Pusher counts the devices it runs on: a random ID, the version and the
  platform, once a day. See [What pusher sends](README.md#what-pusher-sends).

## 1.2.15

- Pusher looks for the robot's Wi-Fi while the project builds, so the hub is in
  range by the time it joins, and a failed join says whether the hub was ever
  broadcasting.

## 1.2.11 - 1.2.14

- `pusher slim` stops rather than pretending to slim a Kotlin DSL project, where
  every pattern it edits matches nothing.
- Pusher Extreme writes its exclusion in the module's own gradle dialect.
- Extreme refuses a project it cannot reload rather than reloading part of it.

## 1.2.7 - 1.2.10

- Menus are grouped and laid out to the terminal.
- The signature that decides whether a reload is honest counts Kotlin DSL build
  files and version catalogs as build inputs.

## 1.2.1 - 1.2.6

- `pusher update` names the tap when handing an upgrade to Homebrew, and checks
  that brew actually upgraded before saying it did.
- `pusher dash diff` compares the robot's dashboard tuning against the source.
- A reload follows an install, so the robot is never left with no OpModes.
- The keep list expands to everything a kept class needs to compile.
