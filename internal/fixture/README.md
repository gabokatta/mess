# Seeding a dev database

`Demo` (in `demo.go`) covers eighteen months with long names,
wide amounts, a missing fx rate, a cadence gap, an ended concept, an
over-saved month, and the rest of the cases that break a layout by accident.
`mess seed` and the test suite share `World` and `Load`. Test helpers live in
`internal/testutil`, keeping the testing package out of the application build.

## Commands

    make seed        # wipes .data/mess.db and refills it with Demo
    make dev         # opens the app on .data/mess.db as it is, no reseeding
    make clean-dev   # removes .data entirely

`mess seed` requires `--db` and refuses a locked database. It builds the demo
in a temporary database, then replaces the destination's rows in one transaction.

`.data/` is gitignored, so it can be deleted at any point without touching
the real database `make run` uses.

The usual loop is `make seed` for a clean, reproducible start, `make dev` to
try things against it, and `make seed` again once the data is mangled or a
fresh screenshot is needed.

## Anchor and `--period`

`Demo`'s newest month is always the current one, so the app opens on
populated data instead of an empty month. Pin it with `--period` when two
runs need to match byte for byte:

    mess seed --db .data/mess.db --period 2026-09

Every period `Demo` writes is relative to the anchor, so pinning it
reproduces the whole eighteen-month window exactly.
