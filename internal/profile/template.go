package profile

// Deliberately the same stylesheet as the power readings and the path
// visualiser, down to the variables. Three pages from one tool that look like
// three different tools make somebody wonder which of them to trust.
const profilePage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.OpMode}} - pusher profile</title>
<style>
  :root {
    --bg: #ffffff; --fg: #1b1f24; --muted: #6b7684; --line: #e3e6ea;
    --panel: #f7f8fa; --accent: #4C9AFF;
  }
  @media (prefers-color-scheme: dark) {
    :root { --bg: #14171a; --fg: #e6e9ec; --muted: #98a2ad; --line: #2a2f36;
            --panel: #1b1f24; --accent: #6BB0FF; }
  }
  * { box-sizing: border-box; }
  body { margin: 0; padding: 24px; background: var(--bg); color: var(--fg);
         font: 14px/1.5 ui-sans-serif, -apple-system, "Segoe UI", Roboto, sans-serif; }
  .wrap { max-width: 1180px; margin: 0 auto; }
  h1 { font-size: 20px; margin: 0 0 2px; }
  h2 { font-size: 13px; text-transform: uppercase; letter-spacing: .04em;
       color: var(--muted); margin: 24px 0 8px; }
  .sub { color: var(--muted); margin-bottom: 20px; }
  .cards { display: flex; flex-wrap: wrap; gap: 12px; margin-bottom: 20px; }
  .card { background: var(--panel); border: 1px solid var(--line); border-radius: 10px;
          padding: 12px 16px; min-width: 150px; flex: 1; }
  .card .k { color: var(--muted); font-size: 12px; text-transform: uppercase;
             letter-spacing: .04em; }
  .card .v { font-size: 22px; font-weight: 600; margin-top: 2px;
             overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .card .v small { font-size: 13px; font-weight: 400; color: var(--muted); }
  .panel { background: var(--panel); border: 1px solid var(--line);
           border-radius: 10px; padding: 10px; }
  .note { color: var(--muted); font-size: 12px; margin-top: 10px; }
  .warn { border-left: 3px solid #E2B203; padding-left: 10px; margin: 12px 0;
          color: var(--muted); font-size: 13px; }

  /* The chart is rows of bars, each positioned by percentage, so zooming is
     arithmetic on two numbers rather than a redraw of anything.

     Built downwards and drawn upwards. A flame graph grows from its root, so
     the widest bars are at the bottom and the stack rises off them: the code
     that was actually executing ends up along the top, which is where the eye
     goes. Drawn the other way up it is an icicle chart, which is a different
     convention and reads as upside down to anybody who has seen a flame graph
     before. */
  #flame { position: relative; overflow-x: hidden;
           display: flex; flex-direction: column-reverse; }
  .row { position: relative; height: 21px; }
  .bar { position: absolute; top: 0; height: 20px; border-radius: 3px;
         font: 11px/20px ui-monospace, SFMono-Regular, Menlo, monospace;
         padding: 0 5px; overflow: hidden; white-space: nowrap;
         cursor: pointer; color: #1b1f24; border: 1px solid rgba(0,0,0,.16); }
  .bar:hover { filter: brightness(1.12); }
  .bar.dim { opacity: .38; }

  #strip { display: flex; height: 26px; border-radius: 6px; overflow: hidden;
           border: 1px solid var(--line); margin-top: 6px; }
  #strip div { height: 100%; }

  table { border-collapse: collapse; width: 100%; margin-top: 8px; }
  th, td { text-align: right; padding: 7px 10px; border-bottom: 1px solid var(--line); }
  th:first-child, td:first-child { text-align: left; }
  th { color: var(--muted); font-size: 12px; text-transform: uppercase;
       letter-spacing: .04em; font-weight: 600; }
  td.name { font: 12px/1.5 ui-monospace, SFMono-Regular, Menlo, monospace;
            word-break: break-all; }
  tr.team td { background: rgba(76,154,255,.09); }
  .crumbs { color: var(--muted); font-size: 12px; margin-bottom: 8px;
            font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
  .crumbs a { color: var(--accent); cursor: pointer; text-decoration: none; }
  footer { color: var(--muted); font-size: 12px; margin-top: 28px;
           border-top: 1px solid var(--line); padding-top: 12px; }
</style>
</head>
<body>
<div class="wrap">

  <h1>{{.OpMode}}</h1>
  <div class="sub">{{.Sub}}</div>

{{if .Problem}}
  <div class="panel">{{.Problem}}</div>
  <footer>
    The profiler is installed and attached, but it wrote this instead of a
    recording. It leaves a note rather than an empty directory, because the
    robot's log is a ring buffer and the line explaining it is usually gone by
    the time anybody looks.
  </footer>
{{else}}

  <div class="cards">
    <div class="card"><div class="k">Samples</div><div class="v">{{.Samples}}</div></div>
    <div class="card"><div class="k">Run</div><div class="v">{{.Duration}} <small>s</small></div></div>
    <div class="card"><div class="k">Every</div><div class="v">{{.Period}} <small>ms</small></div></div>
    <div class="card"><div class="k">Covered</div><div class="v">{{.Coverage}}<small>%</small></div></div>
    <div class="card"><div class="k">Hottest</div><div class="v" title="{{.Hottest}}">{{.Hottest}} <small>{{.HotTime}}s</small></div></div>
  </div>

{{if .Warning}}
  <div class="warn">{{.Warning}}</div>
{{end}}

  <h2>Flame chart</h2>
  <div class="crumbs" id="crumbs"></div>
  <div class="panel"><div id="flame"></div></div>
  <div class="note">
    Each bar is a method, as wide as the share of the run spent inside it and
    everything it called. It grows upwards from the whole run at the bottom, so
    a bar sits on top of the one that called it. Click one to zoom into it,
    click the trail above to come back out. Your own code is the blue bars.
    Width is time in the method <em>or above it</em>: a wide bar with nothing
    on top of it is where the time actually went.
  </div>

  <h2>What was running, over time</h2>
  <div id="strip"></div>
  <div class="note">
    The run left to right, coloured by whatever was on top at that moment. A
    band that repeats is a loop; one wide band is something that blocked.
  </div>

  <h2>Where the time went</h2>
  <table>
    <thead><tr><th>Method</th><th>Share</th><th>Seconds</th></tr></thead>
    <tbody>
    {{range .Rows}}
      <tr{{if .Team}} class="team"{{end}}>
        <td class="name">{{.Name}}</td>
        <td>{{.Share}}%</td>
        <td>{{.Seconds}}</td>
      </tr>
    {{end}}
    </tbody>
  </table>
  <div class="note">
    Ranked by time spent <em>in</em> the method rather than under it, so this is
    the code that was actually executing rather than a list of everything that
    called it.
  </div>

  <footer>
    Time is counted in samples: a method in half of them was running for half
    the run. Sampling stops the OpMode's thread for as long as it takes to walk
    its stack, so this costs loop time and is not for use in a match. Turn it
    off in <code>pusher settings</code> when you are done.
  </footer>

<script>
const PLOT = {{.Plot}};

const byId = new Map();
for (const f of PLOT.frames) byId.set(f.i, f);

// Warm colours, spread by name so a method keeps its colour as you zoom, and
// blue for the team's own code so it stands out of the SDK around it.
function colourFor(f) {
  if (f.m) return "hsl(212 90% 68%)";
  let h = 0;
  for (let i = 0; i < f.n.length; i++) h = (h * 31 + f.n.charCodeAt(i)) | 0;
  return "hsl(" + (18 + (Math.abs(h) % 42)) + " 78% " + (58 + (Math.abs(h >> 8) % 14)) + "%)";
}

let trail = [0];

function draw() {
  const rootId = trail[trail.length - 1];
  const root = byId.get(rootId);
  const flame = document.getElementById("flame");
  flame.innerHTML = "";

  if (!root || !root.t) {
    flame.innerHTML = '<div class="note">No samples in this branch.</div>';
    return;
  }

  const rows = [];
  (function place(id, left, width, depth) {
    const f = byId.get(id);
    if (!f || width <= 0) return;

    while (rows.length <= depth) {
      const row = document.createElement("div");
      row.className = "row";
      flame.appendChild(row);
      rows.push(row);
    }

    const bar = document.createElement("div");
    bar.className = "bar";
    bar.style.left = left + "%";
    bar.style.width = width + "%";
    bar.style.background = colourFor(f);

    const secs = (f.t * PLOT.periodMs / 1000);
    const share = PLOT.samples ? (f.t / PLOT.samples * 100) : 0;
    bar.title = f.n + "\n" + secs.toFixed(2) + "s of the run (" + share.toFixed(1) + "%)"
      + "\n" + (f.f * PLOT.periodMs / 1000).toFixed(2) + "s in this method itself";
    bar.textContent = f.s;
    bar.onclick = () => { trail.push(id); draw(); };

    rows[depth].appendChild(bar);

    let at = left;
    for (const kid of (f.k || [])) {
      const k = byId.get(kid);
      if (!k) continue;
      const w = width * k.t / f.t;
      place(kid, at, w, depth + 1);
      at += w;
    }
  })(rootId, 0, 100, 0);

  const crumbs = document.getElementById("crumbs");
  crumbs.innerHTML = "";
  trail.forEach((id, i) => {
    if (i) crumbs.appendChild(document.createTextNode("  ›  "));
    const a = document.createElement("a");
    a.textContent = i === 0 ? "whole run" : byId.get(id).s;
    a.onclick = () => { trail = trail.slice(0, i + 1); draw(); };
    crumbs.appendChild(a);
  });
}

function strip() {
  const el = document.getElementById("strip");
  const total = PLOT.timeline.reduce((n, s) => n + s.n, 0) || 1;

  for (const s of PLOT.timeline) {
    const f = byId.get(s.f);
    const d = document.createElement("div");
    d.style.width = (s.n / total * 100) + "%";
    d.style.background = f ? colourFor(f) : "var(--line)";
    if (f) {
      d.title = f.n + "\n" + (s.n * PLOT.periodMs / 1000).toFixed(2) + "s without a break";
      d.onclick = () => {
        const path = [];
        for (let at = f; at; at = byId.get(at.p)) path.unshift(at.i);
        trail = [0, s.f];
        draw();
        document.getElementById("flame").scrollIntoView({behavior: "smooth"});
      };
      d.style.cursor = "pointer";
    }
    el.appendChild(d);
  }
}

draw();
strip();
</script>

{{end}}
</div>
</body>
</html>
`
