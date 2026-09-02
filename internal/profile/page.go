package profile

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// The flame chart is drawn in the browser rather than written out as a picture,
// because the first thing anybody does with one is click into it. A bar that is
// two pixels wide at full scale is the whole screen once its parent is zoomed,
// and that is usually where the answer is. A drawing made here would be a
// drawing of the question rather than of the answer.
//
// Everything is inline for the same reason the other pages are: this opens on a
// laptop that may be on a robot's Wi-Fi with no route anywhere, and a page that
// fetches a charting library renders blank.

// pageFrame is one bar, as the browser needs it.
type pageFrame struct {
	ID    int    `json:"i"`
	Kids  []int  `json:"k,omitempty"`
	Name  string `json:"n"`
	Short string `json:"s"`
	Self  int    `json:"f"`
	Total int    `json:"t"`
	Team  bool   `json:"m,omitempty"`
}

type pageSlice struct {
	Frame   int `json:"f"`
	Samples int `json:"n"`
}

type pagePlot struct {
	Frames   []pageFrame `json:"frames"`
	Timeline []pageSlice `json:"timeline"`
	Samples  int         `json:"samples"`
	PeriodMS float64     `json:"periodMs"`
}

type pageRow struct {
	Name    string
	Share   string
	Seconds string
	Team    bool
}

type pageData struct {
	OpMode   string
	Sub      string
	Samples  string
	Duration string
	Period   string
	Coverage string
	Hottest  string
	HotTime  string
	Rows     []pageRow
	Plot     template.JS
	Problem  string
	Warning  string
}

// Open shows the page in whatever the machine uses for HTML.
func Open(path string) {
	var c *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", path)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		c = exec.Command("xdg-open", path)
	}

	c.Start()
}

// Render writes the page and returns where it went.
func (r Report) Render(out string) (string, error) {
	if out == "" {
		name := safeName(r.OpMode)
		if r.Problem != "" {
			name = "problem"
		}
		out = filepath.Join(os.TempDir(), fmt.Sprintf("pusher-profile-%s.html", name))
	}

	tmpl, err := template.New("profile").Parse(profilePage)
	if err != nil {
		return "", err
	}

	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := tmpl.Execute(f, r.page()); err != nil {
		return "", err
	}
	return out, nil
}

func (r Report) page() pageData {
	d := pageData{
		OpMode:  r.OpMode,
		Problem: r.Problem,
	}

	if r.Problem != "" {
		d.OpMode = "Nothing was recorded"
		return d
	}

	d.Sub = fmt.Sprintf("%s · %d samples every %dms",
		r.Started.Format("2 Jan 15:04:05"), r.Samples, r.Period.Milliseconds())

	d.Samples = fmt.Sprintf("%d", r.Samples)
	d.Duration = fmt.Sprintf("%.1f", r.Duration.Seconds())
	d.Period = fmt.Sprintf("%d", r.Period.Milliseconds())
	d.Coverage = fmt.Sprintf("%.0f", r.Coverage()*100)

	if hot := r.Hottest(1); len(hot) > 0 {
		d.Hottest = hot[0].Short()
		d.HotTime = fmt.Sprintf("%.2f", r.Seconds(hot[0].Self))
	}

	for _, f := range r.Hottest(25) {
		share := 0.0
		if r.Samples > 0 {
			share = float64(f.Self) / float64(r.Samples) * 100
		}
		d.Rows = append(d.Rows, pageRow{
			Name:    f.Name,
			Share:   fmt.Sprintf("%.1f", share),
			Seconds: fmt.Sprintf("%.2f", r.Seconds(f.Self)),
			Team:    f.Team(),
		})
	}

	// Said on the page as well as in the terminal. Somebody opens this once and
	// keeps the tab, and the number that decides whether to believe it is how
	// much of the run it actually saw.
	if r.Coverage() < 0.8 && r.Duration > 0 {
		d.Warning = fmt.Sprintf(
			"The sampler only got %.0f%% of the ticks it asked for, so this stands for less of the run than it looks like. "+
				"The robot was busy enough to hold it off, which is itself worth knowing.", r.Coverage()*100)
	}

	d.Plot = r.plotJSON()
	return d
}

// plotJSON is the tree and the timeline, for the browser to draw.
func (r Report) plotJSON() template.JS {
	p := pagePlot{
		Samples:  r.Samples,
		PeriodMS: float64(r.Period.Milliseconds()),
	}

	for _, f := range r.Frames {
		p.Frames = append(p.Frames, pageFrame{
			ID:    f.ID,
			Kids:  f.Kids,
			Name:  f.Name,
			Short: f.Short(),
			Self:  f.Self,
			Total: f.Total,
			Team:  f.Team(),
		})
	}

	for _, s := range r.Timeline {
		p.Timeline = append(p.Timeline, pageSlice{Frame: s.Frame, Samples: s.Samples})
	}

	blob, err := json.Marshal(p)
	if err != nil {
		return template.JS("null")
	}
	return template.JS(blob)
}

// safeName keeps an OpMode's name usable as a file name.
func safeName(name string) string {
	if name == "" {
		return "run"
	}

	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}
