// Counts the devices pusher runs on.
//
// One row per device, keyed by a hash of the random ID the client generated for
// itself. The ID never leaves the client in a form this stores: PEPPER is a
// secret only the Worker knows, so a copy of the database cannot be matched
// against a config file found on a machine.
//
// What arrives is the ID, the pusher version and the platform. Deliberately no
// IP address, no hostname, no project or robot details, and no free text: the
// version and platform are length-capped and stored as sent, and everything
// else in the body is dropped on the floor.

const MAX_BODY = 512;
const ID_PATTERN = /^[0-9a-f]{32}$/;

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    if (request.method === "POST" && url.pathname === "/ping") {
      return ping(request, env);
    }

    if (request.method === "GET" && url.pathname === "/stats") {
      return stats(request, env);
    }

    return new Response("pusher", { status: 404 });
  },
};

async function ping(request, env) {
  const body = await readJSON(request);
  if (body === null || !ID_PATTERN.test(String(body.id || ""))) {
    return new Response(null, { status: 400 });
  }

  const id = await hash(body.id, env.PEPPER);
  const version = clamp(body.version);
  const platform = clamp(body.platform);
  const today = new Date().toISOString().slice(0, 10);

  // first_seen is written once and never touched again, so the row remembers
  // when this device turned up even as the rest of it is overwritten.
  await env.DB.prepare(
    `INSERT INTO devices (id, first_seen, last_seen, version, platform, pings)
          VALUES (?1, ?2, ?2, ?3, ?4, 1)
     ON CONFLICT(id) DO UPDATE SET
          last_seen = ?2,
          version   = ?3,
          platform  = ?4,
          pings     = pings + 1`
  )
    .bind(id, today, version, platform)
    .run();

  return new Response(null, { status: 204 });
}

async function stats(request, env) {
  if (!env.STATS_TOKEN || request.headers.get("authorization") !== `Bearer ${env.STATS_TOKEN}`) {
    return new Response(null, { status: 401 });
  }

  const totals = await env.DB.prepare(
    // COALESCE because SUM over no rows is null, and a fresh counter reporting
    // "active_7d": null reads like a bug rather than like nobody yet.
    `SELECT COUNT(*) AS devices,
            COALESCE(SUM(last_seen >= date('now', '-7 days')),  0) AS active_7d,
            COALESCE(SUM(last_seen >= date('now', '-30 days')), 0) AS active_30d,
            COALESCE(MIN(first_seen), '') AS counting_since
       FROM devices`
  ).first();

  const versions = await env.DB.prepare(
    `SELECT version, COUNT(*) AS devices
       FROM devices GROUP BY version ORDER BY devices DESC`
  ).all();

  const platforms = await env.DB.prepare(
    `SELECT platform, COUNT(*) AS devices
       FROM devices GROUP BY platform ORDER BY devices DESC`
  ).all();

  return Response.json({
    ...totals,
    versions: versions.results,
    platforms: platforms.results,
  });
}

// readJSON refuses anything larger than a ping could possibly be, rather than
// letting a stranger decide how much this Worker parses.
async function readJSON(request) {
  const length = Number(request.headers.get("content-length") || 0);
  if (length > MAX_BODY) {
    return null;
  }

  const text = await request.text();
  if (text.length > MAX_BODY) {
    return null;
  }

  try {
    const body = JSON.parse(text);
    return body && typeof body === "object" ? body : null;
  } catch {
    return null;
  }
}

function clamp(value) {
  return String(value == null ? "" : value)
    .replace(/[^\w.\-\/+]/g, "")
    .slice(0, 32);
}

async function hash(id, pepper) {
  const data = new TextEncoder().encode(`${id}${pepper || ""}`);
  const digest = await crypto.subtle.digest("SHA-256", data);

  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
}
