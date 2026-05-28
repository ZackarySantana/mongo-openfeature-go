# AGENTS.md

## Cursor Cloud specific instructions

### Project overview

Go 1.24 library implementing a MongoDB-backed OpenFeature feature flag provider. Includes:
- Core provider library (`src/`)
- Web-based flag editor (`cmd/editor/`)
- MCP server (`cmd/mcp/`)

### Prerequisites

- **Go 1.24** (already in environment)
- **Docker** (required for testcontainers and for running MongoDB in replica set mode)

### Running tests

```bash
# Unit tests (no Docker needed)
go test ./src/... -count=1

# All tests including internal/editor
go test ./... -count=1
```

Integration tests requiring MongoDB (via testcontainers) may hit Docker Hub rate limits. If that happens, pre-pull the image with `docker pull mongo:8.0` before running.

### Running the editor

Start MongoDB in replica set mode (required for change streams):
```bash
docker run -d --name mongodb-dev -p 27017:27017 mongo:8.0 --replSet rs0 --bind_ip_all --port 27017
sleep 3
docker exec mongodb-dev mongosh --port 27017 --quiet --eval "rs.initiate({_id:'rs0',members:[{_id:0,host:'localhost:27017'}]})"
```

Then run the editor:
```bash
MONGODB_ENDPOINT=mongodb://localhost:27017 go run cmd/editor/main.go
```

The editor serves on `http://localhost:8080` (configurable via `EDITOR_PORT`).

### Running the MCP server

```bash
MONGODB_ENDPOINT=mongodb://localhost:27017 MCP_SERVE=http MCP_PORT=8081 go run cmd/mcp/main.go
```

### Lint / vet

```bash
go vet ./...
```

### Docker in this environment

Docker is configured with `fuse-overlayfs` storage driver and `iptables-legacy` for the nested container environment. The Docker daemon must be started with `sudo dockerd` before use. After starting, run `sudo chmod 666 /var/run/docker.sock` for non-root access.

### Key gotchas

- MongoDB **must** run with `--replSet` flag; the provider uses change streams which require a replica set.
- `USE_TESTCONTAINER=true` mode cannot run inside a Docker container (it uses Docker-in-Docker via testcontainers-go).
- The `cmd/example/main.go` also starts the editor on port 8080; stop any existing editor before running it, or set `EDITOR_PORT` to a different value.
