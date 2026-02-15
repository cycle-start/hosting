# HighMemoryUsage

## What It Means
A node's memory usage has exceeded 90% (warning) or 97% (critical). At high memory usage, the Linux OOM killer may terminate processes, causing service disruptions for tenants on this node.

## Severity
Warning at 90%, Critical at 97%. At critical levels the OOM killer will start terminating processes, which can take down tenant sites, database connections, or the node-agent itself.

## Likely Causes
1. Memory leak in a long-running process (PHP-FPM worker, node-agent, nginx)
2. High tenant load causing many concurrent PHP-FPM workers
3. Insufficient memory allocated to the VM for its workload
4. Filesystem cache growth (usually harmless -- check available vs. free memory)

## Investigation Steps
1. Check overall memory usage:
   ```bash
   ssh ubuntu@<node-ip> free -h
   ```
2. Identify top memory consumers:
   ```bash
   ssh ubuntu@<node-ip> ps aux --sort=-rss | head -20
   ```
3. Check for recent OOM kills:
   ```bash
   ssh ubuntu@<node-ip> dmesg | grep -i "out of memory"
   ssh ubuntu@<node-ip> journalctl -k | grep -i oom
   ```
4. Check PHP-FPM worker count (web nodes):
   ```bash
   ssh ubuntu@<node-ip> ps aux | grep php-fpm | wc -l
   ```
5. Check memory breakdown:
   ```bash
   ssh ubuntu@<node-ip> cat /proc/meminfo | grep -E 'MemTotal|MemFree|MemAvailable|Buffers|Cached|SwapTotal|SwapFree'
   ```

## Remediation

### Immediate
- If a specific process is leaking memory, restart it:
  ```bash
  ssh ubuntu@<node-ip> sudo systemctl restart php8.5-fpm
  ssh ubuntu@<node-ip> sudo systemctl restart node-agent
  ```
- If memory is mostly cache/buffers, it is usually safe -- the kernel will reclaim as needed. Verify with:
  ```bash
  ssh ubuntu@<node-ip> free -h  # check "available" column
  ```
- Drop caches if necessary (safe, non-destructive):
  ```bash
  ssh ubuntu@<node-ip> sudo sh -c 'echo 3 > /proc/sys/vm/drop_caches'
  ```

### Long-term
- Set `pm.max_children` appropriately in PHP-FPM pool configs based on available memory
- Use systemd `MemoryMax=` directives for services to prevent runaway memory usage
- Right-size VM memory allocations in Terraform based on shard role and expected load
- Implement per-tenant PHP-FPM pools with memory limits via systemd socket activation
- Consider adding swap as a safety net (small amount, not as primary memory)

## Escalation
If the node is at 97%+ memory and OOM kills are happening, escalate immediately. If the node-agent process is being killed, Temporal task queues for that node will stall -- coordinate with the platform team.
