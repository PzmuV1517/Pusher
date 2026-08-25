```
From Team #14270

 ██████╗ ██╗   ██╗ █████╗ ███╗   ██╗████████╗██╗   ██╗███╗   ███╗
██╔═══██╗██║   ██║██╔══██╗████╗  ██║╚══██╔══╝██║   ██║████╗ ████║
██║   ██║██║   ██║███████║██╔██╗ ██║   ██║   ██║   ██║██╔████╔██║
██║▄▄ ██║██║   ██║██╔══██║██║╚██╗██║   ██║   ██║   ██║██║╚██╔╝██║
╚██████╔╝╚██████╔╝██║  ██║██║ ╚████║   ██║   ╚██████╔╝██║ ╚═╝ ██║
 ╚══▀▀═╝  ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═══╝   ╚═╝    ╚═════╝ ╚═╝     ╚═╝

 ██████╗  ██████╗ ██████╗  ██████╗ ████████╗██╗ ██████╗███████╗
██╔══██╗██╔═══██╗██╔══██╗██╔═══██╗╚══██╔══╝██║██╔════╝██╔════╝
██████╔╝██║   ██║██████╔╝██║   ██║   ██║   ██║██║     ███████╗
██╔══██╗██║   ██║██╔══██╗██║   ██║   ██║   ██║██║     ╚════██║
██║  ██║╚██████╔╝██████╔╝╚██████╔╝   ██║   ██║╚██████╗███████║
╚═╝  ╚═╝ ╚═════╝ ╚═════╝  ╚═════╝    ╚═╝   ╚═╝ ╚═════╝╚══════╝
```

# Pusher

One command to build an FTC project and deploy it to the robot.

```bash
pusher
```

If a hub is on USB it uses that and leaves your Wi-Fi alone. Otherwise it builds
first, joins the robot's Wi-Fi, deploys, and puts you back on the network you
started on.

### [Pusher Extreme](#pusher-extreme) reloads your code instead of installing it

**2.89 seconds instead of about 39.** Only your team code goes to the robot, and
nothing is installed. Experimental, and under active development.

<details>
<summary><b>How it compares to Sloth</b></summary>

Sloth got here first and is the reason anyone knows this is possible. It is
worth being straight about where each one is ahead.

**Where Pusher Extreme is better**

- **Nothing is added to your robot app.** No library, no runtime, no annotations,
  no gradle plugin. One marked block goes into `TeamCode/build.gradle` and that
  is the entire footprint. Sloth is a dependency that ships a runtime with it.
- **Stock FTC Dashboard.** Sloth includes a drop-in replacement of Dashboard to
  make it compatible. Pusher registers your `@Config` classes with the real one
  from inside the reload, so you keep the Dashboard you already have. Panels is
  not handled yet.
- **It refuses when a reload would be a lie.** If anything outside team code
  changed, it installs instead and says which input moved. Running stale code
  while everything reports success is the worst thing a tool like this can do.
- **It is the command you already run.** `pusher` decides for itself whether to
  reload or install; there is no second task to remember.

**Where Sloth is better**

- **Sloth is faster.** It advertises under a second, ceiling of two. Pusher
  Extreme is 2.89s, of which 1.09s reaches the robot and the rest is compiling.
- **Sloth is safe to deploy while an OpMode is running**, because it applies the
  change when the OpMode ends. Pusher Extreme reloads immediately, and what that
  does mid-OpMode has not been established. Stop the OpMode first.
- **`@Pinned` is finer grained.** Sloth pins individual classes. Pusher keeps
  whole packages in the APK, which is a blunter instrument.
- **Sloth is an extensible runtime.** Sinister classpath scanning lets libraries
  and your own code hook into reloads. Pusher has one internal bridge and no
  public extension point.
- **Sloth is mature.** Pusher Extreme has been measured on one project, on one
  hub, and is explicitly experimental.

</details>

## Install

```bash
brew install PzmuV1517/PzmuV1517/pusher
```

Or from source:

```bash
go build -o pusher && sudo mv pusher /usr/local/bin/
```

