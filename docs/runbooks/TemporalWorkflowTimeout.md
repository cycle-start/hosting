# TemporalWorkflowTimeout / TemporalDown

## What It Means
The Temporal server is unreachable or workflows are timing out. This blocks all asynchronous platform operations: provisioning, convergence, DNS updates, certificate management, and more. The `TemporalDown` alert fires when the Temporal server itself is unreachable; `TemporalWorkflowTimeout` fires when individual workflows are exceeding their timeouts.

## Severity
Critical -- All async platform operations are stalled. Existing tenant services continue to run but no changes can be made, and convergence stops.

## Likely Causes
1. Temporal server crashed or its pod was evicted
2. Temporal's backing database (PostgreSQL) is unavailable or slow
3. Network connectivity issues between Temporal and its clients (core-api, worker)
4. Long-running activities blocking workflow execution
5. Temporal's internal services (frontend, history, matching) degraded

## Investigation Steps
1. Check Temporal pod status:
   ```bash
   kubectl --context hosting get pods | grep temporal
   kubectl --context hosting describe pod -l app=temporal
   ```
2. Check Temporal logs:
   ```bash
   kubectl --context hosting logs statefulset/temporal --tail=100
   ```
3. Check Temporal Web UI:
   - Open `http://temporal.hosting.test`
   - Look for failing workflows, stuck activities, or high latency
4. Check Temporal's database:
   ```bash
   kubectl --context hosting get pods | grep postgres
   kubectl --context hosting logs statefulset/postgresql --tail=50
   ```
5. Check Temporal service health:
   ```bash
   kubectl --context hosting exec statefulset/temporal -- tctl cluster health
   ```

## Remediation

### Immediate
- Restart Temporal:
  ```bash
  kubectl --context hosting rollout restart statefulset/temporal
  ```
- If Temporal's database is down, restart it first:
  ```bash
  kubectl --context hosting rollout restart statefulset/postgresql
  ```
  Wait for PostgreSQL to be ready, then restart Temporal.
- Check and fix network connectivity:
  ```bash
  kubectl --context hosting exec deployment/hosting-worker -- nslookup temporal.hosting.test
  ```

### Long-term
- Deploy Temporal in HA mode (multiple history/matching/frontend services)
- Set appropriate activity and workflow timeouts to prevent indefinite hangs
- Ensure Temporal's database has proper resource allocation
- Add Temporal-specific health checks and dashboards
- Implement workflow retry policies with exponential backoff

## Escalation
If Temporal cannot be restored within 5 minutes, escalate to the platform team immediately. Stalled workflows will automatically resume once Temporal is back, but prolonged outages can cause cascading issues with provisioning and convergence.
