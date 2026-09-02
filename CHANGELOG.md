# Changelog

Every push to `main` cuts a release: the series comes from `VERSION` and the
patch counts what has landed since that file last changed. So a version here is
one commit, and the numbers are the ones on the releases page rather than a
grouping invented afterwards.

Anything not listed is in `git log`, which is the complete record.

## Unreleased

- **A flame chart of where the loop time goes.** The power monitor says what the
  battery is going into; this says where the time is going, which is the other
  half of the same question. Turn **Loop profiler** on in `pusher settings`,
  deploy, run an OpMode, then `pusher profile`. Nothing is instrumented and no
  OpMode is edited: a thread wakes every few milliseconds, asks the thread
  running the OpMode what it is in the middle of, and files the answer, so time
  is counted in samples. Each bar on the chart is as wide as the share of the
  run spent inside that method and everything it called, your own code is drawn
  in blue, and clicking a bar zooms into it. The table ranks by time spent in a
  method rather than under it, which is the difference between the code that was
  running and a list of everything that called it. The page says how much of the
  run the sampler actually got, because a profile that stands for a third of a
  run should not look like one that stands for all of it. It costs loop time,
  says so on every deploy, and is not for matches.

- **The menus stop drawing over themselves.** The repeated entry had two causes,
  and the first one happens before a key is pressed: bubbletea draws one frame
  before it handles the first resize, and the menus started out believing they
  had twenty four rows. In a fourteen row panel that first frame scrolled the
  terminal ten rows out from under the renderer, which had been counting from
  where it started, so every repaint after that landed at the wrong height and
  left rows of old frames on screen. Nothing is drawn now until the terminal has
  said how big it is.

  The second is that a frame exactly as tall as the window scrolls it as the
  last line is written, which puts the renderer one row out for the same reason.
  Every frame now leaves a row spare, which is what the lists padding themselves
  out to fill the window exactly had taken away.

- **The menus stop changing height as you move through them.** Going down a list
  and back up was enough to break them, and the shape was always the same: the
  block grew a row the moment a scroll marker appeared and lost one again at
  either end, so the whole view changed height as the cursor moved. The footer
  walked up and down the screen and a frame that had been taller left its bottom
  rows behind. Every list is now padded to a fixed height, the room for a status
  message is reserved whether or not there is one, and the six screens that had
  their own copy of the layout code use the one that measures rather than
  guesses. The hardware configuration menu had the same fault and its own copy
  again, and on a short terminal it overflowed outright: fourteen rows into ten.

  Moving the cursor now only moves the cursor. There used to be a second answer
  for where the window sits, computed on the keypress from a guessed chrome
  height, which the render then disagreed with and corrected. That is most of
  what made these menus feel unpredictable to move around in.

- **The path visualiser draws the run, not just the plan.** Every recorded
  position was counted in the caption and then thrown away, so the page drew the
  route the robot was asked to follow and called it the run. The driven path is
  now the solid line, taken from the robot's own positions in the order it
  recorded them and coloured by the speed it was really doing, with the planned
  path dashed underneath, so the gap between them is the following error. The
  view is sized to include both, since a run that overshot went outside the
  planned area by definition.

- **Straight legs are estimated again.** blob describes a straight leg with the
  only two points it needs, its ends, and both of those are stationary, so the
  model treated it as a segment that was stopped everywhere it had a point.
  Every straight leg estimated zero, which took the run's estimate, every peak
  and the whole colour scale with it: a page reporting `0.00 s` against a
  measured `28.97`, `-100%`, and a speed ramp topping out at 1 in/s. Curves are
  resampled before profiling, which is exact for a line and lets the heat map
  show the acceleration along one. The demo trace hid this for the life of the
  feature by sampling even its straight legs 49 times; it now records them the
  way blob does.

- **Pusher Extreme will not set up alongside Sloth.** Both reload team code onto
  the robot and both install a class loader for it, so the robot ends up holding
  two copies and loading whichever it reaches first. Traced from a team's robot
  log: Sloth's loader picked up the dex pusher had left in the OnBotJava jars
  directory nine hours earlier, every one of their fifteen classes failed to
  define because the copy pusher ships carries team code alone and the library
  those classes extend was not reachable from that loader, and the Driver
  Station listed no OpModes at all. It survives uninstalling pusher, the broken
  copy being on the robot rather than the laptop. Setup is now refused, and
  `pusher doctor` names the pairing for a project already in that state.

