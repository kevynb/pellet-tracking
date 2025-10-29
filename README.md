# pellet-tracking
Track pellet consumption

## Development

To sync this fork with the canonical repository, configure the `origin` remote to
point at [kevynb/pellet-tracking](https://github.com/kevynb/pellet-tracking):

```bash
git remote add origin https://github.com/kevynb/pellet-tracking.git
```

Fetch the latest changes before rebasing or creating pull requests:

```bash
git fetch origin
```

## Running the server & web UI

The service persists all data to a JSON file and exposes both a REST API and an embedded htmx-powered interface. Defaults are created automatically:

```bash
go run ./cmd/app
```

Open [http://127.0.0.1:8080](http://127.0.0.1:8080) to access the mobile-first UI. Use the navigation bar to switch between purchases, consumptions, stats, and brand management. Forms submit directly to the server—no additional tooling required.

Override paths or listening address via environment variables:

```bash
PELLETS_DATA_FILE=./data/pellets.json \
PELLETS_BACKUP_DIR=./data/backups \
PELLETS_LISTEN_ADDR=0.0.0.0:8080 \
go run ./cmd/app
```

### Optional TSnet (Tailscale) listener

To expose the API/UI on your Tailscale network, enable the bundled TSnet server:

```bash
PELLETS_TSNET_ENABLED=1 \
PELLETS_TSNET_AUTHKEY=tskey-... \
PELLETS_TSNET_DIR=/var/lib/pellets-tsnet \
PELLETS_TSNET_HOSTNAME=pellets \
PELLETS_TSNET_LISTEN_ADDR=:443 \
go run ./cmd/app
```

When TSnet is enabled the process listens exclusively on the Tailscale interface while maintaining graceful shutdown semantics.
