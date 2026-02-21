#!/usr/bin/env python3
"""
Dynamic Ansible inventory backed by the hosting platform API.

Groups produced:
  - By role: web, db, dns, valkey, email, storage, dbadmin, lb
  - By cluster: cluster_{id}
  - By region: region_{id}

Hostvars per host (keyed by hostname):
  - ansible_host = ip_address
  - node_id, cluster_id, region_id, node_roles, node_status, shard_assignments

Config via environment:
  - HOSTING_API_URL (default: http://10.10.10.2:8090/api/v1)
  - HOSTING_API_KEY (required)

Also merges static.ini as a secondary source for hosts not in the API
(e.g. controlplane).
"""

import json
import os
import subprocess
import sys
import urllib.request
import urllib.error

API_URL = os.environ.get("HOSTING_API_URL", "http://10.10.10.2:8090/api/v1")
API_KEY = os.environ.get("HOSTING_API_KEY", "")

# Map API role names to Ansible group names
ROLE_MAP = {
    "web": "web",
    "database": "db",
    "dns": "dns",
    "valkey": "valkey",
    "email": "email",
    "storage": "storage",
    "s3": "storage",
    "dbadmin": "dbadmin",
    "lb": "lb",
}


def api_get(path):
    """Make a GET request to the hosting API with pagination."""
    items = []
    url = f"{API_URL}{path}"

    while True:
        req = urllib.request.Request(url)
        if API_KEY:
            req.add_header("X-API-Key", API_KEY)

        try:
            with urllib.request.urlopen(req, timeout=10) as resp:
                data = json.loads(resp.read().decode())
        except (urllib.error.URLError, urllib.error.HTTPError, OSError):
            return []

        page_items = data.get("items", [])
        if page_items:
            items.extend(page_items)

        if not data.get("has_more", False):
            break

        cursor = data.get("next_cursor", "")
        if not cursor:
            break

        separator = "&" if "?" in path else "?"
        url = f"{API_URL}{path}{separator}cursor={cursor}"

    return items


def get_static_inventory():
    """Parse static.ini to get hosts not in the API (e.g. controlplane)."""
    static_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "static.ini")
    if not os.path.exists(static_path):
        return {}, {}

    groups = {}
    hostvars = {}
    current_group = None

    with open(static_path) as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#") or line.startswith(";"):
                continue
            if line.startswith("[") and line.endswith("]"):
                current_group = line[1:-1]
                if current_group not in groups:
                    groups[current_group] = []
                continue
            if current_group:
                parts = line.split()
                hostname = parts[0]
                groups[current_group].append(hostname)
                for part in parts[1:]:
                    if "=" in part:
                        key, val = part.split("=", 1)
                        if hostname not in hostvars:
                            hostvars[hostname] = {}
                        hostvars[hostname][key] = val

    return groups, hostvars


def build_inventory():
    """Build the full Ansible inventory from the API + static fallback."""
    inventory = {"_meta": {"hostvars": {}}}

    # Track hostnames we've seen from the API to avoid duplicates with static
    api_hostnames = set()

    # Fetch all nodes from the API
    regions = api_get("/regions")
    for region in regions:
        region_id = region["id"]
        region_group = f"region_{region_id}"

        clusters = api_get(f"/regions/{region_id}/clusters")
        for cluster in clusters:
            cluster_id = cluster["id"]
            cluster_group = f"cluster_{cluster_id}"

            nodes = api_get(f"/clusters/{cluster_id}/nodes")
            for node in nodes:
                hostname = node.get("hostname", "")
                ip = node.get("ip_address", "")
                if not hostname or not ip:
                    continue

                api_hostnames.add(hostname)

                # Add to hostvars
                shard_assignments = []
                for s in node.get("shards", []):
                    shard_assignments.append({
                        "shard_id": s.get("shard_id", ""),
                        "shard_role": s.get("shard_role", ""),
                        "shard_index": s.get("shard_index", 0),
                    })

                inventory["_meta"]["hostvars"][hostname] = {
                    "ansible_host": ip,
                    "node_id": node.get("id", ""),
                    "cluster_id": cluster_id,
                    "region_id": region_id,
                    "node_roles": node.get("roles", []),
                    "node_status": node.get("status", ""),
                    "shard_assignments": shard_assignments,
                }

                # Add to region and cluster groups
                if region_group not in inventory:
                    inventory[region_group] = {"hosts": []}
                inventory[region_group]["hosts"].append(hostname)

                if cluster_group not in inventory:
                    inventory[cluster_group] = {"hosts": []}
                inventory[cluster_group]["hosts"].append(hostname)

                # Add to role groups
                for role in node.get("roles", []):
                    group = ROLE_MAP.get(role, role)
                    if group not in inventory:
                        inventory[group] = {"hosts": []}
                    if hostname not in inventory[group]["hosts"]:
                        inventory[group]["hosts"].append(hostname)

    # Merge static inventory for hosts not in the API
    static_groups, static_hostvars = get_static_inventory()
    for group, hosts in static_groups.items():
        for hostname in hosts:
            if hostname in api_hostnames:
                continue
            if group not in inventory:
                inventory[group] = {"hosts": []}
            if hostname not in inventory[group]["hosts"]:
                inventory[group]["hosts"].append(hostname)
            if hostname in static_hostvars:
                inventory["_meta"]["hostvars"][hostname] = static_hostvars[hostname]

    return inventory


def main():
    if "--list" in sys.argv:
        print(json.dumps(build_inventory(), indent=2))
    elif "--host" in sys.argv:
        # Single host — return empty dict, hostvars are in _meta
        print(json.dumps({}))
    else:
        print(json.dumps(build_inventory(), indent=2))


if __name__ == "__main__":
    main()
