# Axottle core

This source snapshot contains the open core and local engine transport.
The standalone API provides bootstrap, sessions, MFA, project identity, general preferences and an audit journal using PostgreSQL, with a separate public browser interface. Module composition is still being completed; this is a development snapshot.

Build: `go build -trimpath -buildvcs=false -o axottle ./cmd/axottle-core`.

Run: `./axottle serve`. An engine in `../install_static/axottle-engine` relative to the core executable, or an adjacent `axottle-engine`, is supervised automatically; AXOTTLE_ENGINE_PATH selects another executable. An explicit AXOTTLE_ENGINE_SOCKET can connect a separately managed engine.
Without an engine, set AXOTTLE_DB_URL and AXOTTLE_SECRET_KEY. The core initializes its own common schema; /health checks the database.

Before building the Go binary, build the public UI: use Node.js from web/admin/.nvmrc, run `python3 scripts/open-core/build-ui.py`, then rebuild the Go binary. Closed UI pages and dependencies are excluded.

## License

The exported first-party core source is licensed under MIT; see LICENSE. Third-party components and fonts retain their own notices and licenses. The separately supplied proprietary engine and module source are not covered by this source snapshot's license.
