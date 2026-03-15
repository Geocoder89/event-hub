# Day 97 - Security Abuse Drill

Goal: validate defensive behavior for auth abuse patterns (rate-limit abuse, refresh misuse, stale-token replay, and invalid access token usage).

## One-command run

```bash
make day97
```

Equivalent:

```bash
bash ./scripts/day97_security_abuse.sh
```

## What Day 97 verifies

1. Starts local stack (`db`, `redis`, `jaeger`, `api`, `worker`) and applies migrations.
2. Sends 6 invalid login attempts from same client and confirms rate limiting.
3. Calls refresh endpoint with no cookie and confirms rejection.
4. Calls refresh endpoint with malformed cookie and confirms rejection.
5. Creates a real user session and validates refresh-token rotation replay protection.
6. Calls an authenticated endpoint with an invalid access token and confirms rejection.

## Expected security contracts

- Login attempts 1-5: `401` with `error.code=invalid_credentials`
- Login attempt 6: `429` with `error.code=rate_limited` (+ `Retry-After` header)
- `POST /auth/refresh` without cookie: `401` with `error.code=no_refresh`
- `POST /auth/refresh` with malformed cookie: `401` with `error.code=invalid_refresh`
- Replay old refresh token after rotation: `401` with `error.code=invalid_refresh`
- Invalid access token on auth route: `401` with `error.code=unauthorized`

## Artifacts produced (`tmp/day97/`)

- `summary.txt`
- `goose_up.txt`
- `rate_limit_login_1.headers.txt` ... `rate_limit_login_6.headers.txt`
- `rate_limit_login_1.body.json` ... `rate_limit_login_6.body.json`
- `refresh_no_cookie.headers.txt`
- `refresh_no_cookie.body.json`
- `refresh_bad_cookie.headers.txt`
- `refresh_bad_cookie.body.json`
- `signup.headers.txt`
- `signup.body.json`
- `refresh_rotate_first.headers.txt`
- `refresh_rotate_first.body.json`
- `refresh_rotate_replay_old.headers.txt`
- `refresh_rotate_replay_old.body.json`
- `invalid_access_token.headers.txt`
- `invalid_access_token.body.json`
- `compose_ps.txt`
- `api_logs_tail.txt`

## Done criteria

- Script exits successfully.
- All negative scenarios return expected status/error code combinations.
- Summary file reports all abuse checks as passed.

## Evidence checklist

1. Screenshot `tmp/day97/summary.txt`
2. Screenshot `tmp/day97/rate_limit_login_6.headers.txt` showing `429`
3. Screenshot `tmp/day97/rate_limit_login_6.body.json` showing `rate_limited`
4. Screenshot `tmp/day97/refresh_no_cookie.body.json` showing `no_refresh`
5. Screenshot `tmp/day97/refresh_rotate_replay_old.body.json` showing `invalid_refresh`
6. Screenshot `tmp/day97/invalid_access_token.body.json` showing `unauthorized`
