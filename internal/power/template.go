package power

// Deliberately the path visualiser's stylesheet, down to the variables. Two
// pages from the same tool that look like two different tools make somebody
// wonder which one to trust.
const powerPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.OpMode}} - pusher power</title>
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
  .card .v { font-size: 22px; font-weight: 600; margin-top: 2px; }
  .card .v small { font-size: 13px; font-weight: 400; color: var(--muted); }
  svg { width: 100%; height: auto; background: var(--panel);
        border: 1px solid var(--line); border-radius: 10px; }
  .legend { display: flex; flex-wrap: wrap; gap: 14px; margin-top: 10px;
            color: var(--muted); font-size: 12px; }
  .legend span.key { display: inline-flex; align-items: center; gap: 6px; }
  .swatch { width: 11px; height: 11px; border-radius: 3px; display: inline-block; }
  table { border-collapse: collapse; width: 100%; font-size: 13px; }
  th, td { text-align: left; padding: 7px 9px; border-bottom: 1px solid var(--line);
           white-space: nowrap; }
  th { color: var(--muted); font-weight: 600; font-size: 11px;
       text-transform: uppercase; letter-spacing: .04em; }
  td.num { text-align: right; font-variant-numeric: tabular-nums; }
  .bar { position: relative; min-width: 160px; }
  .bar i { display: block; height: 9px; border-radius: 5px; }
  .warn { background: rgba(255,86,48,.09); }
  .tag { display: inline-block; padding: 1px 7px; border-radius: 20px; font-size: 11px;
         background: var(--line); color: var(--muted); }
  .note { background: var(--panel); border: 1px solid var(--line); border-left: 3px solid #FF5630;
          border-radius: 8px; padding: 12px 16px; margin-bottom: 20px; }
  footer { margin-top: 28px; color: var(--muted); font-size: 12px; }
  .legend span.key { cursor: pointer; user-select: none; }
  .legend span.key.off { opacity: .35; text-decoration: line-through; }
  .hint { color: var(--muted); font-size: 12px; margin-top: 6px; }
  .readout { font-size: 12px; color: var(--muted); margin-top: 8px; min-height: 20px;
             font-variant-numeric: tabular-nums; }
  .readout b { color: var(--fg); font-weight: 600; }
  #plot .band { fill: var(--accent); fill-opacity: .16; }
</style>
</head>
<body>
<div class="wrap">
  <h1>{{if .Problem}}Nothing was recorded{{else}}{{.OpMode}}{{end}}</h1>
  <div class="sub">{{.Generated}}</div>

