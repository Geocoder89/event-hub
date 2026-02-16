# Day 68 - Events FTS Index + EXPLAIN Notes

## Where the GIN index lives

- Migration: `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/db/migrations/20260212113000_add_events_fts_index.sql`
- Index name: `idx_events_fts`
- Expression:

```sql
to_tsvector('simple', coalesce(title,'') || ' ' || coalesce(description,'') || ' ' || coalesce(city,''))
```

## Where the query uses it

- Repository filter conditions are in:
  - `/Users/oladelemoarukhe/Documents/codes/event-hub/eventhub/internal/repo/postgres/events_repo.go`
- FTS predicate used:

```sql
... @@ websearch_to_tsquery('simple', $n)
```

## Where to run EXPLAIN ANALYZE

Run against the same Postgres database used by the API:

```bash
cd /Users/oladelemoarukhe/Documents/codes/event-hub/eventhub
psql "$DATABASE_URL" -f perf/day68/events_fts_explain.sql
```

## What to look for in the plan

- Preferred nodes:
  - `Bitmap Index Scan on idx_events_fts`
  - `Bitmap Heap Scan` (or `Index Scan` if very selective)
- Acceptable with sorting:
  - `Sort` node before `LIMIT` when needed for `ORDER BY start_at, id`
- Check perf signals:
  - low `actual time`
  - low shared/read buffers for repeated warm-cache runs

If the planner uses `Seq Scan` for selective terms, check:

1. table statistics (`ANALYZE events;`)
2. data volume/selectivity (tiny tables often seq-scan by design)
3. that migration has been applied (`idx_events_fts` exists)
