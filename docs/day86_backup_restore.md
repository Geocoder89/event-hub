# Day 86 - Local Backup and Restore Drill

Goal: prove local disaster-recovery readiness by backing up Postgres and restoring into a fresh database, then verifying data consistency.

## One-command run

```bash
make day86
```

Equivalent:

```bash
bash ./scripts/day86_backup_restore.sh
```

## What the drill does

1. Ensures Postgres service is running.
2. Captures source table row counts (`users`, `events`, `registrations`, `jobs`).
3. Creates SQL backup via `pg_dump`.
4. Recreates temporary restore DB (`eventhub_restore_test` by default).
5. Restores backup into the temporary DB.
6. Captures restored row counts and diffs against source counts.
7. Writes summary and evidence artifacts to `tmp/day86/`.

## Artifacts produced

- `tmp/day86/summary.txt`
- `tmp/day86/source_counts.tsv`
- `tmp/day86/restored_counts.tsv`
- `tmp/day86/counts_diff.txt` (empty when matched)
- `tmp/day86/restore_output.txt`
- `tmp/day86/restored_events_sample.csv`
- `tmp/day86/compose_ps.txt`
- backup SQL file: `tmp/day86/eventhub_backup_<timestamp>.sql`

## Done criteria

- Script exits successfully.
- `summary.txt` indicates row counts matched.
- Restore DB contains expected sample data.

## Evidence checklist

1. Screenshot `tmp/day86/summary.txt` (backup size/hash + duration).
2. Screenshot of backup file present in `tmp/day86/`.
3. Screenshot of `source_counts.tsv` and `restored_counts.tsv` matching.
4. Screenshot of `counts_diff.txt` empty (or no diff).
5. Screenshot of `restore_output.txt` completion.
