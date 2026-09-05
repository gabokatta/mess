DROP TABLE IF EXISTS base_amount;
DROP TABLE IF EXISTS month_entry;
DROP TABLE IF EXISTS saving_allocation;
DROP TABLE IF EXISTS list;
DROP TABLE IF EXISTS concept;
DROP TABLE IF EXISTS category;
DROP TABLE IF EXISTS fx_rate;
DROP TABLE IF EXISTS settings;

CREATE TABLE category (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    color_index INTEGER NOT NULL DEFAULT 0 CHECK (color_index BETWEEN 0 AND 7)
);

CREATE TABLE concept (
    id           INTEGER PRIMARY KEY,
    name         TEXT NOT NULL,
    category_id  INTEGER NOT NULL REFERENCES category (id),
    kind         TEXT NOT NULL CHECK (kind IN ('Income', 'Expense', 'Saving', 'Chore')),
    currency     TEXT CHECK (currency IS NULL OR currency IN ('ARS', 'USD')),
    base_amount  TEXT,
    month_mask   INTEGER NOT NULL,
    active_from  TEXT NOT NULL,
    active_until TEXT
);

CREATE INDEX concept_category_id ON concept (category_id);

CREATE TABLE month_entry (
    concept_id INTEGER NOT NULL REFERENCES concept (id),
    period     TEXT NOT NULL,
    amount     TEXT,
    done       INTEGER NOT NULL DEFAULT 0 CHECK (done IN (0, 1)),
    PRIMARY KEY (concept_id, period)
);

CREATE TABLE note (
    id      INTEGER PRIMARY KEY,
    title   TEXT NOT NULL,
    body_md TEXT NOT NULL DEFAULT '',
    period  TEXT,
    done    INTEGER NOT NULL DEFAULT 0 CHECK (done IN (0, 1))
);

CREATE TABLE fx_rate (
    period TEXT PRIMARY KEY,
    value  TEXT NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('Close', 'Manual')),
    house  TEXT CHECK (house IS NULL OR house IN ('Blue', 'Official', 'MEP'))
);

CREATE TABLE settings (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    fx_house    TEXT NOT NULL CHECK (fx_house IN ('Blue', 'Official', 'MEP')),
    last_export TEXT
);