Requires `adb` and an FTC project with a Gradle wrapper.

| OS | Wi-Fi switching via | adb |
|---|---|---|
| macOS | `networksetup` | `brew install android-platform-tools` |
| Debian/Ubuntu | `nmcli` (`sudo apt install network-manager`) | `sudo apt install adb` |
| Windows | `netsh` + PowerShell | Android SDK Platform-Tools |

## Commands

| Command | Description |
|---|---|
| `pusher` | Build and deploy |
| `pusher connect` | Join the robot Wi-Fi and connect adb |
| `pusher exit` | Disconnect adb and return to your Wi-Fi |
| `pusher dc` | Disconnect adb only |
| `pusher settings` | Profiles and preferences |
| `pusher slim` | Shrink the APK (`--undo` to revert) |
| `pusher hwconfig` | Pull, edit and push the robot's hardware configs |
| `pusher doctor` | Diagnose Wi-Fi, adb and project problems |
| `pusher visualiser <OpMode>` | Draw the path an auto drove, coloured by speed |
| `pusher power` | Show what drew the most current on the last run |
| `pusher prepare` | Cache Gradle dependencies while online |
| `pusher help` | Help |

## Settings

`pusher settings` opens a menu covering robot profiles, which network to return
to, whether to prefer USB, slimming, delta transfer, and Gradle threads. Changes
save immediately to `~/.config/pusher/config.yaml`.

**Update pusher** checks for a newer release and installs it. A Homebrew install
is handed to `brew upgrade` so the next one does not undo it; anything else
replaces its own binary, verified against the release checksums.

Pusher also looks for a newer release on its own, once a day, in the background.
When it finds one it says so with a desktop notification rather than a line in
the terminal, because the point is to reach somebody who has stopped reading it:
macOS through osascript, Linux through notify-send, Windows through a toast. A
version is announced once rather than every day until it is installed, and a
check that cannot reach GitHub says nothing at all. Turn it off with
**Tell me about updates**, or `PUSHER_NO_NOTIFY=1` to keep the check and drop
only the notification.

If your project uses the blob library, a deploy says when a newer release of it
is out, on whichever branch you follow. Twice: once at the start, where it is still cheap to stop and take it,
and once at the end, where it is the last thing on screen rather than something
a minute of Gradle output scrolled past. The check runs beside the deploy rather
than in front of it, and a project without blob, or a machine without access to
the repository, is told nothing.

**Release branch** in the blob menu chooses which of blob's branches this
project follows. blob publishes branch work as a labelled tag,
`v1.8.0-RSTController.1`, which GitHub marks as a pre-release, and the label up
to its first dot is the branch. Pusher reads that out of the tag rather than
asking which branch a release targets, because branches get deleted once merged
while the releases made from them stay.

Picking a branch moves the project onto the newest release from it, so it is one
action rather than a setting that changes and a project that does not. **Version**
then means the newest on that branch, and so does the line a deploy prints.
Everything else is unchanged: same two builds, same asset names, same menu.

## What is drawing the current

Turn **Power monitor** on in `pusher settings`, deploy, drive, stop the OpMode,
then look at what it recorded. Every run is its own file, so a practice session
leaves one per run, newest first:

```
  One run each, newest first.

   TeleOP                       15:31:57
 > TeleOP                       15:24:57
   TeleOP                       15:19:57
   CloseBlue                    15:02:57
```

**Power readings** in `pusher settings` opens that list and shows any of them.
`pusher power` prints the newest without opening a menu:

```
TeleOP, 42s
─────────────────────────────────────────
  motor                    peak  average   peak at
  shooter                21.70A    6.50A     30.1s
  left_front              8.40A    2.10A     12.3s
  right_front             7.90A    2.00A     12.1s
  intake                  5.20A    1.40A      8.0s

                         12.00A drawn by the motors together, on average
  Control_Hub (hub)      33.50A peak, 14.20A average
  battery                13.40V down to 11.20V, a sag of 2.20V
```

