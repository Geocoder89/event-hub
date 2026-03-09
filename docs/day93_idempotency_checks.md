# Day 93 - Idempotency Checks (Publish, Export, Check-In)

Goal: verify duplicate requests are safe and deterministic for key admin workflows.

## One-command run

```bash
make day93
```

Equivalent:

```bash
bash ./scripts/day93_idempotency_checks.sh
```

## What Day 93 validates

1. Duplicate event publish enqueue:
   - second request returns same `jobId`
   - response marks `alreadyEnqueued=true`
   - only one publish job row exists
2. Duplicate registrations CSV export enqueue:
   - second request returns same `jobId`
   - response marks `alreadyEnqueued=true`
   - only one export job row exists for event
3. Duplicate registration check-in:
   - first request succeeds
   - second request returns `409` with `already_checked_in`
4. Pipeline regression safety:
   - publish end-to-end
   - registrations export enqueue/process/download

## Artifacts produced (`tmp/day93/`)

- `summary.txt`
- `goose_up.txt`
- `idempotency_integration_tests.txt`
- `pipeline_regression_tests.txt`
- `compose_ps.txt`
- `api_logs_tail.txt`
- `worker_logs_tail.txt`

## Done criteria

- Script exits successfully.
- Idempotency integration tests and regression tests pass.
- Summary confirms duplicate-request behavior is stable.

## Evidence checklist

1. Screenshot `tmp/day93/summary.txt`
2. Screenshot from `tmp/day93/idempotency_integration_tests.txt` showing:
   - `TestPublishPipeline_IdempotentEnqueue`
   - `TestPipeline_RegistrationsCSVExport_IdempotentEnqueue`
   - `TestRegistrationCheckInIntegration_AlreadyCheckedIn`
3. Screenshot from `tmp/day93/pipeline_regression_tests.txt` showing:
   - `TestPublishPipeline_EndToEnd`
   - `TestPipeline_RegistrationsCSVExport_EnqueueProcessDownload`
4. Screenshot of committed:
   - `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/scripts/day93_idempotency_checks.sh`
   - `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/docs/day93_idempotency_checks.md`
