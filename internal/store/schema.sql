-- slk local cache. SQLite WAL mode.
-- Designed for many small writes (every incoming RTM event) + range reads
-- ordered by ts. Primary keys are natural Slack IDs.

CREATE TABLE IF NOT EXISTS users (
    id           TEXT PRIMARY KEY,
    name         TEXT,
    real_name    TEXT,
    display_name TEXT,
    email        TEXT,
    image_48     TEXT,
    is_bot       INTEGER NOT NULL DEFAULT 0,
    deleted      INTEGER NOT NULL DEFAULT 0,
    updated_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS channels (
    id           TEXT PRIMARY KEY,
    name         TEXT,
    kind         TEXT NOT NULL,    -- channel | group | im | mpim
    is_private   INTEGER NOT NULL DEFAULT 0,
    is_archived  INTEGER NOT NULL DEFAULT 0,
    is_member    INTEGER NOT NULL DEFAULT 0,
    other_user   TEXT,             -- for IMs
    topic        TEXT,
    purpose      TEXT,
    num_members  INTEGER NOT NULL DEFAULT 0,
    updated_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS channels_kind ON channels(kind);
CREATE INDEX IF NOT EXISTS channels_name ON channels(name);

CREATE TABLE IF NOT EXISTS messages (
    channel    TEXT NOT NULL,
    ts         TEXT NOT NULL,
    user       TEXT,
    text       TEXT,
    thread_ts  TEXT,
    subtype    TEXT,
    edited_ts  TEXT,
    raw_json   TEXT,
    PRIMARY KEY (channel, ts)
);
CREATE INDEX IF NOT EXISTS messages_channel_ts ON messages(channel, ts DESC);
CREATE INDEX IF NOT EXISTS messages_thread   ON messages(channel, thread_ts) WHERE thread_ts IS NOT NULL;

CREATE TABLE IF NOT EXISTS cursors (
    channel TEXT PRIMARY KEY,
    last_ts TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS kv (
    k TEXT PRIMARY KEY,
    v TEXT
);
