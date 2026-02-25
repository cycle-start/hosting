# WorkerDown

## What It Means
The Temporal worker pod is unreachable for 1+ minutes. No workflows or activities can be processed, meaning all asynchronous operations (provisioning, convergence, DNS updates) are stalled.

## Severity
Critical -- All async platform operations are blocked. New tenant provisioning, configuration changes, and convergence workflows will not execute until the worker is restored.

## Likely Causes
1. The worker pod has crashed or is in a crash loop
2. The worker lost its connection to the Temporal server
3. The pod was OOM-killed due to memory limits
4. The k3s node evicted the pod due to resource pressure
5. A bad deployment rolled out broken workflow or activity code

## Investigation Steps
1. Check pod status:
   ```bash
   kubectl --context hosting get pods | grep worker
   kubectl --context hosting describe pod -l app=hosting-worker
   ```
2. Check logs for crash reasons:
   ```bash
   kubectl --context hosting logs deployment/hosting-worker --tail=100
   kubectl --context hosting logs deployment/hosting-worker --previous --tail=50
   ```
3. Check if the worker is registered with Temporal:
   - Open Temporal Web UI at `http://temporal.massive-hosting.com`
   - Check task queue workers under the relevant task queues
4. Check Temporal server health:
   ```bash
   kubectl --context hosting get pods | grep temporal
   ```
5. Check pod events:
   ```bash
   kubectl --context hosting get events --sort-by='.lastTimestamp' | grep worker
   ```

## Remediation

### Immediate
- Restart the worker deployment:
  ```bash
  kubectl --context hosting rollout restart deployment/hosting-worker
  ```
- If stuck, delete the pod:
  ```bash
  kubectl --context hosting delete pod -l app=hosting-worker
  ```
- If a bad deploy caused the crash, roll back:
  ```bash
  kubectl --context hosting rollout undo deployment/hosting-worker
  ```
- Verify Temporal connectivity:
  ```bash
  kubectl --context hosting logs deployment/hosting-worker --tail=20 | grep -i "temporal\|connect"
  ```

### Long-term
- Run multiple worker replicas for redundancy
- Set appropriate resource requests and limits in the Helm chart
- Ensure the worker has retry logic for Temporal connection failures
- Add readiness probes tied to Temporal connectivity

## Escalation
If the worker cannot be restored within 5 minutes, escalate to the platform team. If Temporal itself is down, see the [TemporalWorkflowTimeout](TemporalWorkflowTimeout.md) runbook. Stalled workflows will resume automatically once the worker reconnects.