Which turns "the battery keeps dying" into a name and a number. Motors are
ranked by peak, since that is the one that trips breakers and sags the battery,
and the moment it happened is there so it can be matched against what the robot
was doing. The hub is reported separately rather than ranked beside the motors:
its reading is everything plugged into it, so it would win every list and tell
you nothing.

### It costs loop time, and is not for matches

**Do not run this in an official match.** A motor's current cannot be read in a
bulk transfer: the bulk packet carries an over-current flag but not the value,
so every reading is its own round trip over the RS485 bus. Reading eight motors
ten times a second is eighty round trips a second that your robot was not making
before, and they compete with the ones your OpMode makes.

Sampling runs on its own thread rather than in your loop, and at 10Hz rather
than every iteration, so the cost is as small as it can be made. It is not zero.
Pusher says so on **every** deploy while the monitor is installed, because the
way this goes wrong is somebody turning it on during practice and forgetting:

```
[!] The power monitor is installed, so this build reads motor current
    while every OpMode runs. That costs loop time: a motor's current
    cannot be bulk read, so each reading is its own trip over the bus.
    Turn it off in `pusher settings` before an official match.
```

### How it works

Turning it on writes one generated file into your project, under a
`pusherpower` package, and turning it off deletes it. Nothing else in your
project changes: no library, no dependency, and no edits to your OpModes.

It attaches itself. The file carries a `@WebHandlerRegistrar` method, which the
robot controller calls once at startup, and from there it listens for OpModes
starting and stopping. That is the same hook FtcDashboard uses to attach, which
is the reason to trust it. Your OpModes do not know it exists, so there is
nothing to remember before a practice run and nothing to take out afterwards
except the toggle.

Readings are held in memory and written once when the OpMode stops, to
`/sdcard/FIRST/pusher-power`, which is where `pusher power` reads them from.
`pusher power --clear` deletes them.

With Pusher Extreme set up, the monitor is kept in the APK rather than reloaded.
It has to be: the startup hook is called when the robot controller's web server
comes up, long before anything is reloaded, so a monitor that arrived in a
reload would be found and never called. Pusher adds it to the keep list itself.

## Hardware configurations

The robot's hardware configuration is one XML file in `/sdcard/FIRST` that the
Driver Station writes. `pusher hwconfig` brings those into your project so they
can be read, edited and committed next to the code that names the devices.

Run it on its own and it opens a menu covering all of it. Everything below is
also a subcommand, for scripting or for when you know exactly what you want:

```
pusher hwconfig                 open the menu
pusher hwconfig list            what the robot and the project each have
pusher hwconfig pull            copy the robot's configs into configs/
pusher hwconfig view comp       show what is wired where
pusher hwconfig edit comp       open it in $EDITOR, check it, offer to push
pusher hwconfig diff            what changed against the robot
pusher hwconfig push comp       copy it back
```

Configurations land in `configs/` at your FTC project root. Use `--dir` to keep
them somewhere else.

### The editor

The menu's editor works on ports and devices rather than on XML, which is what
lets it help:

- **Device types autocomplete.** Type `pinpoint` and it finds
  `goBILDAPinpoint`; type `go` and it lists the goBILDA parts. The list is every
  type the SDK ships, read out of the FTC jars.
- **New devices land on a free port.** Pick a type and the port is filled in
  with the lowest one nothing is using, per bus, for I2C.
- **Problems show up as you type**, not after a failed push: a name that is
  already taken, a port that is already used, a port the hub does not have.
- **Nothing is written until you save.** Backing out of an edit leaves the file
  untouched, and the whole tree is marked with what is wrong before you push.

Reading, saving and pushing all preserve the file byte for byte apart from what
you actually changed, same declaration, same indentation, same attribute order
as the Driver Station writes. A rename comes out as a one-line diff.

If you would rather use your own editor, `pusher hwconfig edit <name>` opens
`$EDITOR` on the raw XML and checks it when you save.

Files move byte for byte in both directions. Pusher parses them to check and
describe them, never to rewrite them.

**Before pushing**, each file is checked for what the robot controller would
reject: two devices sharing a name, two devices on one port, a port the hub does
not have, an Expansion Hub on the address reserved for the Control Hub. Errors
stop the push (`--force` overrides); anything pusher is unsure about is a
warning. Device types it does not recognise, your own OnBotJava or external
library drivers, still have their names checked but are left alone otherwise.

