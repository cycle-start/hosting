# TemporalTaskQueueBacklog

## What It Means
A Temporal task queue has 100+ pending tasks or high schedule-to-start latency. This means workflows and activities are queued but not being picked up by workers fast enough, causing delays in all platform operations.

## Severity
Warning at 100 pending tasks or schedule-to-start latency above 30s. Critical at 500+ pending tasks or latency above 2 minutes. Platform operations are delayed proportionally to the backlog.

## Likely Causes
1. Worker pod is under-provisioned (not enough goroutines or too few replicas)
2. Worker pod crashed and tasks are accumulating
3. Activities are taking too long, blocking worker capacity
4. A node-agent is down, causing its `node-{id}` task queue to back up
5. A burst of operations (e.g., mass provisioning) created more work than workers can handle

## Investigation Steps
1. Check Temporal Web UI task queues:
   - Open `http://temporal.massive-hosting.com`
   - Navigate to task queues and check pending task counts and pollers
2. Check worker pod status:
   ```bash
   kubectl --context hosting get pods | grep worker
   kubectl --context hosting logs deployment/hosting-worker --tail=50
   ```
3. Check which task queues are backed up:
   - In Temporal Web UI, check both the main task queue and individual `node-{id}` queues
4. If a node-specific queue is backed up, check the node-agent:
   ```bash
   ssh ubuntu@<node-ip> systemctl status node-agent
   ssh ubuntu@<node-ip> journalctl -u node-agent --since '30 minutes ago' --no-pager | tail -30
   ```
5. Check for long-running activities:
   - In Temporal Web UI, look for activities with long execution times

## Remediation

### Immediate
- If the worker is down, restart it:
  ```bash
  kubectl --context hosting rollout restart deployment/hosting-worker
  ```
- If a node-agent is down, restart it:
  ```bash
  ssh ubuntu@<node-ip> sudo systemctl restart node-agent
  ```
- If activities are stuck, check and fix the underlying issue (e.g., external service down)
- If the backlog is due to a burst, wait for it to drain (monitor the queue size)

### Long-term
- Scale worker replicas based on observed task queue depth
- Implement auto-scaling for workers based on queue backlog metrics
- Set appropriate activity timeouts to prevent workers from being blocked
- Implement rate limiting for bulk operations to prevent queue flooding
- Add task queue depth dashboards and alerts at lower thresholds for early warning

## Escalation
If the backlog is growing and not draining after investigation, escalate to the platform team. If a node-specific queue is backed up and the node cannot be reached, see the [NodeDown](NodeDown.md) runbook.