{{if .Problem}}
  <div class="note">{{.Note}}</div>
  <footer>
    The monitor writes one of these when it cannot measure anything, so that the
    reason is on the robot rather than in a log that has already scrolled away.
  </footer>
{{else}}

  <div class="cards">
    <div class="card"><div class="k">Ran for</div>
      <div class="v">{{.Seconds}}<small> s</small></div></div>
    {{if .WorstName}}
    <div class="card"><div class="k">Biggest peak</div>
      <div class="v" style="font-size:16px">{{.WorstName}}
        <small>{{.WorstPeak}} A</small></div></div>
    {{end}}
    {{if .Hungriest}}
    <div class="card"><div class="k">Most charge used</div>
      <div class="v" style="font-size:16px">{{.Hungriest}}</div></div>
    {{end}}
    {{if .TotalPeak}}
    <div class="card"><div class="k">All motors, peak</div>
      <div class="v">{{.TotalPeak}}<small> A</small></div></div>
    {{end}}
    <div class="card"><div class="k">All motors, average</div>
      <div class="v">{{.TotalMean}}<small> A</small></div></div>
    {{if .HasBattery}}
    <div class="card"><div class="k">Battery sag</div>
      <div class="v">{{.Sag}}<small> V</small></div></div>
    {{end}}
  </div>

  <h2>{{.Current.Title}}</h2>
  {{if .Current.Empty}}
    <div class="note">This recording has no readings to draw. It was written by an
      older monitor, which kept only the totals.</div>
  {{else}}
  <div id="chart">
    <svg id="plot" viewBox="{{.Current.ViewBox}}"></svg>
    <div class="readout" id="readout"></div>
    <div class="legend" id="legend"></div>
    <div class="hint">Drag across the graph to zoom in, like an oscilloscope.
      Double-click to zoom back out. Click a name to hide it.</div>
  </div>
  {{end}}

  {{if not .Battery.Empty}}
  <h2>{{.Battery.Title}}</h2>
  <svg id="battery" viewBox="{{.Battery.ViewBox}}" data-h="{{.Battery.Height}}">
    {{range .Battery.YAxis}}
      <line x1="0" y1="{{printf "%.1f" .Pos}}" x2="{{$.Battery.Width}}" y2="{{printf "%.1f" .Pos}}"
            stroke="#8894a3" stroke-opacity=".18" stroke-width="1"/>
      <text x="-8" y="{{printf "%.1f" .Pos}}" dy="4" text-anchor="end" font-size="11" fill="#8894a3">{{.Label}}</text>
    {{end}}
    {{range .Battery.XAxis}}
      <text x="{{printf "%.1f" .Pos}}" y="{{$.Battery.Height}}" dy="16" text-anchor="middle"
            font-size="11" fill="#8894a3">{{.Label}}</text>
    {{end}}
    {{range .Battery.Lines}}
      <path d="{{.Path}}" fill="none" stroke="{{.Colour}}" stroke-width="2"
            stroke-linejoin="round"/>
    {{end}}
  </svg>
  <div class="legend"><span>Dropped from {{.PeakVolts}} V to {{.LowVolts}} V under load.</span></div>
  {{end}}

  <h2>Per motor</h2>
  <table>
    <thead><tr>
      <th>Motor</th><th class="bar">Peak</th>
      <th class="num">Peak</th><th class="num">Average</th>
      <th class="num">Charge</th><th class="num">Share</th><th class="num">Peak at</th>
    </tr></thead>
    <tbody>
    {{range .Bars}}
      <tr{{if .Failed}} class="warn"{{end}}>
        <td>{{.Name}}{{if .Failed}} <span class="tag">{{.Failed}} reads failed</span>{{end}}</td>
        <td class="bar"><i style="width:{{printf "%.1f" .Bar}}%;background:{{.Colour}}"></i></td>
        <td class="num">{{.Peak}} A</td>
        <td class="num">{{.Mean}} A</td>
        <td class="num">{{if .Charge}}{{.Charge}} As{{else}}-{{end}}</td>
        <td class="num">{{if .Charge}}{{printf "%.0f" .Share}}%{{else}}-{{end}}</td>
        <td class="num">{{.PeakAt}} s</td>
      </tr>
    {{end}}
    </tbody>
  </table>

  {{if .Cameras}}
  <h2>Cameras</h2>
  <table>
    <thead><tr><th>Camera</th><th class="num">Connected</th><th class="num">FPS</th><th class="num">CPU</th></tr></thead>
    <tbody>
    {{range .Cameras}}
      <tr><td>{{.Name}}</td>
        <td class="num">{{printf "%.0f" .Connected}}%</td>
        <td class="num">{{printf "%.1f" .FPS}}</td>
        <td class="num">{{printf "%.0f" .CPU}}%</td></tr>
    {{end}}
    </tbody>
  </table>
  <div class="hint">Not a current reading. A Limelight is not on a rail the hub
    measures, so what it draws is inside the hub's total and nowhere else; this
    is how hard it was working, which is the nearest thing there is.</div>
  {{end}}

  {{if .Hubs}}
  <h2>Hubs and buses</h2>
  <table>
    <thead><tr><th>Hub</th><th class="num">Peak</th><th class="num">Average</th></tr></thead>
    <tbody>
    {{range .Hubs}}
      <tr><td>{{.Name}}</td><td class="num">{{.Peak}} A</td><td class="num">{{.Mean}} A</td></tr>
    {{end}}
    </tbody>
  </table>
  <div class="hint">A hub's own reading is everything plugged into it.
    <b>:servos</b> is the whole servo bus and <b>:i2c</b> is everything on I2C,
    because neither servos nor I2C sensors have current sensing of their own:
    one number each is all the hardware can say.</div>
  {{end}}

  <footer>
    Sampled every {{.Period}} ms, {{.SampleCount}} readings.
    {{if .Dropped}}The run was longer than the monitor keeps, so {{.Dropped}} later
    readings are missing from the graphs; the totals still cover all of it.{{end}}
    Peaks shorter than the sampling interval are not in here at all.
    <br><br>
    <b>Charge</b> is current added up over time, in amp-seconds, and is usually the
    more useful number: a motor that pulls 20 A for a tenth of a second costs the
    battery far less than one sitting at 4 A all match. <b>Peak</b> is what trips
    breakers and sags the battery.
    <br><br>
    A hub's reading is everything plugged into it, which is why hubs are listed
    apart from the motors rather than ranked beside them.
    <br><br>
    This costs loop time and is not for use in an official match: a motor's
    current cannot be read in a bulk transfer, so every reading here was its own
    round trip over the bus.
  </footer>
{{end}}
</div>
{{if .Plot}}
<script>
// Everything the graph needs, so nothing is fetched: this opens on a laptop
// that may be sitting on a robot's Wi-Fi with no route to anywhere.
const DATA = {{.Plot}};

