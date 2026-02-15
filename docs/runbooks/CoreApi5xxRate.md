# CoreApi5xxRate

## What It Means
The core-api is returning 5xx errors at a rate above 1% (warning) or 5% (critical) of all requests. This means API consumers (admin UI, automation, hostctl) are experiencing failures.

## Severity
Warning at 1% error rate, Critical at 5%. At critical levels, the platform is largely unusable for management operations.

## Likely Causes
1. PostgreSQL database connection failure or pool exhaustion
2. Temporal server unavailable, causing workflow start failures
3. Unhandled panic or bug in handler code (often after a bad deploy)
4. A bad deployment with broken code
5. Resource exhaustion on the core-api pod (OOM, CPU throttling)

## Investigation Steps
1. Check core-api logs for error patterns:
   ```bash
   kubectl --context hosting logs deployment/hosting-core-api --tail=200 | grep -i "error\|panic\|500"
   ```
2. Check database connectivity:
   ```bash
   kubectl --context hosting exec deployment/hosting-core-api -- env | grep DATABASE
   kubectl --context hosting get pods | grep postgres
   ```
3. Check Temporal connectivity:
   ```bash
   kubectl --context hosting logs deployment/hosting-core-api --tail=50 | grep -i temporal
   ```
4. Check recent deployments:
   ```bash
   kubectl --context hosting rollout history deployment/hosting-core-api
   ```
5. Check error rate in Prometheus:
   - Query: `rate(http_requests_total{service="core-api", status=~"5.."}[5m]) / rate(http_requests_total{service="core-api"}[5m])`

## Remediation

### Immediate
- If caused by a bad deploy, roll back:
  ```bash
  kubectl --context hosting rollout undo deployment/hosting-core-api
  ```
- If database is unreachable, check and restart PostgreSQL:
  ```bash
  kubectl --context hosting get pods | grep postgres
  kubectl --context hosting logs statefulset/postgresql --tail=50
  ```
- Restart core-api to clear transient connection issues:
  ```bash
  kubectl --context hosting rollout restart deployment/hosting-core-api
  ```

### Long-term
- Implement circuit breakers for downstream dependencies (Temporal, DB)
- Add structured error logging with request correlation IDs
- Implement canary deployments to catch errors before full rollout
- Add integration tests that run before deploy
- Ensure database connection pool is properly sized (see [DatabaseConnectionPoolExhaustion](DatabaseConnectionPoolExhaustion.md))

## Escalation
If the 5xx rate is above 5% and the cause is not a simple rollback, escalate to the platform team. If the root cause is database-related, involve the database team.
