# Day 90 - Worker Reliability Drill (Retry/Backoff/Dead-Letter)

Goal: harden async job processing by continuously validating failure-path behavior, not only happy paths.

## One-command run

```bash
make day90
```

Equivalent:

```bash
bash ./scripts/day90_worker_reliability.sh
```

## What Day 90 validates

1. Applies latest DB migrations for a consistent test baseline.
2. Runs worker reliability unit tests covering:
   - retry scheduling when attempts remain
   - dead-letter transition when max attempts are exhausted
   - fallback to `MarkFailed` when reschedule write fails
   - no-op behavior when queue is empty
   - exponential backoff jitter bounds
3. Runs DB-backed integration tests for worker pipelines:
   - publish pipeline end-to-end
   - registration confirmation pipeline (send-once/idempotent behavior)
   - registrations CSV export pipeline
4. Captures worker logs and service status for evidence.

## Artifacts produced (`tmp/day90/`)

- `summary.txt`
- `goose_up.txt`
- `worker_reliability_unit_tests.txt`
- `worker_reliability_integration_tests.txt`
- `worker_logs_tail.txt`
- `compose_ps.txt`

## Done criteria

- Script exits successfully.
- Worker unit and integration logs show pass.
- `summary.txt` reports retry/dead-letter coverage and successful pipeline checks.

## Evidence checklist

1. Screenshot `tmp/day90/summary.txt`
2. Screenshot from `tmp/day90/worker_reliability_unit_tests.txt` showing:
   - `TestHandleFailure_SchedulesRetryWhenAttemptsRemain`
   - `TestHandleFailure_DeadLettersWhenAttemptsExhausted`
   - `TestHandleFailure_RescheduleFailureFallsBackToMarkFailed`
3. Screenshot from `tmp/day90/worker_reliability_integration_tests.txt` showing pipeline tests passing
4. Screenshot from `tmp/day90/worker_logs_tail.txt` showing worker lifecycle logs
5. Screenshot of committed `scripts/day90_worker_reliability.sh` and this runbook