const W = 900, H = 260, BH = 160;
const svg = document.getElementById("plot");
const battery = document.getElementById("battery");
const legend = document.getElementById("legend");
const readout = document.getElementById("readout");

const hidden = new Set();
let lo = 0, hi = DATA.t.length - 1;
let drag = null;

const ns = "http://www.w3.org/2000/svg";
const el = (name, attrs) => {
  const e = document.createElementNS(ns, name);
  for (const k in attrs) e.setAttribute(k, attrs[k]);
  return e;
};

const nice = v => {
  if (v <= 0) return 1;
  const step = Math.pow(10, Math.floor(Math.log10(v))) / 2;
  return Math.ceil(v / step) * step;
};

const shown = i => !hidden.has(DATA.names[i]);

function peakIn(from, to) {
  let top = 0;
  for (let i = 0; i < DATA.names.length; i++) {
    if (!shown(i)) continue;
    for (let j = from; j <= to; j++) if (DATA.a[i][j] > top) top = DATA.a[i][j];
  }
  if (!hidden.has("all motors")) {
    for (let j = from; j <= to; j++) if (DATA.total[j] > top) top = DATA.total[j];
  }
  return nice(top || 1);
}

function pathOf(values, from, to, top, height) {
  const span = Math.max(1, to - from);
  let d = "";
  for (let j = from; j <= to; j++) {
    const x = (j - from) / span * W;
    const y = height - Math.min(values[j], top) / top * height;
    d += (j === from ? "M" : " L") + x.toFixed(1) + " " + y.toFixed(1);
  }
  return d;
}

function draw() {
  const from = Math.round(lo), to = Math.round(hi);
  const top = peakIn(from, to);
  const t0 = DATA.t[from], t1 = DATA.t[to];

  svg.textContent = "";

  for (let k = 0; k <= 4; k++) {
    const y = H - k / 4 * H;
    svg.appendChild(el("line", {x1: 0, y1: y, x2: W, y2: y,
      stroke: "#8894a3", "stroke-opacity": ".18", "stroke-width": 1}));

    const label = el("text", {x: -8, y: y, dy: 4, "text-anchor": "end",
      "font-size": 11, fill: "#8894a3"});
    label.textContent = (top * k / 4).toFixed(1) + "A";
    svg.appendChild(label);
  }

  for (let k = 0; k <= 4; k++) {
    const label = el("text", {x: k / 4 * W, y: H, dy: 16, "text-anchor": "middle",
      "font-size": 11, fill: "#8894a3"});
    label.textContent = (t0 + (t1 - t0) * k / 4).toFixed(1) + "s";
    svg.appendChild(label);
  }

  if (!hidden.has("all motors")) {
    svg.appendChild(el("path", {d: pathOf(DATA.total, from, to, top, H), fill: "none",
      stroke: "#8894a3", "stroke-width": 2, "stroke-dasharray": "4 3", opacity: .75}));
  }

  for (let i = 0; i < DATA.names.length; i++) {
    if (!shown(i)) continue;
    svg.appendChild(el("path", {d: pathOf(DATA.a[i], from, to, top, H), fill: "none",
      stroke: DATA.colours[i], "stroke-width": 2, "stroke-linejoin": "round"}));
  }

  if (drag) {
    const x0 = Math.min(drag.a, drag.b), x1 = Math.max(drag.a, drag.b);
    svg.appendChild(el("rect", {class: "band", x: x0, y: 0, width: x1 - x0, height: H}));
  }

  drawBattery(from, to);
}