**Overwriting is guarded.** The robot's copy of anything about to be replaced is
saved into `configs/.pusher-backup/` first, because it may have been changed on
the Driver Station since you pulled it. `--no-backup` skips that.

**Pushing does not activate.** The robot controller reads a configuration when
it is selected, not while it is running one, so overwriting the active file
changes nothing until you re-select it on the Driver Station: Configure Robot →
pick it → Activate. Pusher says so when the file you pushed is the active one.

Reading *which* configuration is active needs privileged adb. That works on a
Control Hub; on a phone robot controller pusher says it could not tell rather
than guessing.

## Visualising an autonomous

`pusher visualiser CloseBlue` pulls a path trace off the robot and renders an HTML
page: the whole flow of the auto, every curve coloured by modelled speed, and a
duration estimate next to the measured time.

```bash
pusher visualiser CloseBlue     # newest trace for that OpMode
pusher visualiser               # newest trace on the robot
pusher visualiser --file t.json # a trace you already have
```

Segments are labelled with the `case` they came from. The blob library captures a
stack trace on each path commit and pusher maps the line number back into your
source, so it works whatever shape the auto is: state machine, inheritance chain,
poses from a constants class. Nothing to annotate.

Colour is modelled speed, not commanded power. Pusher runs a forward/backward
sweep over each curve capped by `maxPower`, by acceleration, and by how hard the
curve bends, so a leg that stays cold is usually cornering-limited and lowering
maxPower there costs you nothing. Tune the model to your drivetrain with
`--top-speed`, `--accel`, `--decel` and `--lat-accel`; the gap between the
estimate and the measured time tells you how far off the defaults are.

Recording requires the `blob-dev` artifact and `BlobParams.recordTrace = true`.
Competition builds of blob contain no recording code at all, so a robot you take
to a match cannot log even if the flag is set.

## Making deploys faster

**Put the Control Hub on 5 GHz.** Hold the hub's button through power-on and
release when the LED turns magenta (yellow is 2.4 GHz). Needs Control Hub OS
1.1.2+. Biggest win available, and it costs nothing.

**Only changed parts are sent.** On by default. The hub keeps the APK in pieces
under `/data/local/tmp/pusher`, which survives reboots, so later pushes transfer
only what differs, measured at 0.6 MB instead of 74 MB for a one-line change.
The rebuilt APK is checksummed on the hub before installing; anything unexpected
falls back to a full transfer.

**`pusher slim`** drops the native libraries for the CPU your hub does not have,
which is about 10 MB of a stock FTC APK. It asks the connected hub which
architecture it runs and refuses to guess, so connect the robot first. Files it
edits are backed up next to themselves; `pusher slim --undo` restores them.

It needs a Groovy `build.common.gradle`. On a project configured with the Kotlin
DSL it will not work, and is not going to: the lines it patches are written by
your team in whichever of several shapes you wrote them, so it would sit there
matching nothing while every deploy went on packaging everything. Pusher stops
rather than pretend. Pusher Extreme is different and supports both the Kotlin
DSL and Kotlin source files, because it writes one block of its own rather than
editing yours.

## Deploy speed

A deploy is two halves that behave differently: getting the bytes to the robot,
and the package manager installing them once they arrive. The install is not
just a copy. It writes the APK into `/data/app`, verifies the signature,
extracts the native libraries if they are compressed, and runs dexopt over
every dex file. On a stock FTC project that is tens of megabytes of writes and
tens of megabytes of dex to compile.

`pusher settings` -> **Deploy speed** has a switch for each part of that, because
what wins over USB is not what wins over the robot's 2.4 GHz hotspot:

| setting | what it does | default |
|---|---|---|
| Send only changed parts | sends only the chunks of the APK that changed | on |
| Skip install when unchanged | does nothing at all if the robot already holds this build | on |
| Stream the install | writes the APK straight into an install session instead of pushing it to a temporary file first, halving what gets written on the robot | on |
| Store native libraries uncompressed | stops the install extracting 20 MB+ of libraries, at the cost of a bigger APK. Applied by `pusher slim` | off |
| Install only changed splits | when the project builds a base plus a feature module, installs only the module that changed | off |

