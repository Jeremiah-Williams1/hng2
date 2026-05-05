# SOLUTION.md — Stage 4B

## Part 1 — Query Performance

### Indexes
Added indexes on the four columns `BuildProfileQuery` filters on:

```sql
CREATE INDEX IF NOT EXISTS idx_profiles_gender ON profiles(gender);
CREATE INDEX IF NOT EXISTS idx_profiles_country_id ON profiles(country_id);
CREATE INDEX IF NOT EXISTS idx_profiles_age ON profiles(age);
CREATE INDEX IF NOT EXISTS idx_profiles_age_group ON profiles(age_group);
```

Added to `InitializeSchema()` with `IF NOT EXISTS` so they run once safely on every server start. Without indexes, Postgres does a full table scan on every filtered query. With indexes, it jumps directly to matching rows.

### In-Memory Cache
Added a `QueryCache` struct — a map wrapped in a `sync.RWMutex` — in `cache/cache.go`. Before any query hits the database, the handler checks the cache. On a hit, the cached response is returned immediately. On a miss, the query runs and the result is stored.

`RWMutex` over plain `Mutex` because read traffic dominates. Multiple goroutines can hold a read lock simultaneously. A write lock is only acquired when storing a new entry.

TTL is 2 minutes. A background goroutine wakes every 5 minutes and evicts expired entries so the map doesn't grow forever.

No Redis — the constraint was no new infrastructure.

---

## Part 2 — Query Normalization

Before building the cache key, filter params are collected into a map, empty values are dropped, and the keys are sorted alphabetically:

```go
sort.Strings(keys)
for _, k := range keys {
    if v != "" {
        result += fmt.Sprintf("%s=%s;", k, v)
    }
}
```

Sorting makes the key deterministic regardless of map iteration order. Dropping empty values means `?gender=female` and `?gender=female&country_id=` produce the same key. Values are already lowercased upstream so case differences are handled before the key is built.

Two queries with different wording but the same parsed filters will produce the same cache key and share the same cached result.

---

## Part 3 — CSV Ingestion

Endpoint: `POST /api/profiles/upload` accepts a multipart form with a `file` field.

**Streaming:** `csv.NewReader` reads one row at a time. The entire file is never loaded into memory. Memory usage stays constant regardless of file size.

**Chunked inserts:** Rows are batched into groups of 500 and inserted with a single bulk `INSERT ... ON CONFLICT (name) DO NOTHING`. At 500k rows this is 1000 DB round trips instead of 500,000. `ON CONFLICT DO NOTHING` handles duplicates atomically at the DB level using the existing `UNIQUE` constraint on `name`.

**Per-row validation:** Each row is validated in Go before being added to the chunk. Bad rows are skipped and counted, never sent to the DB.

**No rollback on partial failure:** Following the requirement — rows already inserted remain if processing fails midway. Each chunk is an independent operation.

**Cache invalidation:** After upload completes, the entire cache is invalidated so the next read reflects the new data.

### Edge Cases

| Case | Handling |
|---|---|
| Missing required column in header | Reject file, 400 |
| Wrong column count | Skip row, `missing_fields` |
| Empty required field | Skip row, `missing_fields` |
| Invalid or negative age | Skip row, `invalid_age` |
| Gender not male/female | Skip row, `invalid_gender` |
| Duplicate name | Skip row, `duplicate_name` |
| Malformed CSV row | Skip row, `malformed_row` |
| Chunk-level DB error | Skip chunk, `db_error` |