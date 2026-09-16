package follower

const followerPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}}</title>
<style>
  :root {
    --bg: #ffffff; --fg: #1b1f24; --muted: #6b7684; --line: #e3e6ea;
    --panel: #f7f8fa; --grid: rgba(136,148,163,.22); --warn: #FF5630;
  }
  @media (prefers-color-scheme: dark) {
    :root { --bg: #14171a; --fg: #e6e9ec; --muted: #98a2ad; --line: #2a2f36;
            --panel: #1b1f24; --grid: rgba(136,148,163,.20); --warn: #FF7452; }
  }
  * { box-sizing: border-box; }
  body { margin: 0; padding: 24px; background: var(--bg); color: var(--fg);
         font: 14px/1.5 ui-sans-serif, -apple-system, "Segoe UI", Roboto, sans-serif; }
  .wrap { max-width: 1180px; margin: 0 auto; }
  h1 { font-size: 20px; margin: 0 0 2px; }
  .sub { color: var(--muted); margin-bottom: 18px; }
  .notice { background: var(--panel); border: 1px solid var(--line);
            border-left: 3px solid var(--warn); border-radius: 8px;
            padding: 10px 14px; margin-bottom: 10px; }
  .cards { display: flex; flex-wrap: wrap; gap: 10px; margin: 18px 0 8px; }
  .card { background: var(--panel); border: 1px solid var(--line); border-radius: 10px;
          padding: 10px 14px; min-width: 148px; flex: 1; }
  .card.warn { border-color: var(--warn); }
  .card .k { color: var(--muted); font-size: 11px; text-transform: uppercase;
             letter-spacing: .04em; }
  .card .v { font-size: 21px; font-weight: 600; margin-top: 1px;
             font-variant-numeric: tabular-nums; }
  .card .v small { font-size: 12px; font-weight: 400; color: var(--muted); }
  .card .s { color: var(--muted); font-size: 11px; }
  .bar { display: flex; align-items: center; gap: 10px; margin: 18px 0 6px;
         color: var(--muted); font-size: 12px; }
  .bar button { font: inherit; color: var(--fg); background: var(--panel);
                border: 1px solid var(--line); border-radius: 6px; padding: 3px 10px;
                cursor: pointer; }
  .chart { margin-bottom: 14px; }
  .head { display: flex; justify-content: space-between; align-items: baseline;
          gap: 12px; margin-bottom: 2px; }
  .head .t { font-weight: 600; font-size: 13px; }
  .head .keys { color: var(--muted); font-size: 11px; }
  .head .keys i { font-style: normal; padding-left: 10px; }
  .head .keys i::before { content: "\25A0  "; }
  .read { color: var(--muted); font-size: 12px; font-variant-numeric: tabular-nums;
          white-space: nowrap; }
  .plot { position: relative; height: 132px; background: var(--panel);
          border: 1px solid var(--line); border-radius: 8px; overflow: hidden;
          cursor: crosshair; }
  .plot canvas { display: block; width: 100%; height: 100%; }
  .cursor { position: absolute; top: 0; bottom: 0; width: 1px;
            background: var(--fg); opacity: .35; display: none; pointer-events: none; }
  .sel { position: absolute; top: 0; bottom: 0; background: rgba(76,154,255,.18);
         border-left: 1px solid #4C9AFF; border-right: 1px solid #4C9AFF;
         display: none; pointer-events: none; }
  .why { color: var(--muted); font-size: 11.5px; margin-top: 3px; }
  footer { margin-top: 28px; color: var(--muted); font-size: 12px; }
  footer p { margin: 0 0 8px; }
  code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
</style>
</head>
<body>
<div class="wrap">
  <h1>{{.Title}}</h1>
  <div class="sub">{{.Sub}}</div>

  {{range .Notices}}<div class="notice">{{.}}</div>{{end}}
  {{if .Extra}}<div class="notice">{{.Extra}}</div>{{end}}

  <div class="cards">
    {{range .Cards}}
    <div class="card{{if .Warn}} warn{{end}}">
      <div class="k">{{.Key}}</div>
      <div class="v">{{.Value}}{{if .Unit}} <small>{{.Unit}}</small>{{end}}</div>
      <div class="s">{{.Sub}}</div>
    </div>
    {{end}}
  </div>

  <div class="bar">
    <button id="reset">Whole run</button>
    <span id="window"></span>
    <span>Drag across any plot to zoom every plot. Double click to come back out.</span>
  </div>

  <div id="charts"></div>

  <footer>
    <p><b>Reference against measurement</b> is what the whole log is for. A gap that
    stays open means the loop cannot keep up, so either the constants are wrong or the
    closed loop time constant is asking for more than the pack can supply. A gap that
    oscillates means that constant is too small for the rate this loop actually ran at.
    Nothing else tells those two apart.</p>

    <p>The gap figures in the panel are measured only while the follower is commanding.
    On a settled loop it commands nothing and every reference is written zero, so a
    robot still coasting would otherwise read as an enormous tracking error.</p>

    <p><b>Heading error</b> is drawn in the field's convention, CCW positive, which is
    the negative of the heading_error column. blob stores that column mirrored, where a
    positive error asks for a clockwise turn, and comparing it against omega without
    negating it makes a robot that is turning correctly look like one turning the wrong
    way. That has caused real bugs inside blob.</p>

    <p><b>Delivered</b> is recovered from the four logged wheel commands by running
    blob's mix backwards. On loops where nothing saturated it reproduces the volts
    exactly; where it does not, the mix could not deliver what the controller asked for,
    and the shaded bands are those loops. While following a path blob keeps the turn and
    scales translation down, so a turn command that is short is a sign the follower was
    not following a path.</p>

    <p>The file is rewritten whole every couple of seconds while the run is going and
    carries no end marker, so a log pulled mid run is complete, valid and shorter than
    the run it came from. Pull it again for the rest.</p>
  </footer>
</div>

<script>
var D = {{.Plot}};
var N = D.t.length;
var lo = 0, hi = N - 1;

var ASK = "#FF5630", GOT = "#4C9AFF", GIVEN = "#36B37E", FEED = "#8894a3";
var WHEEL = ["#4C9AFF", "#36B37E", "#FFAB00", "#FF5630"];

var CHARTS = [
  { id: "f", title: "Forward: asked against measured", unit: "in/s",
    why: "The axis the path spends most of its time on.",
    lines: [ {k: "ref_f", c: ASK, l: "ref_f asked", w: 2.6}, {k: "vf", c: GOT, l: "vf measured", w: 1.2} ] },

  { id: "l", title: "Left: asked against measured", unit: "in/s",
    why: "Same frame and signs as forward, so the two are read the same way.",
    lines: [ {k: "ref_l", c: ASK, l: "ref_l asked", w: 2.6}, {k: "vl", c: GOT, l: "vl measured", w: 1.2} ] },

  { id: "a", title: "Turn: asked against measured", unit: "rad/s",
    why: "CCW positive, both of them.",
    lines: [ {k: "ref_omega", c: ASK, l: "ref_omega asked", w: 2.6}, {k: "omega", c: GOT, l: "omega measured", w: 1.2} ] },

  { id: "t", title: "Turn command: asked against delivered", unit: "V",
    why: "Shaded where the mix saturated. A heading loop starved of volts looks identical to a badly tuned one, and nothing else separates them.",
    bands: "sat", banddesc: "mix saturated",
    lines: [ {k: "volts_t", c: ASK, l: "volts_t asked", w: 2.6}, {k: "deliv_t", c: GIVEN, l: "delivered", w: 1.2} ] },

  { id: "vf", title: "Forward volts, against the pack", unit: "V",
    why: "The filled area is the kS feedforward. What is left above it is the controller doing the work.",
    battery: true, fill: "static_f",
    lines: [ {k: "volts_f", c: ASK, l: "volts_f"}, {k: "static_f", c: FEED, l: "static_f"} ] },

  { id: "vl", title: "Left volts, against the pack", unit: "V",
    battery: true, fill: "static_l",
    lines: [ {k: "volts_l", c: ASK, l: "volts_l"}, {k: "static_l", c: FEED, l: "static_l"} ] },

  { id: "vt", title: "Turn volts, against the pack", unit: "V",
    battery: true, fill: "static_t",
    lines: [ {k: "volts_t", c: ASK, l: "volts_t"}, {k: "static_t", c: FEED, l: "static_t"} ] },

  { id: "w", title: "What reached the motors", unit: "power",
    why: "After mixing and desaturation, with full scale marked. A persistent asymmetry here points at one motor rather than at the controller.",
    guides: [1, -1],
    lines: [ {k: "lf", c: WHEEL[0], l: "lf"}, {k: "lb", c: WHEEL[1], l: "lb"},
             {k: "rf", c: WHEEL[2], l: "rf"}, {k: "rb", c: WHEEL[3], l: "rb"} ] },

  { id: "h", title: "Heading error, field convention", unit: "deg",
    why: "Shaded where the follower was settled. Entering and leaving the settle band repeatedly is one shape a wobble takes, and it is invisible in a path trace.",
    bands: "settled", banddesc: "settled",
    guides: [0],
    lines: [ {k: "herr", c: GOT, l: "field heading error"} ] }
];

var PAD = {l: 54, r: 10, t: 8, b: 16};

function css(name) {
  return getComputedStyle(document.body).getPropertyValue(name).trim();
}

function fmt(v, unit) {
  var abs = Math.abs(v);
  var digits = abs >= 100 ? 0 : abs >= 10 ? 1 : abs >= 1 ? 2 : 3;
  return v.toFixed(digits) + " " + unit;
}

function build() {
  var host = document.getElementById("charts");

  CHARTS.forEach(function (ch) {
    var box = document.createElement("div");
    box.className = "chart";

    var keys = ch.lines.map(function (s) {
      return "<i style=\"color:" + s.c + "\">" + s.l + "</i>";
    }).join("");
    if (ch.bands) keys += "<i>" + ch.banddesc + " shaded</i>";

    box.innerHTML =
      "<div class=\"head\"><span class=\"t\">" + ch.title + "</span>" +
      "<span class=\"read\" id=\"read-" + ch.id + "\"></span></div>" +
      "<div class=\"head\"><span class=\"keys\">" + keys + "</span></div>" +
      "<div class=\"plot\"><canvas></canvas><div class=\"cursor\"></div><div class=\"sel\"></div></div>" +
      (ch.why ? "<div class=\"why\">" + ch.why + "</div>" : "");

    host.appendChild(box);

    ch.plot = box.querySelector(".plot");
    ch.canvas = box.querySelector("canvas");
    ch.cursor = box.querySelector(".cursor");
    ch.sel = box.querySelector(".sel");
    ch.read = box.querySelector(".read");

    wire(ch);
  });

  buildHistogram(host);
}

// Every plot shares one window, so a drag on any of them is a drag on all of
// them: the whole point of stacking these is comparing the same instant across
// axes, and a per plot zoom would break exactly that.
function wire(ch) {
  var from = null;

  // Where the pointer is inside this plot, taken from the plot's own box
  // rather than from offsetX. The drag ends on a mouseup anywhere in the
  // window, and offsetX there is measured against whatever element happens to
  // be under the pointer, so a drag that finished past the edge of the plot
  // zoomed to somewhere nobody asked for.
  var localX = function (e) {
    return e.clientX - ch.plot.getBoundingClientRect().left;
  };

  ch.plot.addEventListener("mousedown", function (e) {
    from = localX(e);
    ch.sel.style.display = "block";
    ch.sel.style.left = from + "px";
    ch.sel.style.width = "0px";
    e.preventDefault();
  });

  ch.plot.addEventListener("mousemove", function (e) {
    var x = localX(e);
    if (from !== null) {
      ch.sel.style.left = Math.min(from, x) + "px";
      ch.sel.style.width = Math.abs(x - from) + "px";
    }
    hover(ch, x);
  });

  ch.plot.addEventListener("mouseleave", function () {
    hover(null, 0);
  });

  window.addEventListener("mouseup", function (e) {
    if (from === null) return;

    var x = localX(e);
    var a = Math.min(from, x), b = Math.max(from, x);
    from = null;
    ch.sel.style.display = "none";

    // A click is not a zoom. Without this, every attempt to read a value off a
    // plot would collapse the window to one loop.
    if (b - a < 6) return;

    var w = ch.plot.clientWidth;
    var at = function (x) {
      var f = (x - PAD.l) / Math.max(1, w - PAD.l - PAD.r);
      return Math.round(lo + Math.max(0, Math.min(1, f)) * (hi - lo));
    };

    var na = at(a), nb = at(b);
    if (nb - na < 2) return;

    lo = na; hi = nb;
    drawAll();
  });

  ch.plot.addEventListener("dblclick", function () {
    lo = 0; hi = N - 1;
    drawAll();
  });
}

function hover(active, x) {
  var index = -1;

  if (active) {
    var w = active.plot.clientWidth;
    var f = (x - PAD.l) / Math.max(1, w - PAD.l - PAD.r);
    index = Math.round(lo + Math.max(0, Math.min(1, f)) * (hi - lo));
  }

  CHARTS.forEach(function (ch) {
    if (index < 0) {
      ch.cursor.style.display = "none";
      ch.read.textContent = "";
      return;
    }

    var w = ch.plot.clientWidth;
    var px = PAD.l + (index - lo) / Math.max(1, hi - lo) * (w - PAD.l - PAD.r);

    ch.cursor.style.display = "block";
    ch.cursor.style.left = px + "px";

    var parts = [D.t[index].toFixed(2) + " s"];
    ch.lines.forEach(function (s) {
      parts.push(s.l + " " + fmt(D[s.k][index], ch.unit));
    });
    ch.read.textContent = parts.join("   ");
  });
}

// Per pixel column rather than per loop. At four hundred loops to the pixel a
// polyline through every point is both slow and a lie: it draws whichever loops
// happen to land on a column boundary. Taking the smallest and largest in each
// column keeps the envelope, which is the part that says whether this is
// ringing.
function envelope(values, width) {
  var cols = Math.max(1, Math.round(width));
  var span = hi - lo + 1;
  var out = new Array(cols);

  for (var c = 0; c < cols; c++) {
    var a = lo + Math.floor(c * span / cols);
    var b = lo + Math.floor((c + 1) * span / cols) - 1;
    if (b < a) b = a;
    if (b > hi) b = hi;

    var min = Infinity, max = -Infinity;
    for (var i = a; i <= b; i++) {
      var v = values[i];
      if (v < min) min = v;
      if (v > max) max = v;
    }
    out[c] = [min, max];
  }
  return out;
}

// trace adds one series to the current path, either as the loops themselves or
// as their envelope, depending on how many of them are fighting over each
// pixel.
//
// Zoomed in far enough that every loop has room, the points are what somebody
// is looking at: drawing an envelope there turns a smooth response into a
// staircase of one pixel columns. Zoomed out, the envelope is the only honest
// drawing.
function trace(g, values, y, xOf, pw, sign) {
  var span = hi - lo + 1;

  if (span <= pw) {
    for (var i = lo; i <= hi; i++) {
      var px = xOf(i), py = y(sign * values[i]);
      if (i === lo) g.moveTo(px, py); else g.lineTo(px, py);
    }
    return;
  }

  var e = envelope(values, pw);
  for (var c = 0; c < e.length; c++) {
    var x = PAD.l + c;
    if (c === 0) g.moveTo(x, y(sign * e[c][0]));
    g.lineTo(x, y(sign * e[c][0]));
    g.lineTo(x, y(sign * e[c][1]));
  }
}

function range(ch) {
  var min = Infinity, max = -Infinity;

  var take = function (v) {
    if (v < min) min = v;
    if (v > max) max = v;
  };

  ch.lines.forEach(function (s) {
    var a = D[s.k];
    for (var i = lo; i <= hi; i++) take(a[i]);
  });

  if (ch.guides) ch.guides.forEach(take);

  if (ch.battery) {
    var b = 0;
    for (var j = lo; j <= hi; j++) b = Math.max(b, D.battery[j]);
    take(b); take(-b);
  }

  if (!isFinite(min) || !isFinite(max)) { min = -1; max = 1; }
  if (max - min < 1e-9) { min -= 1; max += 1; }

  var pad = (max - min) * 0.08;
  return [min - pad, max + pad];
}

function ticks(min, max) {
  var step = Math.pow(10, Math.floor(Math.log(Math.max(1e-9, max - min)) / Math.LN10));
  var n = (max - min) / step;
  if (n < 2) step /= 5; else if (n < 5) step /= 2;

  var out = [];
  for (var v = Math.ceil(min / step) * step; v <= max; v += step) {
    // Snapped, because adding a step repeatedly leaves 0.30000000000000004
    // sitting on an axis.
    out.push(Math.round(v / step) * step);
  }
  out.step = step;
  return out;
}

function draw(ch) {
  var c = ch.canvas;
  var w = ch.plot.clientWidth, h = ch.plot.clientHeight;
  var dpr = window.devicePixelRatio || 1;

  c.width = Math.round(w * dpr);
  c.height = Math.round(h * dpr);

  var g = c.getContext("2d");
  g.setTransform(dpr, 0, 0, dpr, 0, 0);
  g.clearRect(0, 0, w, h);

  var pw = w - PAD.l - PAD.r, ph = h - PAD.t - PAD.b;
  var band = range(ch), min = band[0], max = band[1];

  var y = function (v) { return PAD.t + (max - v) / (max - min) * ph; };
  var xOf = function (i) { return PAD.l + (i - lo) / Math.max(1, hi - lo) * pw; };

  // The shaded loops go down first, so everything else is read against them.
  if (ch.bands && D[ch.bands]) {
    g.fillStyle = ch.bands === "sat" ? "rgba(255,86,48,.16)" : "rgba(136,148,163,.20)";
    D[ch.bands].forEach(function (s) {
      if (s[1] < lo || s[0] > hi) return;
      var a = xOf(Math.max(lo, s[0])), b = xOf(Math.min(hi, s[1]));
      g.fillRect(a, PAD.t, Math.max(1, b - a), ph);
    });
  }

  g.strokeStyle = css("--grid");
  g.lineWidth = 1;
  g.fillStyle = css("--muted");
  g.font = "10px ui-sans-serif, sans-serif";
  g.textAlign = "right";
  g.textBaseline = "middle";

  var marks = ticks(min, max);
  var places = marks.step >= 1 ? 0 : marks.step >= 0.1 ? 1 : 2;

  marks.forEach(function (v) {
    var py = Math.round(y(v)) + 0.5;
    g.beginPath();
    g.moveTo(PAD.l, py);
    g.lineTo(w - PAD.r, py);
    g.stroke();
    g.fillText(v.toFixed(places), PAD.l - 6, py);
  });

  // Time, labelled at both ends, because the window moves and a plot that does
  // not say where it is looking is a plot of an unknown five seconds.
  g.textAlign = "left";
  g.textBaseline = "bottom";
  g.fillText(D.t[lo].toFixed(2) + " s", PAD.l, h - 3);
  g.textAlign = "right";
  g.fillText(D.t[hi].toFixed(2) + " s", w - PAD.r, h - 3);

  if (ch.guides) {
    g.strokeStyle = css("--muted");
    g.setLineDash([3, 4]);
    ch.guides.forEach(function (v) {
      var py = Math.round(y(v)) + 0.5;
      g.beginPath();
      g.moveTo(PAD.l, py);
      g.lineTo(w - PAD.r, py);
      g.stroke();
    });
    g.setLineDash([]);
  }

  if (ch.battery) {
    g.strokeStyle = "rgba(255,171,0,.55)";
    g.lineWidth = 1;
    g.setLineDash([2, 3]);
    [1, -1].forEach(function (sign) {
      g.beginPath();
      trace(g, D.battery, y, xOf, pw, sign);
      g.stroke();
    });
    g.setLineDash([]);
  }

  if (ch.fill && D[ch.fill]) {
    g.fillStyle = "rgba(136,148,163,.30)";
    g.beginPath();
    g.moveTo(PAD.l, y(0));
    trace(g, D[ch.fill], y, xOf, pw, 1);
    g.lineTo(PAD.l + pw, y(0));
    g.closePath();
    g.fill();
  }

  ch.lines.forEach(function (s) {
    g.strokeStyle = s.c;
    g.lineWidth = s.w || 1.4;
    g.beginPath();
    trace(g, D[s.k], y, xOf, pw, 1);
    g.stroke();
  });

  g.strokeStyle = css("--line");
  g.strokeRect(PAD.l + 0.5, PAD.t + 0.5, pw - 1, ph - 1);
}

// The loop period gets a histogram rather than a trace, because the question is
// what the distribution looks like: one hump at 17 ms and a second at 40 is a
// robot that stalls somewhere, and a mean of 19 says nothing at all about it.
function buildHistogram(host) {
  var box = document.createElement("div");
  box.className = "chart";
  box.innerHTML =
    "<div class=\"head\"><span class=\"t\">Loop period</span>" +
    "<span class=\"read\">mean " + D.meanDt.toFixed(1) + " ms, worst " +
    D.maxDt.toFixed(1) + " ms, so nothing below tauClosedLoop = " +
    D.minTau.toFixed(3) + " s is usable</span></div>" +
    "<div class=\"plot\" style=\"cursor:default\"><canvas></canvas></div>" +
    "<div class=\"why\">Four times the worst period is the fastest closed loop constant this " +
    "rate supports. blob's autotune assumes 20 ms when it derives that constant, so a loop " +
    "slower than that has had its pole placed where the loop cannot follow it, and it rings.</div>";

  host.appendChild(box);
  hist = {plot: box.querySelector(".plot"), canvas: box.querySelector("canvas")};
}

var hist = null;

function drawHistogram() {
  var w = hist.plot.clientWidth, h = hist.plot.clientHeight;
  var dpr = window.devicePixelRatio || 1;

  hist.canvas.width = Math.round(w * dpr);
  hist.canvas.height = Math.round(h * dpr);

  var g = hist.canvas.getContext("2d");
  g.setTransform(dpr, 0, 0, dpr, 0, 0);
  g.clearRect(0, 0, w, h);

  var pw = w - PAD.l - PAD.r, ph = h - PAD.t - PAD.b;
  var counts = D.histCounts, edges = D.histEdges;

  var most = 0;
  for (var i = 0; i < counts.length; i++) most = Math.max(most, counts[i]);
  if (most === 0) return;

  var bw = pw / counts.length;
  g.fillStyle = "#4C9AFF";
  for (var j = 0; j < counts.length; j++) {
    var bh = counts[j] / most * ph;
    g.fillRect(PAD.l + j * bw, PAD.t + ph - bh, Math.max(1, bw - 1), bh);
  }

  var xOf = function (ms) {
    var f = (ms - edges[0]) / Math.max(1e-9, edges[edges.length - 1] - edges[0]);
    return PAD.l + Math.max(0, Math.min(1, f)) * pw;
  };

  [[D.meanDt, "mean"], [D.maxDt, "worst"]].forEach(function (mark) {
    var px = Math.round(xOf(mark[0])) + 0.5;
    g.strokeStyle = mark[1] === "mean" ? "#36B37E" : "#FF5630";
    g.beginPath();
    g.moveTo(px, PAD.t);
    g.lineTo(px, PAD.t + ph);
    g.stroke();

    g.fillStyle = g.strokeStyle;
    g.font = "10px ui-sans-serif, sans-serif";
    g.textAlign = px > PAD.l + pw / 2 ? "right" : "left";
    g.textBaseline = "top";
    g.fillText(mark[1] + " " + mark[0].toFixed(1) + " ms", px + (px > PAD.l + pw / 2 ? -4 : 4), PAD.t + 2);
  });

  g.fillStyle = css("--muted");
  g.font = "10px ui-sans-serif, sans-serif";
  g.textAlign = "left";
  g.textBaseline = "bottom";
  g.fillText(edges[0].toFixed(1) + " ms", PAD.l, h - 3);
  g.textAlign = "right";
  g.fillText(edges[edges.length - 1].toFixed(1) + " ms", w - PAD.r, h - 3);

  g.strokeStyle = css("--line");
  g.strokeRect(PAD.l + 0.5, PAD.t + 0.5, pw - 1, ph - 1);
}

function drawAll() {
  CHARTS.forEach(draw);
  drawHistogram();

  document.getElementById("window").textContent =
    "showing " + (hi - lo + 1) + " loops, " + D.t[lo].toFixed(2) + " s to " + D.t[hi].toFixed(2) + " s";
}

build();
drawAll();

document.getElementById("reset").addEventListener("click", function () {
  lo = 0; hi = N - 1;
  drawAll();
});

window.addEventListener("resize", drawAll);

// Redrawn on a theme change as well, since the grid and the labels are read
// out of the stylesheet rather than written into the canvas calls.
if (window.matchMedia) {
  var dark = window.matchMedia("(prefers-color-scheme: dark)");
  if (dark.addEventListener) dark.addEventListener("change", drawAll);
}
</script>
</body>
</html>
`
