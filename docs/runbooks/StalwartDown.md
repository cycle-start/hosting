# StalwartDown

## What It Means
The Stalwart email server is unreachable for 2+ minutes. This means all email services (SMTP, IMAP, JMAP) are down for tenants. Incoming emails will be deferred by sending servers (typically retried for up to 5 days), and tenants cannot send or read email.

## Severity
Critical -- All tenant email services are unavailable. Incoming mail will queue at sending MTAs, but prolonged outages risk bounced emails and missed communications.

## Likely Causes
1. Stalwart process crashed or was OOM-killed
2. Disk full on the email node (Stalwart needs disk for mail storage and databases)
3. Configuration error after a convergence update
4. Port conflict (SMTP 25/587, IMAP 993, JMAP 443)
5. The email node VM itself is down (see [NodeDown](NodeDown.md))

## Investigation Steps
1. Check Stalwart service status:
   ```bash
   ssh ubuntu@<email-ip> systemctl status stalwart
   ```
2. Check Stalwart logs:
   ```bash
   ssh ubuntu@<email-ip> journalctl -u stalwart --since '30 minutes ago' --no-pager | tail -50
   ```
3. Check for OOM kills:
   ```bash
   ssh ubuntu@<email-ip> dmesg | grep -i "out of memory"
   ```
4. Check disk space:
   ```bash
   ssh ubuntu@<email-ip> df -h
   ```
5. Check port availability:
   ```bash
   ssh ubuntu@<email-ip> ss -tlnp | grep -E ':25|:587|:993|:443'
   ```
6. Check node-agent for recent convergence activity:
   ```bash
   ssh ubuntu@<email-ip> journalctl -u node-agent --since '30 minutes ago' | grep -i stalwart
   ```

## Remediation

### Immediate
- Restart Stalwart:
  ```bash
  ssh ubuntu@<email-ip> sudo systemctl restart stalwart
  ```
- If disk is full, free space (see [HighDiskUsage](HighDiskUsage.md)) then restart:
  ```bash
  ssh ubuntu@<email-ip> sudo journalctl --vacuum-time=2d
  ssh ubuntu@<email-ip> sudo systemctl restart stalwart
  ```
- If configuration is broken, check the config:
  ```bash
  ssh ubuntu@<email-ip> cat /etc/stalwart/config.toml
  ```
- Verify email services are working:
  ```bash
  # Test SMTP
  ssh ubuntu@<email-ip> nc -zv localhost 25
  # Test IMAP
  ssh ubuntu@<email-ip> nc -zv localhost 993
  ```

### Long-term
- Ensure Stalwart has `Restart=always` in its systemd unit
- Implement disk space monitoring with email-specific thresholds
- Set up mail queue monitoring to detect delivery issues early
- Add Stalwart health check endpoint to monitoring
- Implement regular configuration validation via node-agent convergence
- Plan for Stalwart HA (multiple email nodes) for production

## Escalation
If Stalwart cannot be restarted or the issue is a persistent configuration error, escalate to the platform team. If the outage has lasted more than 30 minutes, notify affected tenants about potential email delays. Incoming mail will be retried by sending servers for up to 5 days, but time-sensitive communications may be affected.
