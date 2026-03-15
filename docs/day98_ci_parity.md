# Day 98 - Local CI Parity

Goal: run the same core quality, security, test, and Docker build gates locally with one command, using a dedicated temporary database and step-level artifacts.

## One-command run

```bash
make day98
```

Equivalent:

```bash
bash ./scripts/day98_ci_parity.sh
```

## What Day 98 verifies

1. Starts local `db` and `redis` services.
2. Creates a dedicated temporary test database for the run.
3. Applies migrations to that database.
4. Runs formatting, vet, binary builds, tests, lint, `gosec`, and `govulncheck`.
5. Builds Docker `api` and `worker` targets locally.

## CI-parity gates covered

- `go fmt ./...`
- `go vet ./...`
- `go build -v ./cmd/api`
- `go build -v ./cmd/worker`
- `go test ./... -v`
- `golangci-lint run ./...`
- `golangci-lint run --no-config --enable-only gosec ./...`
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
- `docker build --target api`
- `docker build --target worker`

## Artifacts produced (`tmp/day98/`)

- `summary.txt`
- `create_temp_db.txt`
- `goose_up.txt`
- `fmt.txt`
- `vet.txt`
- `build_binaries.txt`
- `test.txt`
- `lint.txt`
- `gosec.txt`
- `govuln.txt`
- `docker_build_api.txt`
- `docker_build_worker.txt`
- `docker_images.txt`
- `compose_ps.txt`

## Done criteria

- Script exits successfully.
- All CI-parity gates pass.
- Summary file lists a green result for each local gate.
- Docker `api` and `worker` target images are built locally.

## Evidence checklist

1. Screenshot `tmp/day98/summary.txt`
2. Screenshot `tmp/day98/test.txt` tail showing `PASS`
3. Screenshot `tmp/day98/govuln.txt` showing `No vulnerabilities found.`
4. Screenshot `tmp/day98/docker_build_api.txt` showing successful export
5. Screenshot `tmp/day98/docker_build_worker.txt` showing successful export
6. Screenshot `tmp/day98/docker_images.txt` showing `eventhub-api:day98-local` and `eventhub-worker:day98-local`
