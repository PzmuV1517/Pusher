import worker from "./src/index.js";

let bound = [];
const env = {
  PEPPER: "pepper",
  STATS_TOKEN: "secret",
  DB: {
    prepare(sql) {
      return {
        bind(...args) { bound.push({ sql, args }); return this; },
        async run() { return {}; },
        async first() { return { devices: 2, active_7d: 1, active_30d: 1, counting_since: "2026-08-14" }; },
        async all() { return { results: [{ version: "1.2.3", devices: 2 }] }; },
      };
    },
  },
};

const post = (body, headers = {}) =>
  new Request("https://x/ping", { method: "POST", body: typeof body === "string" ? body : JSON.stringify(body), headers });

let fails = 0;
const check = (name, cond, extra = "") => {
  if (!cond) { fails++; console.log(`FAIL  ${name} ${extra}`); } else { console.log(`ok    ${name}`); }
};

// a good ping
bound = [];
let r = await worker.fetch(post({ id: "a".repeat(32), version: "1.2.3", platform: "darwin/arm64" }), env);
check("good ping returns 204", r.status === 204, `got ${r.status}`);
check("one row written", bound.length === 1);
check("id is hashed, not stored raw", bound[0]?.args[0] !== "a".repeat(32) && /^[0-9a-f]{64}$/.test(bound[0]?.args[0] || ""), bound[0]?.args[0]);
check("version and platform pass through", bound[0]?.args[2] === "1.2.3" && bound[0]?.args[3] === "darwin/arm64", JSON.stringify(bound[0]?.args));

// the pepper actually changes the hash
bound = [];
await worker.fetch(post({ id: "a".repeat(32) }), { ...env, PEPPER: "other" });
const otherHash = bound[0].args[0];
bound = [];
await worker.fetch(post({ id: "a".repeat(32) }), env);
check("pepper changes the hash", otherHash !== bound[0].args[0]);

// the same id always hashes the same
bound = [];
await worker.fetch(post({ id: "b".repeat(32) }), env);
const first = bound[0].args[0];
bound = [];
await worker.fetch(post({ id: "b".repeat(32) }), env);
check("same id, same hash", first === bound[0].args[0]);

// junk
for (const [name, body] of [
  ["missing id", { version: "1" }],
  ["short id", { id: "abc" }],
  ["non-hex id", { id: "z".repeat(32) }],
  ["not json", "{{{"],
  ["not an object", "42"],
]) {
  bound = [];
  r = await worker.fetch(post(body), env);
  check(`${name} rejected`, r.status === 400 && bound.length === 0, `got ${r.status}`);
}

// oversized body
bound = [];
r = await worker.fetch(post({ id: "a".repeat(32), version: "x".repeat(4000) }), env);
check("oversized body rejected", r.status === 400 && bound.length === 0, `got ${r.status}`);

// hostile version string gets stripped and capped
bound = [];
await worker.fetch(post({ id: "a".repeat(32), version: "<script>alert(1)</script>", platform: "'; DROP TABLE devices;--" }), env);
check("junk stripped from version", !/[<>']/.test(bound[0].args[2]), bound[0].args[2]);
check("junk stripped from platform", !/[<>';]/.test(bound[0].args[3]), bound[0].args[3]);
check("fields capped at 32", bound[0].args[2].length <= 32 && bound[0].args[3].length <= 32);

// stats
r = await worker.fetch(new Request("https://x/stats"), env);
check("stats needs a token", r.status === 401, `got ${r.status}`);
r = await worker.fetch(new Request("https://x/stats", { headers: { authorization: "Bearer wrong" } }), env);
check("stats rejects a wrong token", r.status === 401, `got ${r.status}`);
r = await worker.fetch(new Request("https://x/stats", { headers: { authorization: "Bearer secret" } }), env);
check("stats returns json", r.status === 200, `got ${r.status}`);
const stats = await r.json();
check("stats has the totals and the breakdowns", stats.devices === 2 && Array.isArray(stats.versions) && Array.isArray(stats.platforms), JSON.stringify(stats));

// wrong method and path
r = await worker.fetch(new Request("https://x/ping"), env);
check("GET /ping is not a ping", r.status === 404, `got ${r.status}`);
r = await worker.fetch(new Request("https://x/", { method: "POST" }), env);
check("unknown path", r.status === 404, `got ${r.status}`);

console.log(fails === 0 ? "\nALL WORKER CHECKS PASSED" : `\n${fails} FAILED`);
process.exit(fails === 0 ? 0 : 1);
