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

## Running the server

The HTTP API persists all data to a JSON file. Default paths are created automatically, so you can simply run:

```bash
go run ./cmd/app
```

Override paths or listening address via environment variables:

```bash
PELLETS_DATA_FILE=./data/pellets.json \
PELLETS_BACKUP_DIR=./data/backups \
PELLETS_LISTEN_ADDR=0.0.0.0:8080 \
go run ./cmd/app
```
