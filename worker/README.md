# pusher-count

The device counter. A Cloudflare Worker and a D1 database, both on the free
tier, both owned by you: no third party is involved and nothing leaves
Cloudflare.

The client sends a random ID it made up for itself, the pusher version and the
platform, at most once a day. The Worker hashes the ID with a secret pepper
before storing it, so this database cannot be matched against a config file
found on somebody's machine. There is no IP address, no hostname and no free
text in the table.

## Setting it up

Once, from this directory. It takes about five minutes.

Everything below uses `npx wrangler`, which fetches wrangler on first use and
caches it. There is nothing to install and nothing to keep up to date. Verified
against wrangler 4.123.

```bash
npx wrangler login             # opens a browser, needs a free Cloudflare account
```

Sign up first at <https://dash.cloudflare.com/sign-up> if you have not. The
Workers free plan needs no card.

**1. Create the database.**

```bash
npx wrangler d1 create pusher-devices
```

It prints a `database_id`. Paste it into `wrangler.toml`, replacing
`PASTE_DATABASE_ID_HERE`.

**2. Create the table.**

```bash
npx wrangler d1 execute pusher-devices --remote --file=schema.sql
```

`--remote` matters. Without it you create the table in a local emulator and the
deployed Worker sees an empty database.

**3. Deploy.**

```bash
npx wrangler deploy
```

It prints the URL. For this account that is
`https://pusher-count.quantum-robotics-9fc.workers.dev`, and the database lives
in EEUR.

Deploying before setting the secrets is deliberate: `npx wrangler secret put` stops
to ask whether to create the Worker if it does not exist yet, which a piped
secret cannot answer.

**4. Set the two secrets.**

```bash
head -c 32 /dev/urandom | xxd -p -c 32 | npx wrangler secret put PEPPER
head -c 32 /dev/urandom | xxd -p -c 32 | npx wrangler secret put STATS_TOKEN
```

They take effect immediately, with no redeploy. Save the `STATS_TOKEN` value
somewhere: it is the only way to read the numbers, and it is not recoverable
afterwards.

Never change `PEPPER` once devices are counted. Every existing device would hash
differently and be counted all over again as new. Between step 3 and this one
the Worker hashes with no pepper at all, which is harmless only because no build
of pusher knows the URL yet.

**5. Point pusher at it.** Put that URL in `internal/telemetry/telemetry.go`, in
the `endpoint` variable. It is already set to the deployed Worker above.

Left empty, pusher counts nothing and sends nothing: an empty endpoint disables
the whole feature rather than failing against a dead URL, and `pusher settings`
shows the row as "not set up".

It can also be set at build time without touching the source, which is handy for
testing against a Worker running locally:

```bash
go build -ldflags "-X github.com/andreibanu/pusher/internal/telemetry.endpoint=http://127.0.0.1:8787"
```

The Homebrew formula builds from source with its own ldflags, so the value in
the file is the one that ships.

## Testing it without deploying

```bash
node test.mjs
```

Runs the Worker against a stand-in for D1 and checks what it stores, what it
rejects, and that the ID is hashed before it is written. No network, no
Cloudflare account, no dependencies beyond node itself.

## Reading the numbers

```bash
curl -H "Authorization: Bearer $STATS_TOKEN" \
  https://pusher-count.quantum-robotics-9fc.workers.dev/stats
```

```json
{
  "devices": 47,
  "active_7d": 31,
  "active_30d": 44,
  "counting_since": "2026-08-14",
  "versions":  [{ "version": "1.2.3", "devices": 40 }, { "version": "1.2.2", "devices": 7 }],
  "platforms": [{ "platform": "darwin/arm64", "devices": 38 }]
}
```

`devices` is every device ever counted. `active_7d` is how many ran pusher in
the last week, which is the honest number for "how many people use this".

Or query it directly:

```bash
npx wrangler d1 execute pusher-devices --remote \
  --command "SELECT version, COUNT(*) FROM devices GROUP BY version"
```

### From the dashboard, without the token

<https://dash.cloudflare.com> → **Storage & Databases** → **D1 SQL database** →
**pusher-devices** → **Console**, and paste this:

```sql
SELECT COUNT(*)                                            AS devices,
       SUM(last_seen >= date('now', '-7 days'))            AS active_7d,
       SUM(last_seen >= date('now', '-30 days'))           AS active_30d,
       MIN(first_seen)                                     AS counting_since
  FROM devices;
```

The **Tables** tab next to it lists the rows themselves, which is the fastest
way to confirm a ping arrived while testing.

The Worker has its own **Metrics** tab, under **Compute** → **Workers & Pages**
→ **pusher-count**. Read it as requests, not devices: it counts pings, so a
device that reinstalls or a stranger who finds the URL both show up there and
neither changes the number of rows. Observability is on, so **Logs** on the same
page shows individual requests while you are testing.

## What it costs

Nothing, at any volume this will see. The Worker free tier is 100,000 requests a
day and each device sends at most one; D1's free tier is 100,000 writes a day
and 5GB of storage, against rows of about 100 bytes.

## Changing it later

`npx wrangler deploy` again. The client treats every response as success and ignores
the body, so the Worker can change shape freely without breaking old versions of
pusher. If the Worker is deleted or the URL changes, old clients quietly fail
their ping and carry on: nothing about a deploy depends on this.
