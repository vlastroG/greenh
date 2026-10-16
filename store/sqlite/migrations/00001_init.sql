-- +goose Up
CREATE TABLE nodes (
    id        TEXT PRIMARY KEY,
    name      TEXT NOT NULL DEFAULT '',
    last_seen INTEGER NOT NULL
);

CREATE TABLE node_channels (
    node_id TEXT NOT NULL REFERENCES nodes(id),
    ch      INTEGER NOT NULL,
    PRIMARY KEY (node_id, ch)
);

CREATE TABLE readings (
    node_id      TEXT NOT NULL,
    ts           INTEGER NOT NULL,
    temp_c       REAL,
    humidity_pct REAL,
    light_lux    REAL,
    PRIMARY KEY (node_id, ts)
);

CREATE TABLE channel_readings (
    node_id      TEXT NOT NULL,
    ts           INTEGER NOT NULL,
    ch           INTEGER NOT NULL,
    moisture_raw INTEGER NOT NULL,
    PRIMARY KEY (node_id, ch, ts)
);

CREATE TABLE pots (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    name             TEXT NOT NULL,
    species          TEXT NOT NULL,
    category         TEXT NOT NULL,
    pot_ml           INTEGER NOT NULL,
    soil_type        TEXT NOT NULL,
    placement        TEXT NOT NULL,
    node_id          TEXT NULL,
    channel          INTEGER NULL,
    dry_raw          INTEGER NULL,
    wet_raw          INTEGER NULL,
    threshold_pct    REAL,
    amount_fraction  REAL,
    check_interval_s INTEGER,
    created_at       INTEGER NOT NULL
);
CREATE UNIQUE INDEX pots_link ON pots(node_id, channel) WHERE node_id IS NOT NULL;

CREATE TABLE waterings (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    pot_id     INTEGER NOT NULL REFERENCES pots(id) ON DELETE CASCADE,
    ts         INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    amount_ml  INTEGER NOT NULL,
    source     TEXT NOT NULL
);
CREATE INDEX waterings_created_at ON waterings(created_at);

CREATE TABLE advice (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    pot_id          INTEGER NOT NULL REFERENCES pots(id) ON DELETE CASCADE,
    created_at      INTEGER NOT NULL,
    trigger         TEXT NOT NULL,
    status          TEXT NOT NULL,
    recommendation  TEXT NULL,
    rules           TEXT NULL,
    provider        TEXT NOT NULL DEFAULT '',
    model           TEXT NOT NULL DEFAULT '',
    prompt_version  TEXT NOT NULL DEFAULT '',
    fallback        INTEGER NOT NULL DEFAULT 0,
    fallback_reason TEXT NOT NULL DEFAULT '',
    disagree        INTEGER NOT NULL DEFAULT 0,
    summary         TEXT NULL
);
CREATE INDEX advice_pot_created ON advice(pot_id, created_at);

-- +goose Down
DROP TABLE advice;
DROP TABLE waterings;
DROP TABLE pots;
DROP TABLE channel_readings;
DROP TABLE readings;
DROP TABLE node_channels;
DROP TABLE nodes;
