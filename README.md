# mess
simple monthly tracker for your life.

## Build

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

See [`internal/fixture/README.md`](internal/fixture/README.md) for seeding a
throwaway database to develop against.