The last two are not free. Storing the libraries makes the APK bigger, which
costs transfer time, so it is a win on USB or 5 GHz and a question on 2.4 GHz.
Stored entries also make the delta cache far more effective, because one changed
byte in a deflate stream shifts everything after it while stored bytes do not
move.

Everything falls back safely. A streaming install that the hub does not like
drops to the staged one; a split install with nothing to inherit from installs
the whole APK.

Do not guess which of these to turn on. Measure them.

## Pusher Extreme

**Experimental, and under active development.** It works, it is measured, and
it is not finished. Read the precautions before a competition.

Pusher Extreme reloads your OpModes onto a running robot instead of installing
an APK. Only your team code is sent, so the write-run-test cycle stops being
dominated by waiting.

A full deploy on a Control Hub measured around **39 seconds**, and every
transfer trick pusher has was within noise of that, because the cost is the
install and not the transfer. Pusher Extreme does not install:

| stage | best of 5 | spread |
|---|---|---|
| resolve the classpath (gradle) | 307ms | 216ms |
| compile (javac + d8) | 1.47s | 173ms |
| push and trigger | 1.09s | 112ms |
| **total** | **2.89s** | 484ms |

204 classes, 727 KB sent. Timed from the start of the command to the robot
being told to reload; the robot picks it up on its next event loop tick.

**Roughly thirteen times faster than installing**, and the numbers above are
the whole command, not the interesting part of it. Two of the three stages
never touch the robot, and resolving the classpath is not cached yet.

For honest comparison: Sloth advertises under a second, with a ceiling of two.
Pusher Extreme is slower than that today. Its 1.09s is the part that reaches
the robot; the rest is compiling, which those figures may not include.

### What it does beyond swapping the code over

- **Changes survive restarts and power cycles.** The reloaded code lives on the
  robot, not in memory.
- **It refuses to reload when a reload would be a lie.** If anything outside
  team code changed, or the robot is not running the build this project
  produces, it installs and says which. Reloading where an install was needed
  leaves a robot running stale code with everything reporting success, and that
  is discovered by the robot driving last week's autonomous.
- **FtcDashboard keeps working.** Dashboard finds `@Config` classes by scanning
  the APK, which reloaded code is not in. Pusher registers them from inside the
  reload instead, so live tuning survives. 43 classes in the project measured
  above.
- **No changes to the robot controller app.** No custom classloader, no
  bytecode rewriting, nothing installed on the robot that is not your code.
  Pusher uses a path the SDK already supports.
- **Nothing is triggered until it is verified.** Both files are checked on the
  robot before anything is told to reload, because re-registration abandons
  everything on the first failure rather than skipping one file.
- **One marked block** is added to `TeamCode/build.gradle`. The same menu takes
  it out.

### Precautions

- **Only `org.firstinspires.ftc.teamcode` is reloaded.** Everything else lives
  in the APK.
- **Hardware device drivers written in team code stay in the APK.** Pusher finds
  them and keeps them there automatically, one file at a time, so the rest of
  the package still reloads. They cannot be reloaded: every reload builds a new
  classloader, and a driver loaded from the reload is a different class each
  time while the device in the hardware map was built under an earlier one. The
  robot would report that it cannot find its own hardware.
- **Your team code is not in the APK while this is set up.** That is what makes
  it work: a class in the APK always wins. It also means a teammate deploying
  from Android Studio gets a robot with no OpModes until pusher reloads them.
- **Do not reload while an OpMode is running.** A reload rebuilds the
  classloader under a live app, and what the SDK does with that mid-OpMode has
  not been established. Stop the OpMode first.
- **Code can compile and still not reload.** Installing or changing a library,
  or changing anything outside team code, needs a full install. Pusher detects
  that and installs instead, but it cannot detect a library that reaches back
  into your classes: that fails at runtime, not at compile time.
