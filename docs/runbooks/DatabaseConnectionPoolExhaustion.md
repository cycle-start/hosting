# DatabaseConnectionPoolExhaustion

## What It Means
The pgx connection pool in core-api is above 80% utilization (warning) or completely exhausted at 100% (critical). When the pool is exhausted, new database queries block waiting for a connection, causing API request latency to spike and eventually timeout.

## Severity
Warning at 80% pool utilization, Critical at 100%. At critical levels, API requests will start timing out and returning 5xx errors.

## Likely Causes
1. Slow PostgreSQL queries holding connections open for too long
2. Connection leaks in application code (connections not returned to pool)
3. High request volume exceeding the pool's capacity
4. PostgreSQL itself is unresponsive or slow (disk I/O, lock contention)
5. Pool size configured too small for the workload

## Investigation Steps
1. Check pool metrics in Prometheus:
   - Query: `pgxpool_acquired_connections` and `pgxpool_total_connections`
   - Query: `pgxpool_acquire_duration_seconds` for connection wait times
2. Check PostgreSQL active connections:
   ```bash
   kubectl --context hosting exec -it statefulset/postgresql -- psql -U hosting -c "SELECT count(*), state FROM pg_stat_activity GROUP BY state;"
   ```
3. Check for long-running queries:
   ```bash
   kubectl --context hosting exec -it statefulset/postgresql -- psql -U hosting -c "SELECT pid, now() - query_start AS duration, state, query FROM pg_stat_activity WHERE state != 'idle' ORDER BY duration DESC LIMIT 10;"
   ```
4. Check for lock contention:
   ```bash
   kubectl --context hosting exec -it statefulset/postgresql -- psql -U hosting -c "SELECT blocked_locks.pid AS blocked_pid, blocking_locks.pid AS blocking_pid, blocked_activity.query AS blocked_query FROM pg_locks blocked_locks JOIN pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid JOIN pg_locks blocking_locks ON blocking_locks.locktype = blocked_locks.locktype WHERE NOT blocked_locks.granted;"
   ```
5. Check core-api logs for timeout errors:
   ```bash
   kubectl --context hosting logs deployment/hosting-core-api --tail=100 | grep -i "pool\|timeout\|connection"
   ```

## Remediation

### Immediate
- Restart core-api to reset the connection pool:
  ```bash
  kubectl --context hosting rollout restart deployment/hosting-core-api
  ```
- Kill long-running queries if they are blocking the pool:
  ```bash
  kubectl --context hosting exec -it statefulset/postgresql -- psql -U hosting -c "SELECT pg_terminate_backend(<pid>);"
  ```
- Temporarily increase the pool size via environment variable:
  ```bash
  kubectl --context hosting set env deployment/hosting-core-api DATABASE_POOL_MAX_CONNS=30
  ```

### Long-term
- Tune pool size based on observed usage patterns (`DATABASE_POOL_MAX_CONNS` in config)
- Add query timeouts (`statement_timeout` in PostgreSQL)
- Optimize slow queries and add missing indexes
- Consider adding PgBouncer as a connection pooler between core-api and PostgreSQL
- Implement connection pool metrics dashboards with alerting at lower thresholds
- Review code for connection leaks (ensure all queries use proper context cancellation)

## Escalation
If the pool is at 100% and API is returning errors, escalate to the platform team. If PostgreSQL itself is unresponsive, coordinate with the database team for investigation.
