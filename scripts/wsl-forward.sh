#!/bin/bash
# Enable IP forwarding from WSL2 to libvirt VMs.
# This allows Windows to reach the 10.10.10.0/24 VM network via WSL2.
#
# Required once per WSL2 boot (iptables rules are not persistent).
#
# Usage:
#   sudo ./scripts/wsl-forward.sh          # Enable forwarding
#   sudo ./scripts/wsl-forward.sh stop      # Remove forwarding rules
#   ./scripts/wsl-forward.sh status         # Check current state

set -e

NETWORK="${VM_NETWORK:-10.10.10.0/24}"
BRIDGE="${VM_BRIDGE:-virbr1}"

status() {
    echo "IP forwarding: $(cat /proc/sys/net/ipv4/ip_forward)"
    echo ""
    echo "FORWARD rules for $BRIDGE:"
    iptables -L FORWARD -n --line-numbers 2>/dev/null | grep -E "$BRIDGE|Chain|num" || echo "  (none)"
    echo ""
    echo "NAT POSTROUTING for $NETWORK:"
    iptables -t nat -L POSTROUTING -n --line-numbers 2>/dev/null | grep -E "$NETWORK|Chain|num" || echo "  (none)"
}

start() {
    # Enable IP forwarding.
    sysctl -w net.ipv4.ip_forward=1 > /dev/null

    # Allow forwarding from eth0 (Windows side) to virbr1 (VM side).
    # Insert at top to take precedence over libvirt's REJECT rules.
    iptables -I FORWARD 1 -i eth0 -o "$BRIDGE" -d "$NETWORK" -j ACCEPT
    iptables -I FORWARD 2 -i "$BRIDGE" -o eth0 -s "$NETWORK" -j ACCEPT

    # Masquerade so VMs see traffic from WSL2's bridge IP (10.10.10.1),
    # not from Windows' NAT IP (172.x.x.x) which VMs can't route back to.
    iptables -t nat -C POSTROUTING -s 172.16.0.0/12 -d "$NETWORK" -j MASQUERADE 2>/dev/null || \
        iptables -t nat -A POSTROUTING -s 172.16.0.0/12 -d "$NETWORK" -j MASQUERADE

    WSL_IP=$(ip -4 addr show eth0 | grep -oP '(?<=inet\s)\d+\.\d+\.\d+\.\d+')

    echo "Forwarding enabled: Windows -> WSL2 ($WSL_IP) -> VMs ($NETWORK)"
    echo ""
    echo "Run this on Windows (PowerShell as Administrator):"
    echo "  route add 10.10.10.0 mask 255.255.255.0 $WSL_IP"
    echo ""
    echo "Add to C:\\Windows\\System32\\drivers\\etc\\hosts:"
    echo "  10.10.10.2  admin.hosting.test api.hosting.test temporal.hosting.test dbadmin.hosting.test"
}

stop() {
    iptables -D FORWARD -i eth0 -o "$BRIDGE" -d "$NETWORK" -j ACCEPT 2>/dev/null || true
    iptables -D FORWARD -i "$BRIDGE" -o eth0 -s "$NETWORK" -j ACCEPT 2>/dev/null || true
    iptables -t nat -D POSTROUTING -s 172.16.0.0/12 -d "$NETWORK" -j MASQUERADE 2>/dev/null || true
    echo "Forwarding rules removed."
}

case "${1:-start}" in
    start)  start ;;
    stop)   stop ;;
    status) status ;;
    *)      echo "Usage: $0 [start|stop|status]"; exit 1 ;;
esac