- **Deleting a whole `@Config` class** leaves its entry in the dashboard until
  the robot app restarts. Adding classes, and adding or removing fields, are
  handled.
- **OnBotJava cannot be used at the same time.** Both use the same mechanism to
  say where classes live, and whichever ran last wins.

### Setting it up

`pusher settings` -> **Pusher Extreme** -> Set up this project, then turn on
Use it when deploying. Deploy once to install an APK without team code in it;
every deploy after that reloads.

The same menu undoes it. Deploy once afterwards so the robot gets an APK with
your team code back in it.

Turning **Use it when deploying** off does not put team code back in the APK.
Only undoing the setup does: the exclusion lives in your gradle file, and that
is what the build obeys. Pusher deploys correctly either way, reloading after
the install even when the setting is off, and says so when it does. The menu
shows that state as `off (project still set up)`.

### It checks that the robot agreed

Delivering classes and having them registered are different things, and until
now pusher only knew about the first. A reload that pushed every file and
registered none of them printed the same success as one that worked.

So after each reload pusher asks the robot what it actually has. FtcDashboard
reports the robot's own OpMode list, straight out of `RegisteredOpModes`, which
is the same list the Driver Station shows. Compared against the OpModes your
source declares, that turns a silent disappearance into a line of output:

```
[!] the robot registered 32 of 33 OpModes. Missing: RST TUNING
    They compiled and reached the robot, so this is registration rather
    than delivery. Usually the app never rescanned: restart the robot app,
    since the SDK attaches its reload watch when it starts.
```

The check costs one connection and is best-effort: a project without
FtcDashboard deploys exactly as before and is told nothing, because the
dashboard is not a requirement and a working deploy should not end with a
paragraph about a library you chose not to use.

### Kotlin

Both kinds. A module configured with `build.gradle.kts` gets its exclusion
written in that dialect, and `.kt` source files are compiled and reloaded
alongside `.java` ones.

The compiler is the one your project already builds with. Pusher reads the
Kotlin version off the `kotlin-stdlib` on your compile classpath and takes the
matching `kotlin-compiler-embeddable` out of the Gradle cache, where the Kotlin
plugin put it. Nothing is downloaded, and a project that has never been built is
told to build once rather than guessed at. Kotlin is compiled first, with your
Java files passed alongside for their signatures, then javac compiles the Java
against what came out, so the two halves can refer to each other in both
directions. Measured on a real Kotlin project: 7 files, 2.0 seconds to a dex.

The exclusion has a second half for Kotlin, and it is the half worth knowing
about. Excluding from the android source set does nothing to `.kt` files,
because the Kotlin plugin compiles from a source set of its own. Without the
extra block, team Kotlin stayed in the APK while also being reloaded, and
parent-first delegation means the packaged copy is the one that runs: the reload
would report success and change nothing. Measured before the fix, six of six
Kotlin classes survived an exclusion that reported success.

`@Config` classes written as Kotlin `object`s are bridged like any other, named
by their declaration rather than their file, since Kotlin does not require the
two to match.

### How it works, briefly

The FTC SDK already loads classes from outside the APK, and already watches a
file to know when to rescan. Pusher compiles your team code on your laptop,
puts it where the SDK reads from, and touches that file. The SDK throws away its
classloader, builds a new one and rediscovers your OpModes through the same path
it uses for everything else.

Checked against the libraries in a real project, reading the full artifacts
rather than their API jars: pedro, ftclib, EasyOpenCV and blob do not go looking
for classes at all, so they need nothing.

Two do. **FtcDashboard** scans the APK with `getPackageCodePath`, and is handled:
pusher registers your `@Config` classes with it from inside the reload.
**Panels** scans the same way through its own `ClassFinder`, and is **not
handled yet**, so anything Panels discovers by scanning will not see reloaded
classes. Panels used directly from your own code is unaffected.

A library that scans and is not handled can have its package kept in the APK
instead.

## Per-OS notes

Pusher needs to know which network to put you back on. How it works that out
differs by platform. `pusher doctor` shows which backend is in use.

