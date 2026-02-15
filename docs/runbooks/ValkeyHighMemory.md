# ValkeyHighMemory

## What It Means
Valkey (managed Redis-compatible) memory usage has exceeded 80% of the configured `maxmemory`. If memory reaches 100% and no eviction policy is set, Valkey will reject write commands, breaking tenant applications that depend on caching or session storage.

## Severity
Warning at 80% of maxmemory, Critical at 95%. At critical levels, tenants may experience write failures and application errors if eviction is not configured or all keys are non-evictable.

## Likely Causes
1. Tenants storing too many or too-large keys without TTLs
2. No eviction policy configured (default `noeviction` rejects writes when full)
3. Memory fragmentation causing inefficient memory use
4. A single tenant consuming a disproportionate amount of memory
5. Lack of per-tenant memory quotas

## Investigation Steps
1. Check Valkey memory usage:
   ```bash
   ssh ubuntu@<valkey-ip> redis-cli INFO memory | grep -E 'used_memory_human|maxmemory_human|mem_fragmentation_ratio|maxmemory_policy'
   ```
2. Check total number of keys:
   ```bash
   ssh ubuntu@<valkey-ip> redis-cli DBSIZE
   ```
3. Check eviction policy:
   ```bash
   ssh ubuntu@<valkey-ip> redis-cli CONFIG GET maxmemory-policy
   ```
4. Find large keys (sample-based):
   ```bash
   ssh ubuntu@<valkey-ip> redis-cli --bigkeys
   ```
5. Check key expiration stats:
   ```bash
   ssh ubuntu@<valkey-ip> redis-cli INFO keyspace
   ```

## Remediation

### Immediate
- Enable an eviction policy if not set:
  ```bash
  ssh ubuntu@<valkey-ip> redis-cli CONFIG SET maxmemory-policy allkeys-lru
  ```
- Delete unused or large keys if identifiable:
  ```bash
  ssh ubuntu@<valkey-ip> redis-cli DEL <key>
  ```
- Increase maxmemory if the node has available RAM:
  ```bash
  ssh ubuntu@<valkey-ip> redis-cli CONFIG SET maxmemory <bytes>
  ```
- Flush a specific database if it contains only cache data (destructive):
  ```bash
  ssh ubuntu@<valkey-ip> redis-cli -n <db-number> FLUSHDB
  ```

### Long-term
- Implement per-tenant memory quotas and key namespacing
- Require TTLs on cache keys (enforce via application conventions)
- Set an appropriate eviction policy (`allkeys-lru` for caches, `volatile-lru` for mixed workloads)
- Monitor per-tenant key counts and memory usage
- Right-size `maxmemory` based on the node's available RAM and workload
- Add Valkey memory dashboards to Grafana

## Escalation
If Valkey is at 95%+ memory and eviction is already enabled, escalate to the platform team to investigate tenant usage patterns. If tenant applications are failing due to write rejections, coordinate with the tenant support team.
