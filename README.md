# mess

A monthly finance tracker for the terminal, with ARS/USD amounts, recurring
concepts, notes, and exchange rates stored in SQLite.

## Build

Requires Go 1.27. The terminal UI currently needs at least 135 columns and 30 rows.

    make build       # bin/mess
    make build-all   # darwin/arm64 and windows/amd64 release binaries

## Usage

    mess                        # opens on the default database
    mess --db path/to/mess.db   # opens on a specific database
    mess export --db path       # dumps every table to stdout as JSON
    mess import --db path file  # replaces the database with a backup, after confirming

The default database lives at `mess/mess.db` under the OS config directory
(`~/Library/Application Support` on macOS).

## Development

    make check       # formatting, vet, tests, and native build
    make fmt         # format Go sources
    make seed        # replace .data/mess.db with demo data
    make dev         # open the demo database

See [`internal/fixture/README.md`](internal/fixture/README.md) for seeding a
throwaway database to develop against.
