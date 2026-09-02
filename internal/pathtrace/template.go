package pathtrace

const pageTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.OpMode}} - blob path</title>
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
  .sub { color: var(--muted); margin-bottom: 20px; }
  .cards { display: flex; flex-wrap: wrap; gap: 12px; margin-bottom: 20px; }
  .card { background: var(--panel); border: 1px solid var(--line); border-radius: 10px;
          padding: 12px 16px; min-width: 150px; flex: 1; }
  .card .k { color: var(--muted); font-size: 12px; text-transform: uppercase;
             letter-spacing: .04em; }
  .card .v { font-size: 22px; font-weight: 600; margin-top: 2px; }
  .card .v small { font-size: 13px; font-weight: 400; color: var(--muted); }
  .layout { display: flex; gap: 20px; flex-wrap: wrap; align-items: flex-start; }
  .fieldbox { flex: 1 1 480px; min-width: 320px; }
  svg { width: 100%; height: auto; background: var(--panel);
        border: 1px solid var(--line); border-radius: 10px; }
  .legend { display: flex; align-items: center; gap: 8px; margin-top: 10px;
            color: var(--muted); font-size: 12px; }
  .ramp { flex: 1; height: 10px; border-radius: 5px; }
  .tablebox { flex: 1 1 420px; min-width: 320px; overflow-x: auto; }
  table { border-collapse: collapse; width: 100%; font-size: 13px; }
  th, td { text-align: left; padding: 7px 9px; border-bottom: 1px solid var(--line);
           white-space: nowrap; }
  th { color: var(--muted); font-weight: 600; font-size: 11px;
       text-transform: uppercase; letter-spacing: .04em; }
  td.num { text-align: right; font-variant-numeric: tabular-nums; }
  tr.slow td { background: rgba(255,86,48,.09); }
  .tag { display: inline-block; padding: 1px 7px; border-radius: 20px; font-size: 11px;
         background: var(--line); color: var(--muted); }
  .src { color: var(--muted); font-size: 11px; }
  footer { margin-top: 24px; color: var(--muted); font-size: 12px; }
</style>
</head>
<body>
<div class="wrap">
  <h1>{{.OpMode}}</h1>
  <div class="sub">{{.Generated}}</div>

  <div class="cards">
    <div class="card"><div class="k">Measured</div>
      <div class="v">{{.ActualTotal}}<small> s</small></div></div>
    <div class="card"><div class="k">Model estimate</div>
      <div class="v">{{.EstTotal}}<small> s</small></div></div>
    <div class="card"><div class="k">Model vs real</div>
      <div class="v" style="font-size:16px">{{.Delta}}</div></div>
    <div class="card"><div class="k">Distance</div>
      <div class="v">{{.TotalLength}}<small> in</small></div></div>
    <div class="card"><div class="k">Slowest leg</div>
      <div class="v" style="font-size:16px">{{.SlowestName}}
        <small>{{.SlowestTime}} s</small></div></div>
  </div>

  <div class="layout">
    <div class="fieldbox">
      <svg viewBox="0 0 {{.ViewSize}} {{.ViewSize}}">
        {{range .GridLines}}
        <line x1="{{.X1}}" y1="{{.Y1}}" x2="{{.X2}}" y2="{{.Y2}}"
              stroke="{{if .Major}}#8894a3{{else}}#8894a3{{end}}"
              stroke-opacity="{{if .Major}}.45{{else}}.15{{end}}" stroke-width="2"/>
        {{end}}

        {{range .Segments}}
          {{range .Strokes}}
          {{if $.HasSamples}}
          <line x1="{{.X1}}" y1="{{.Y1}}" x2="{{.X2}}" y2="{{.Y2}}"
                stroke="#8894a3" stroke-opacity=".38" stroke-width="4"
                stroke-linecap="round" stroke-dasharray="7 9"/>
          {{else}}
          <line x1="{{.X1}}" y1="{{.Y1}}" x2="{{.X2}}" y2="{{.Y2}}"
                stroke="{{.Colour}}" stroke-width="7" stroke-linecap="round"/>
          {{end}}
          {{end}}
        {{end}}

        {{range .Robot}}
        <line x1="{{.X1}}" y1="{{.Y1}}" x2="{{.X2}}" y2="{{.Y2}}"
              stroke="{{.Colour}}" stroke-width="7" stroke-linecap="round"/>
        {{end}}

        {{range .Segments}}
          {{range .Markers}}
            {{if eq .Kind "intercept"}}
            <circle cx="{{.X}}" cy="{{.Y}}" r="7" fill="#36B37E"
                    stroke="#fff" stroke-width="2"><title>{{.Title}}</title></circle>
            {{else}}
            <rect x="{{.X}}" y="{{.Y}}" width="11" height="11" transform="translate(-5.5,-5.5)"
                  fill="#36B37E" stroke="#fff" stroke-width="2"><title>{{.Title}}</title></rect>
            {{end}}
          {{end}}
          {{if .HasTarget}}
          <circle cx="{{.TargetX}}" cy="{{.TargetY}}" r="9" fill="none"
                  stroke="#FF5630" stroke-width="3"><title>{{.Label}} target</title></circle>
          <text x="{{.TargetX}}" y="{{.TargetY}}" dy="-14" text-anchor="middle"
                font-size="19" fill="#8894a3">{{.Index}}</text>
          {{end}}
        {{end}}
      </svg>

      <div class="legend">
        <span>{{if .HasSamples}}measured{{else}}modelled{{end}} 0 in/s</span>
        <div class="ramp" style="background:linear-gradient(to right{{range .LegendStops}},{{.Colour}}{{end}})"></div>
        <span>{{.SpeedMax}} in/s</span>
      </div>
    </div>

    <div class="tablebox">
      <table>
        <thead><tr>
          <th>#</th><th>State</th><th>Type</th>
          <th class="num">Power</th><th class="num">Dist</th>
          <th class="num">Est</th><th class="num">Real</th><th class="num">Peak</th>
        </tr></thead>
        <tbody>
        {{range .Segments}}
          <tr{{if .Slow}} class="slow"{{end}}>
            <td class="num">{{.Index}}</td>
            <td>{{.Label}}<br><span class="src">{{.Source}}</span></td>
            <td><span class="tag">{{.Type}}</span></td>
            <td class="num">{{.MaxPower}}</td>
            <td class="num">{{.Length}}</td>
            <td class="num">{{.Est}}</td>
            <td class="num">{{.Actual}}</td>
            <td class="num">{{.Peak}}</td>
          </tr>
        {{end}}
        </tbody>
      </table>
    </div>
  </div>

  <footer>
    {{if .HasSamples}}
    The solid line is where the robot actually went, drawn from its recorded
    positions in the order it recorded them, and coloured by the speed it was
    really doing: cold is slow. The dashed line underneath is the path it was
    asked to follow, so the gap between them is the following error.
    {{else}}
    Colour is modelled speed: cold is slow. This run recorded no positions, so
    only the path it was asked to follow can be drawn.
    {{end}}
    Highlighted rows never get above half the run's top speed, which usually
    means the curve is tight enough that maxPower is not the thing limiting you.
    "Real" comes from the robot; "Est" comes from the kinematic model, so a
    large gap means the model's limits need tuning to match your drivetrain.
  </footer>
</div>
</body>
</html>
`
