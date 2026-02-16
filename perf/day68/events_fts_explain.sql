-- Postgres full-text-search plan checks for EventHub events.
-- Run with: psql "$DATABASE_URL" -f perf/day68/events_fts_explain.sql

-- Keep table stats fresh before testing.
ANALYZE events;

-- 1) FTS-only lookup (expects use of idx_events_fts).
EXPLAIN (ANALYZE, BUFFERS)
SELECT id, title, description, city, start_at, capacity, created_at, updated_at
FROM events
WHERE to_tsvector('simple', coalesce(title,'') || ' ' || coalesce(description,'') || ' ' || coalesce(city,'')) @@ websearch_to_tsquery('simple', 'go backend')
ORDER BY start_at ASC, id ASC
LIMIT 20;

-- 2) FTS + city + date window (matches API filter style).
EXPLAIN (ANALYZE, BUFFERS)
SELECT id, title, description, city, start_at, capacity, created_at, updated_at
FROM events
WHERE city = 'lagos'
  AND start_at >= NOW() - INTERVAL '30 days'
  AND start_at <= NOW() + INTERVAL '180 days'
  AND to_tsvector('simple', coalesce(title,'') || ' ' || coalesce(description,'') || ' ' || coalesce(city,'')) @@ websearch_to_tsquery('simple', 'distributed systems')
ORDER BY start_at ASC, id ASC
LIMIT 20;

-- 3) Count query for includeTotal=true path.
EXPLAIN (ANALYZE, BUFFERS)
SELECT COUNT(*)
FROM events
WHERE to_tsvector('simple', coalesce(title,'') || ' ' || coalesce(description,'') || ' ' || coalesce(city,'')) @@ websearch_to_tsquery('simple', 'kubernetes workshop');
