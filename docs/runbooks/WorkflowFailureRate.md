# WorkflowFailureRate

## What It Means
The rate of workflow failures is above 5% or the rate of activity failures is above 10%. This means platform operations (tenant provisioning, DNS updates, convergence) are failing at an abnormal rate, and some tenants may be stuck in provisioning or inconsistent states.

## Severity
Warning at 5% workflow failure or 10% activity failure rate. Critical if the rate continues to climb, indicating a systemic issue.

## Likely Causes
1. Node agents are unreachable (node VMs down or node-agent process crashed)
2. Bugs in activity code introduced by a recent deploy
3. External service failures (DNS server, database, Ceph, Stalwart)
4. Configuration errors (bad environment variables, wrong credentials)
5. Temporal worker is overwhelmed and activities are timing out

## Investigation Steps
1. Check Temporal Web UI for failing workflows:
   - Open `http://temporal.hosting.test`
   - Filter by status "Failed" or "TimedOut"
   - Inspect the specific activity that failed and its error message
2. Check worker logs for errors:
   ```bash
   kubectl --context hosting logs deployment/hosting-worker --tail=200 | grep -i "error\|fail\|panic"
   ```
3. Check node-agent status on affected nodes:
   ```bash
   ssh ubuntu@<node-ip> systemctl status node-agent
   ssh ubuntu@<node-ip> journalctl -u node-agent --since '30 minutes ago' --no-pager | tail -50
   ```
4. Check if specific task queues have no workers:
   - In Temporal Web UI, navigate to task queues and verify pollers exist
5. Check recent deployments:
   ```bash
   kubectl --context hosting rollout history deployment/hosting-worker
   ```

## Remediation

### Immediate
- If node agents are unreachable, restart them:
  ```bash
  ssh ubuntu@<node-ip> sudo systemctl restart node-agent
  ```
- If a bad deploy caused activity failures, roll back:
  ```bash
  kubectl --context hosting rollout undo deployment/hosting-worker
  ```
- If configuration is wrong, fix it and restart:
  ```bash
  kubectl --context hosting get configmap hosting-config -o yaml
  # Fix the config, then:
  kubectl --context hosting rollout restart deployment/hosting-worker
  ```
- Retry failed workflows from the Temporal Web UI if the root cause is fixed

### Long-term
- Implement comprehensive activity error handling with meaningful error messages
- Add integration tests for all activities
- Set appropriate retry policies with exponential backoff for transient failures
- Add per-activity success rate dashboards
- Implement circuit breakers for external service calls in activities

## Escalation
If the failure rate is above 20% or affecting a large number of tenants, escalate to the platform team immediately. If failures are specific to a node or shard, coordinate with the infrastructure team.
