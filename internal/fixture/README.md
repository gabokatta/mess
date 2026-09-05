# Seed a development database

`Demo` in `demo.go` contains eighteen months of data. It includes long names,
large amounts, a missing exchange rate, cadence gaps, ended concepts, and
over-saved months. `mess seed` and the tests use the same `World` and `Load`.
Test helpers live in `internal/testutil`.

## Commands

    make seed        # replaces .data/mess.db with Demo
    make dev         # opens .data/mess.db without reseeding
    make clean-dev   # removes .data

`mess seed` requires `--db` and refuses a locked database. It builds the demo
in a temporary database, then replaces the destination rows in one transaction.

`.data/` is ignored by Git. Deleting it does not affect the default database
used by `make run`.

Run `make seed` for a clean start, then `make dev`. Run `make seed` again when
you want a fresh database or screenshot.

## Anchor and `--period`

`Demo` ends in the current month, so the app opens with populated data. Pass
`--period` when two runs need identical data:

    mess seed --db .data/mess.db --period 2026-09

Every `Demo` period is relative to the anchor. Pinning it reproduces the same
eighteen-month window.