**macOS** hides the current Wi-Fi name from command-line tools, and your
terminal cannot be added to Location Services by hand: macOS only lists apps
that have already asked, and command-line tools have not been able to ask since
macOS 13. Pusher instead reads the saved-network list, which macOS keeps in
most-recently-joined order, so the network you are on is the first entry. No
permission needed, recomputed every run.

**Debian/Ubuntu** is the easiest case. NetworkManager reports the SSID freely
and records a real last-connected timestamp per saved network, so returning you
to the right network is stored fact rather than inference. Machines managed by
ifupdown or systemd-networkd instead of NetworkManager cannot switch networks;
connect to the robot yourself and pusher will deploy over that connection.

**Windows** reports the SSID freely, so a normal push is fine. But it keeps no
record of when each saved network was last used, so a standalone `pusher exit`
cannot tell where you came from. Set the network to return to in
`pusher settings` → Home Wi-Fi network. Note also that `netsh` cannot take a
password inline, so pusher generates a WPA2-PSK profile and imports it before
connecting.

If the network is ever guessed wrong on any platform, pin it in
`pusher settings` → Home Wi-Fi network, which always wins.

### Looking for the hub

`Could not find network 14270-RC` does not mean the hub is off. It means the
network was not in the driver's list of what is nearby at the moment the join
asked, which is the normal state for a hub that was switched on a minute ago:
the list is only refreshed when something scans, and nothing had.

So pusher scans. It starts looking for the hub's network in the background
before the build begins, which costs nothing because the build was going to take
that long anyway, and by the time the join happens the hub is in the list. It
stops scanning before the join and stays stopped for the deploy, because a scan
hops the radio across every channel and would cost the transfer several seconds
of throughput.

The payoff is that a failed join now comes with the half the OS cannot tell you:

```
[!] 14270-RC is not broadcasting.
    Trying anyway, but check the hub is powered on and nearby.
Error: failed to join "14270-RC": Could not find network 14270-RC.
    pusher never saw that network while it was looking, so the hub is
    probably switched off, still starting up, or out of range
```

and when the hub *was* seen, pusher says so and retries the join once, which is
usually all a hub that is sitting right there needs.

`pusher doctor` answers the same question on demand, under **Robot Wi-Fi**.

On macOS this works despite Location Services, which is the part worth knowing:
the name filter is applied inside the OS, before the redaction that hides
network names from command-line tools. Pusher never learns the names of the
networks around you, only whether the one it asked about is among them. The
pace is set by the radio, which takes about four seconds per scan and refuses
another for ten. Linux asks nmcli for a rescan; Windows reads the list netsh
keeps. Set `PUSHER_NO_SCAN=1` to turn it off everywhere.

## What pusher sends

Once a day, pusher reports that it ran. Three things: a random ID it generated
for itself the first time it started, the pusher version, and your operating
system and CPU (`darwin/arm64`). That is the whole payload, and there is a test
that fails if anything else is ever added to it.

It does not send your team number, your project, your file paths, your robot
configuration, your Wi-Fi names, your username, your hostname, or your IP beyond
the fact that a connection was made. The ID is random, not derived from your
hardware, so it identifies a copy of pusher and nothing else. The server hashes
it with a secret before storing it, so even the database cannot be matched back
to the ID sitting in your config file.

It exists to answer one question: how many teams use this, and which versions
are they on. That is what makes it safe to change something, rather than
guessing about who would notice.

The request runs in the background, never blocks a deploy and never fails one.
If it cannot reach the server it gives up in three seconds and says nothing.

To turn it off:

```bash
PUSHER_NO_TELEMETRY=1 pusher        # once
pusher settings  →  Count this device  # for good
```

With it off, nothing is sent and no ID is generated at all. Builds with no
counter configured send nothing regardless of the setting; `pusher settings`
says "not set up" when that is the case.

The server is a Cloudflare Worker in [`worker/`](worker/), about a hundred lines
of it, including what it stores and what it refuses.

## Credits

Made with love by **Andrei "PzmuV1517" Banu**

From **Team #14270**

MIT licensed.
