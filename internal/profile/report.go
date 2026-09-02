package profile

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A profile is a tree of call paths with a sample count on each, plus the order
// the samples came in. Everything the page shows is worked out from those two
// things: the flame chart is the tree, the timeline is the order, and time is
// counted in samples multiplied by the period they were taken at.

// Frame is one call path in the tree.
type Frame struct {
	ID     int
	Parent int
	Name   string

	// Self is samples that ended here, Total is samples anywhere beneath it.
	// Self is what the robot was actually running; Total is what it was in the
	// middle of, which is the number a flame chart draws.
	Self  int
	Total int

	Kids []int
}

// Short is the frame without its package, which is what fits on a bar.
func (f Frame) Short() string {
	name := f.Name

	if i := strings.LastIndex(name, "."); i > 0 {
		if j := strings.LastIndex(name[:i], "."); j >= 0 {
			return name[j+1:]
		}
	}
	return name
}

// Team reports whether this frame is the team's own code rather than a library
// or the SDK, which is the part somebody can do something about.
func (f Frame) Team() bool {
	return strings.HasPrefix(f.Name, "org.firstinspires.ftc.teamcode.")
}

// Slice is one stretch of the run spent in one call path.
type Slice struct {
	Frame   int
	Samples int
}

// Report is one profiled run.
type Report struct {
	OpMode    string
	Class     string
	Started   time.Time
	Period    time.Duration
	Duration  time.Duration
	Samples   int
	Missed    int
	Truncated int

	Frames   []Frame
	Timeline []Slice

	// Problem is the note the profiler left instead of a recording.
	Problem string
}

// Seconds is how long a sample count stands for.
func (r Report) Seconds(samples int) float64 {
	return float64(samples) * r.Period.Seconds()
}

// Coverage is the fraction of the run the profiler actually sampled.
//
// Worth showing rather than hiding. A robot too busy to let the sampler run at
// its period produces a profile that is still correctly shaped but stands for
// less of the run than it looks like it does.
func (r Report) Coverage() float64 {
	if r.Duration <= 0 {
		return 0
	}
	return r.Seconds(r.Samples) / r.Duration.Seconds()
}

// Root is the frame everything hangs off.
func (r Report) Root() (Frame, bool) {
	for _, f := range r.Frames {
		if f.ID == 0 {
			return f, true
		}
	}
	return Frame{}, false
}

// Hottest is the frames that ran the most, by self samples, worst first.
//
// Self rather than total, because a method that is only ever waiting on the
// thing below it did not cost anything itself, and a list ranked by total is a
// list of every caller of the slow thing.
func (r Report) Hottest(limit int) []Frame {
	ranked := make([]Frame, 0, len(r.Frames))
	for _, f := range r.Frames {
		if f.ID != 0 && f.Self > 0 {
			ranked = append(ranked, f)
		}
	}

	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].Self > ranked[j].Self })

	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

// Title is what the page and the terminal call this run.
func (r Report) Title() string {
	if r.Started.IsZero() {
		return r.OpMode
	}
	return fmt.Sprintf("%s at %s", r.OpMode, r.Started.Format("15:04:05"))
}

