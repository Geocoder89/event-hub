# Day 87 - Migration Safety Drill (Up/Down/Up)

Goal: verify migrations are reversible and deterministic on a clean scratch database.

## One-command run

```bash
make day87
```

Equivalent:

```bash
bash ./scripts/day87_migration_safety.sh
```

## What the drill validates

1. Creates a fresh scratch DB (`eventhub_migration_safety`).
2. Runs `goose up` to latest.
3. Captures schema snapshot #1.
4. Runs `goose reset` (down all migrations).
5. Runs `goose up` again to latest.
6. Captures schema snapshot #2.
7. Diffs normalized schema snapshots to detect drift.
8. Verifies no `Pending` migrations remain after second `up`.

## Artifacts produced (`tmp/day87/`)

- `summary.txt`
- `goose_up_1.txt`
- `goose_reset.txt`
- `goose_up_2.txt`
- `goose_status_after_up_1.txt`
- `goose_status_after_reset.txt`
- `goose_status_after_up_2.txt`
- `schema_up_1.sql`
- `schema_up_2.sql`
- `schema_up_1.normalized.sql`
- `schema_up_2.normalized.sql`
- `schema_diff.txt` (empty when no drift)
- `compose_ps.txt`

## Done criteria

- Script exits successfully.
- `schema_diff.txt` is empty.
- `goose_status_after_up_2.txt` has no pending migrations.
- `summary.txt` reports drift check passed.

## Evidence checklist

1. Screenshot `summary.txt`.
2. Screenshot `goose_status_after_up_2.txt` showing all applied.
3. Screenshot `schema_diff.txt` empty.
4. Screenshot of normalized schema hashes from `summary.txt`.
5. Screenshot of committed script + runbook.
