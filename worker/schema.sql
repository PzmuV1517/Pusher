-- One row per device. The id is a hash of the client's random ID, peppered with
-- a secret only the Worker holds, so this table cannot be matched against a
-- config file found on somebody's machine.
--
-- Dates are stored as YYYY-MM-DD rather than timestamps: the question is how
-- many devices, not what time anyone was working.
CREATE TABLE IF NOT EXISTS devices (
    id         TEXT PRIMARY KEY,
    first_seen TEXT NOT NULL,
    last_seen  TEXT NOT NULL,
    version    TEXT NOT NULL DEFAULT '',
    platform   TEXT NOT NULL DEFAULT '',
    pings      INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS devices_last_seen ON devices (last_seen);
