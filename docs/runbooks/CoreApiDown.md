# CoreApiDown

## What It Means
The core-api service is unreachable for 1+ minutes. No API requests can be processed, meaning the admin UI is down and no management operations (tenant creation, DNS changes, etc.) can be performed.

## Severity
Critical -- All API-driven operations are blocked. Existing tenant sites continue to work but no changes can be made to the platform.

## Likely Causes
1. The core-api pod has crashed or is in a crash loop
2. The pod was OOM-killed due to memory limits
3. A port conflict on the host network (core-api uses hostNetwork)
4. The k3s node evicted the pod due to resource pressure
5. A bad deployment rolled out broken code

## Investigation Steps
1. Check pod status:
   ```bash
   kubectl --context hosting get pods | grep core-api
   kubectl --context hosting describe pod -l app=hosting-core-api
   ```
2. Check logs for crash reasons:
   ```bash
   kubectl --context hosting logs deployment/hosting-core-api --tail=100
   kubectl --context hosting logs deployment/hosting-core-api --previous --tail=50
   ```
3. Check pod events:
   ```bash
   kubectl --context hosting get events --sort-by='.lastTimestamp' | grep core-api
   ```
4. Check if the port is available:
   ```bash
   ssh ubuntu@10.10.10.2 ss -tlnp | grep 8080
   ```
5. Check k3s node resources:
   ```bash
   kubectl --context hosting top nodes
   kubectl --context hosting top pods
   ```

## Remediation

### Immediate
- Restart the deployment:
  ```bash
  kubectl --context hosting rollout restart deployment/hosting-core-api
  ```
- If the pod is stuck, delete it and let the deployment recreate it:
  ```bash
  kubectl --context hosting delete pod -l app=hosting-core-api
  ```
- If a bad deploy caused the crash, roll back:
  ```bash
  kubectl --context hosting rollout undo deployment/hosting-core-api
  ```

### Long-term
- Set appropriate resource requests and limits in the Helm chart
- Add a PodDisruptionBudget if running multiple replicas
- Ensure health check probes (`/healthz`, `/readyz`) are configured correctly
- Implement pre-deploy smoke tests to catch broken builds before rollout

## Escalation
If the core-api cannot be restored within 5 minutes, escalate to the platform team. If the issue is related to database connectivity (PostgreSQL down), see database-related runbooks and escalate accordingly.
