CREATE TABLE category (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE concept (
    id           INTEGER PRIMARY KEY,
    name         TEXT NOT NULL,
    category_id  INTEGER NOT NULL REFERENCES category (id),
    kind         TEXT NOT NULL CHECK (kind IN ('Income', 'FixedExpense', 'VariableExpense')),
    currency     TEXT NOT NULL CHECK (currency IN ('ARS', 'USD')),
    month_mask   INTEGER NOT NULL,
    share        TEXT NOT NULL DEFAULT '1',
    due_day      INTEGER,
    sort_order   INTEGER NOT NULL DEFAULT 0,
    active_from  TEXT NOT NULL,
    active_until TEXT
);

CREATE INDEX concept_category_id ON concept (category_id);

CREATE TABLE base_amount (
    concept_id     INTEGER NOT NULL REFERENCES concept (id),
    effective_from TEXT NOT NULL,
    amount         TEXT NOT NULL,
    PRIMARY KEY (concept_id, effective_from)
);

CREATE TABLE month_entry (
    concept_id INTEGER NOT NULL REFERENCES concept (id),
    period     TEXT NOT NULL,
    amount     TEXT,
    done       INTEGER NOT NULL DEFAULT 0 CHECK (done IN (0, 1)),
    PRIMARY KEY (concept_id, period)
);

CREATE TABLE chore (
    id           INTEGER PRIMARY KEY,
    name         TEXT NOT NULL,
    month_mask   INTEGER NOT NULL,
    due_day      INTEGER,
    active_from  TEXT NOT NULL,
    active_until TEXT,
    sort_order   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE chore_entry (
    chore_id INTEGER NOT NULL REFERENCES chore (id),
    period   TEXT NOT NULL,
    done     INTEGER NOT NULL DEFAULT 0 CHECK (done IN (0, 1)),
    PRIMARY KEY (chore_id, period)
);

CREATE TABLE saving_allocation (
    id          INTEGER PRIMARY KEY,
    period      TEXT NOT NULL,
    destination TEXT NOT NULL CHECK (destination IN ('Cash', 'Invested')),
    amount      TEXT NOT NULL,
    currency    TEXT NOT NULL CHECK (currency IN ('ARS', 'USD'))
);

CREATE INDEX saving_allocation_period ON saving_allocation (period);

CREATE TABLE project (
    id        INTEGER PRIMARY KEY,
    name      TEXT NOT NULL,
    body_md   TEXT NOT NULL DEFAULT '',
    period    TEXT,
    closed_at TEXT
);

CREATE TABLE fx_rate (
    period TEXT PRIMARY KEY,
    value  TEXT NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('Fetched', 'Manual'))
);

CREATE TABLE settings (
    id                     INTEGER PRIMARY KEY CHECK (id = 1),
    allowance_cap          TEXT NOT NULL,
    allowance_rate         TEXT NOT NULL,
    fx_house               TEXT NOT NULL CHECK (fx_house IN ('Blue', 'Official', 'MEP')),
    opening_period         TEXT NOT NULL,
    opening_leftover_pesos TEXT NOT NULL DEFAULT '0',
    opening_cash_usd       TEXT NOT NULL DEFAULT '0',
    opening_invested_usd   TEXT NOT NULL DEFAULT '0'
);
