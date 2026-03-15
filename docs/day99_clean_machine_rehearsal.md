# Day 99 - Clean-Machine Rehearsal

Goal: simulate a first-run setup from a clean workspace copy, bring the full stack up in isolation, apply migrations, and pass smoke checks with an exact reproducible command list.

## One-command run

```bash
make day99
```

Equivalent:

```bash
bash ./scripts/day99_clean_machine_rehearsal.sh
```

## What Day 99 verifies

1. Copies the current workspace into a clean rehearsal directory with no `.git`, `tmp`, or local editor state.
2. Generates a runnable `.env` in that rehearsal copy using clean-machine-safe local defaults.
3. Starts an isolated Docker stack from the rehearsal copy using a Day 99 compose override.
4. Builds fresh API and worker Docker images from the rehearsal copy.
5. Applies migrations from the rehearsal copy against the isolated Day 99 database.
6. Runs the existing Day 83 readiness smoke from that rehearsal copy.
7. Writes the exact command sequence needed to reproduce the same setup manually.

## Isolation model

- Rehearsal workspace: `/tmp/eventhub-day99-rehearsal`
- Compose project: `eventhub-day99`
- API host port: `18080`
- Worker host port: `18081`
- DB host port: `15433`
- Redis host port: `16379`
- Jaeger UI host port: `16687`
- Prometheus host port: `19090`
- Grafana host port: `13001`

## Files added for Day 99

- `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/docker-compose.day99.yml`
- `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/scripts/day99_clean_machine_rehearsal.sh`

## Artifacts produced (`tmp/day99/`)

- `summary.txt`
- `rehearsal_commands.txt`
- `rehearsal_workspace.txt`
- `compose_project_name.txt`
- `compose_up_infra.txt`
- `compose_up_app.txt`
- `docker_build_api.txt`
- `docker_build_worker.txt`
- `goose_up.txt`
- `day83_rehearsal_run.txt`
- `compose_ps.txt`
- `api_logs_tail.txt`
- `worker_logs_tail.txt`
- `day83_rehearsal/summary.txt`
- `day83_rehearsal/compose_ps.txt`
- `day83_rehearsal/api_logs_tail.txt`
- `day83_rehearsal/worker_logs_tail.txt`

## Done criteria

- Rehearsal stack boots from the clean copy.
- Fresh API and worker images build from the clean copy.
- Migrations apply successfully from the clean copy.
- Day 83 smoke passes against the isolated clean-machine environment.
- Reproducible command list is written and usable.

## Evidence checklist

1. Screenshot `tmp/day99/summary.txt`
2. Screenshot `tmp/day99/rehearsal_commands.txt`
3. Screenshot `tmp/day99/day83_rehearsal/summary.txt`
4. Screenshot `tmp/day99/compose_ps.txt`
5. Screenshot `tmp/day99/api_logs_tail.txt`
6. Screenshot `tmp/day99/worker_logs_tail.txt`