- **A reload nobody could check says so.** The check that the robot actually
  registered what a reload sent asked FtcDashboard, and said nothing at all when
  it could not ask, on the grounds that a dashboard is not a requirement. That
  is exactly backwards for a check whose whole purpose is the deploy that did
  not work, and the team it happened to ran Panels. A reload that cannot be
  verified now says which OpModes went unchecked and how to check them.

- **Panels is supported alongside FtcDashboard.** `pusher dash diff`, the
  dashboard watch and the post-reload check all ask both, because a team runs
  one or the other and which one is not something a laptop can tell without
  asking. Read out of the published AARs rather than from documentation: Panels
  serves a WebSocket on 8002, frames are `{pluginID, messageID, data}`, and the
  OpMode list and the tunables arrive unasked for when a client connects. Panels
  reports fully qualified class names where FtcDashboard reports simple ones,
  so its values are keyed the same way the source is and the comparison does not
  care which answered. `@Configurable` classes are read like `@Config` ones.

- **Anything that needs the robot offers to go and get it.** `pusher power`,
  `pusher hwconfig`, `pusher dash diff` and the visualiser stopped at "no robot
  connected - run `pusher connect`", which is one command telling you to run a
  second command so you can run the first one again. They now say what network
  they would join and ask, and connect if you say yes. Declining leaves the
  error exactly as it was, and a pusher running in a script is never asked
  anything: it gets the error it always got.

  The menus make the same offer with a key, since there is nowhere in a menu to
  print a question or read an answer. The hardware configuration menu, Power
  readings, the run picker and the developer menu say what would be joined and
  take `c`, then go back for whatever they could not read.

- **`pusher hwconfig push` checks that the robot got what it was sent.** Every
  configuration is read back and compared byte for byte, and the robot's own
  list is checked for the name. A push that adb reported as fine but that left
  nothing usable behind now fails where it happens instead of turning up later
  as a configuration that never appears.

- **`--restart`** on that push restarts the robot controller afterwards. The
  robot controller lists the directory afresh whenever it is asked, so a pushed
  configuration is there the next time the Driver Station asks; what it does not
  do is notice one appearing underneath a screen that is already open. Pushing
  now says so, and `--restart` settles it.

## 1.2.25

- **Power readings** in `pusher settings` lists the runs on the robot and opens
  any of them as a page, the way the path visualiser does, in the same style.
  Current against time for every motor and their sum, battery voltage against
  time, and a table ranked by charge drawn rather than by peak, since a motor
  that pulls 20 A for a tenth of a second costs the battery far less than one
  sitting at 4 A all match. Each run is its own recording, named by OpMode and
  time so two runs of the same OpMode are told apart.

  The graph is worked rather than looked at: drag across it to zoom in like an
  oscilloscope, double-click to zoom back out, click a name to take that line
  out of the way, and the battery graph follows the same window because the
  question after seeing a spike is always what the battery did at that moment.
  Moving across it reads out every value at that instant. All of it is inline,
  so the page still works on a laptop with no route anywhere.

  The monitor now keeps every reading rather than only the totals, which is what
  makes the graphs possible. Added as lines an older pusher skips, so recordings
  stay readable both ways. `pusher power` opens the newest; `--text` prints the
  numbers instead.

- **Everything the hardware can measure, not just motors.** The hub reports two
  more rails, and both are now recorded: the servo bus and the I2C bus. Neither
  servos nor I2C sensors have current sensing of their own, so one number each
  is all there is, and it is more than nothing. A Limelight is reported too, as
  how hard it is working rather than what it draws: it is not on a rail the hub
  measures, so its draw is inside the hub's total and nowhere else. It is found
  by name through reflection, so the generated file still compiles for a team on
  an SDK that has never heard of one.

- **Sample rate** in `pusher settings` chooses how often the monitor reads,
  from 20 ms to 500 ms. It is the whole cost of the feature, so the menu says
  what each rate costs rather than presenting them as equivalent. The value is
  baked into the generated file, since the robot cannot read this laptop's
  settings, so changing it rewrites the monitor and wants a deploy.

- **Changing the blob library forces a full install.** Adding, removing or
  swapping an AAR changes what the APK contains, and a reload only ever carries
  team code. Pusher worked that out for itself from the build inputs, but only
  once it had seen the robot: a library swapped in the menu with no robot
  connected left the next deploy reloading against the jar that was there
  before. It now says so up front and installs.

## 1.2.24

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