// The battery follows the same window, because the question after seeing a
// spike is always what the battery did at that moment.
function drawBattery(from, to) {
  if (!battery || !DATA.v.length) return;

  let low = Infinity, high = 0;
  for (let j = from; j <= to; j++) {
    const v = DATA.v[j];
    if (v <= 0) continue;
    if (v < low) low = v;
    if (v > high) high = v;
  }
  if (!isFinite(low) || high <= 0) return;

  const floor = Math.floor((low - 0.3) * 2) / 2, ceil = Math.ceil((high + 0.3) * 2) / 2;
  const span = Math.max(1, to - from);

  battery.textContent = "";

  for (let k = 0; k <= 4; k++) {
    const y = BH - k / 4 * BH;
    battery.appendChild(el("line", {x1: 0, y1: y, x2: W, y2: y,
      stroke: "#8894a3", "stroke-opacity": ".18", "stroke-width": 1}));

    const label = el("text", {x: -8, y: y, dy: 4, "text-anchor": "end",
      "font-size": 11, fill: "#8894a3"});
    label.textContent = (floor + (ceil - floor) * k / 4).toFixed(1) + "V";
    battery.appendChild(label);
  }

  for (let k = 0; k <= 4; k++) {
    const label = el("text", {x: k / 4 * W, y: BH, dy: 16, "text-anchor": "middle",
      "font-size": 11, fill: "#8894a3"});
    label.textContent = (DATA.t[from] + (DATA.t[to] - DATA.t[from]) * k / 4).toFixed(1) + "s";
    battery.appendChild(label);
  }

  let d = "";
  for (let j = from; j <= to; j++) {
    const x = (j - from) / span * W;
    const y = BH - (DATA.v[j] - floor) / (ceil - floor) * BH;
    d += (j === from ? "M" : " L") + x.toFixed(1) + " " + Math.max(0, Math.min(BH, y)).toFixed(1);
  }
  battery.appendChild(el("path", {d: d, fill: "none", stroke: "#FFAB00", "stroke-width": 2}));
}

function buildLegend() {
  const entries = [{name: "all motors", colour: "#8894a3"}];
  for (let i = 0; i < DATA.names.length; i++) {
    entries.push({name: DATA.names[i], colour: DATA.colours[i]});
  }

  for (const entry of entries) {
    const span = document.createElement("span");
    span.className = "key";
    span.innerHTML = '<i class="swatch" style="background:' + entry.colour + '"></i>';
    span.appendChild(document.createTextNode(entry.name));

    span.onclick = () => {
      hidden.has(entry.name) ? hidden.delete(entry.name) : hidden.add(entry.name);
      span.classList.toggle("off", hidden.has(entry.name));
      draw();
    };

    legend.appendChild(span);
  }
}

// Where in the data a pixel is. The viewBox has a margin on the left, so this
// has to go through the SVG's own coordinates rather than the element's.
function atPointer(event) {
  const box = svg.getBoundingClientRect();
  const scale = (W + 78) / box.width;
  return (event.clientX - box.left) * scale - 46;
}

svg.addEventListener("mousedown", e => {
  drag = {a: atPointer(e), b: atPointer(e)};
  draw();
});

svg.addEventListener("mousemove", e => {
  const x = atPointer(e);

  if (drag) {
    drag.b = x;
    draw();
    return;
  }

  const from = Math.round(lo), to = Math.round(hi);
  const j = Math.max(from, Math.min(to, from + Math.round(x / W * (to - from))));
  if (!DATA.t[j]) return;

  let html = "<b>" + DATA.t[j].toFixed(1) + "s</b>";
  if (!hidden.has("all motors")) html += " · all motors <b>" + DATA.total[j].toFixed(2) + "A</b>";
  for (let i = 0; i < DATA.names.length; i++) {
    if (!shown(i)) continue;
    html += " · " + DATA.names[i] + " <b>" + DATA.a[i][j].toFixed(2) + "A</b>";
  }
  if (DATA.v[j] > 0) html += " · battery <b>" + DATA.v[j].toFixed(2) + "V</b>";
  readout.innerHTML = html;
});

window.addEventListener("mouseup", () => {
  if (!drag) return;

  const from = Math.round(lo), to = Math.round(hi);
  const a = Math.min(drag.a, drag.b), b = Math.max(drag.a, drag.b);
  drag = null;

  // A click rather than a drag is not a zoom to nothing.
  if (b - a > 6) {
    const span = to - from;
    const next = [from + a / W * span, from + b / W * span];
    if (next[1] - next[0] >= 2) { lo = Math.max(0, next[0]); hi = Math.min(DATA.t.length - 1, next[1]); }
  }
  draw();
});

svg.addEventListener("dblclick", () => {
  lo = 0;
  hi = DATA.t.length - 1;
  draw();
});

svg.addEventListener("mouseleave", () => { readout.innerHTML = ""; });

buildLegend();
draw();
</script>
{{end}}
</body>
</html>
`
