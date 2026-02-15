# CoreApiHighLatency

## What It Means
The core-api P99 response latency has exceeded 2 seconds (warning) or 5 seconds (critical). API consumers are experiencing slow responses, which degrades the admin UI experience and can cause timeouts in automated tooling.

## Severity
Warning at P99 > 2s, Critical at P99 > 5s. At critical levels, API clients may time out and retry, amplifying load.

## Likely Causes
1. Slow PostgreSQL queries (missing indexes, lock contention, large result sets)
2. Temporal server latency affecting workflow start operations
3. Resource contention on the k3s control plane node (CPU or memory pressure)
4. Network latency between core-api and its dependencies
5. High request volume causing queuing

## Investigation Steps
1. Check core-api logs for slow requests:
   ```bash
   kubectl --context hosting logs deployment/hosting-core-api --tail=200 | grep -i "slow\|latency\|timeout"
   ```
2. Check PostgreSQL for slow queries:
   ```bash
   kubectl --context hosting exec -it statefulset/postgresql -- psql -U hosting -c "SELECT pid, now() - pg_stat_activity.query_start AS duration, query FROM pg_stat_activity WHERE state = 'active' ORDER BY duration DESC LIMIT 10;"
   ```
3. Check core-api pod resource usage:
   ```bash
   kubectl --context hosting top pods | grep core-api
   ```
4. Check Temporal task queue latency:
   - Open Temporal Web UI at `http://temporal.hosting.test`
   - Check schedule-to-start latency on task queues
5. Check control plane node resources:
   ```bash
   kubectl --context hosting top nodes
   ```

## Remediation

### Immediate
- If a specific slow query is identified, check for missing indexes
- Restart core-api to clear potential connection pool issues:
  ```bash
  kubectl --context hosting rollout restart deployment/hosting-core-api
  ```
- If the control plane node is resource-constrained, identify and reduce load:
  ```bash
  kubectl --context hosting top pods --sort-by=cpu
  ```

### Long-term
- Add database query logging with slow query threshold
- Optimize frequently-hit queries and add appropriate indexes
- Tune the pgx connection pool size based on observed usage
- Implement response caching for read-heavy endpoints
- Add request timeout middleware to prevent unbounded request processing
- Consider PgBouncer for connection pooling at the database level

## Escalation
If latency remains above 5s after investigation, escalate to the platform team. If the root cause is PostgreSQL performance, involve the database team for query optimization.