// Parse reads a recording.
//
// Unknown keys are skipped rather than refused: a robot may be running a
// profiler written by a newer pusher, and a reader that stops at the first line
// it does not recognise turns a forwards compatible format into a broken one.
func Parse(content string) (Report, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return Report{}, fmt.Errorf("the recording is empty")
	}

	head := strings.Fields(strings.TrimSpace(lines[0]))
	switch {
	case len(head) == 0:
		return Report{}, fmt.Errorf("the recording is empty")
	case head[0] == "pusher-profile-problem":
		return problemReport(lines), nil
	case head[0] != "pusher-profile":
		return Report{}, fmt.Errorf("this is not a pusher profile recording")
	}

	report := Report{Period: DefaultPeriod * time.Millisecond}
	byID := map[int]int{}

	for _, line := range lines[1:] {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}

		switch fields[0] {
		case "opmode":
			report.OpMode = fields[1]
		case "class":
			report.Class = fields[1]
		case "started":
			if ms, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				report.Started = time.UnixMilli(ms)
			}
		case "period-ms":
			if ms, err := strconv.Atoi(fields[1]); err == nil && ms > 0 {
				report.Period = time.Duration(ms) * time.Millisecond
			}
		case "duration-ms":
			if ms, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				report.Duration = time.Duration(ms) * time.Millisecond
			}
		case "samples":
			report.Samples, _ = strconv.Atoi(fields[1])
		case "missed":
			report.Missed, _ = strconv.Atoi(fields[1])
		case "truncated":
			report.Truncated, _ = strconv.Atoi(fields[1])

		case "node":
			// node <id> <parent> <self> <name>
			if len(fields) < 5 {
				continue
			}
			id, err := strconv.Atoi(fields[1])
			if err != nil {
				continue
			}
			parent, err := strconv.Atoi(fields[2])
			if err != nil {
				continue
			}
			self, err := strconv.Atoi(fields[3])
			if err != nil {
				continue
			}

			byID[id] = len(report.Frames)
			report.Frames = append(report.Frames, Frame{
				ID:     id,
				Parent: parent,
				Name:   strings.Join(fields[4:], " "),
				Self:   self,
			})

		case "run":
			// run <node> <count>
			if len(fields) < 3 {
				continue
			}
			id, err := strconv.Atoi(fields[1])
			if err != nil {
				continue
			}
			count, err := strconv.Atoi(fields[2])
			if err != nil || count <= 0 {
				continue
			}
			report.Timeline = append(report.Timeline, Slice{Frame: id, Samples: count})
		}
	}

	if len(report.Frames) == 0 {
		return report, fmt.Errorf("the recording has no samples in it")
	}

	// The root is implied rather than written, every other node naming it as a
	// parent. Adding it here means the tree has one place to start from.
	report.Frames = append([]Frame{{ID: 0, Name: "all"}}, report.Frames...)
	link(&report)

	return report, nil
}

// link fills in each frame's children and the totals underneath it.
func link(r *Report) {
	at := make(map[int]int, len(r.Frames))
	for i, f := range r.Frames {
		at[f.ID] = i
	}

	for i := range r.Frames {
		f := &r.Frames[i]
		if f.ID == 0 {
			continue
		}
		if p, ok := at[f.Parent]; ok {
			r.Frames[p].Kids = append(r.Frames[p].Kids, f.ID)
		}
	}

	// Totals from the leaves up. Walked by depth rather than recursively,
	// because a recording is written by a robot and a cycle in it, however it
	// got there, must not become an unbounded recursion on this end.
	order := make([]int, 0, len(r.Frames))
	seen := make(map[int]bool, len(r.Frames))

	var walk func(id int, depth int)
	walk = func(id, depth int) {
		if depth > 512 || seen[id] {
			return
		}
		seen[id] = true
		order = append(order, id)

		if i, ok := at[id]; ok {
			for _, kid := range r.Frames[i].Kids {
				walk(kid, depth+1)
			}
		}
	}
	walk(0, 0)

	for i := range r.Frames {
		r.Frames[i].Total = r.Frames[i].Self
	}

	for i := len(order) - 1; i >= 0; i-- {
		idx, ok := at[order[i]]
		if !ok {
			continue
		}
		f := r.Frames[idx]
		if p, ok := at[f.Parent]; ok && f.ID != 0 {
			r.Frames[p].Total += f.Total
		}
	}

	// Children in the order that draws well: the widest bar first, so a flame
	// chart reads left to right by cost rather than by whatever order the robot
	// happened to see the calls in.
	for i := range r.Frames {
		kids := r.Frames[i].Kids
		sort.SliceStable(kids, func(a, b int) bool {
			return r.Frames[at[kids[a]]].Total > r.Frames[at[kids[b]]].Total
		})
	}
}

func problemReport(lines []string) Report {
	report := Report{Problem: "the profiler recorded nothing and did not say why"}

	for _, line := range lines[1:] {
		if rest, found := strings.CutPrefix(strings.TrimSpace(line), "message "); found {
			report.Problem = rest
			break
		}
	}

	return report
}

// Lines is the report as a few lines of terminal output.
func (r Report) Lines() []string {
	if r.Problem != "" {
		return []string{r.Problem}
	}

	out := []string{
		fmt.Sprintf("%d samples over %.1fs at %dms, covering %.0f%% of the run",
			r.Samples, r.Duration.Seconds(), r.Period.Milliseconds(), r.Coverage()*100),
	}

	for _, f := range r.Hottest(8) {
		share := 0.0
		if r.Samples > 0 {
			share = float64(f.Self) / float64(r.Samples) * 100
		}
		out = append(out, fmt.Sprintf("%5.1f%%  %6.2fs  %s", share, r.Seconds(f.Self), f.Name))
	}

	return out
}
