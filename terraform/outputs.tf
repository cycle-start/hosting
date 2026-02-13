output "web_node_ips" {
  description = "IP addresses of web nodes"
  value       = { for k, v in var.web_nodes : v.name => v.ip }
}

output "db_node_ips" {
  description = "IP addresses of database nodes"
  value       = { for k, v in var.db_nodes : v.name => v.ip }
}

output "dns_node_ips" {
  description = "IP addresses of DNS nodes"
  value       = { for k, v in var.dns_nodes : v.name => v.ip }
}

output "valkey_node_ips" {
  description = "IP addresses of Valkey nodes"
  value       = { for k, v in var.valkey_nodes : v.name => v.ip }
}

output "web_node_ids" {
  description = "Generated UUIDs for web nodes"
  value       = { for k, v in random_uuid.web_node_id : var.web_nodes[k].name => v.result }
}

output "db_node_ids" {
  description = "Generated UUIDs for DB nodes"
  value       = { for k, v in random_uuid.db_node_id : var.db_nodes[k].name => v.result }
}

output "dns_node_ids" {
  description = "Generated UUIDs for DNS nodes"
  value       = { for k, v in random_uuid.dns_node_id : var.dns_nodes[k].name => v.result }
}

output "valkey_node_ids" {
  description = "Generated UUIDs for Valkey nodes"
  value       = { for k, v in random_uuid.valkey_node_id : var.valkey_nodes[k].name => v.result }
}

output "s3_node_ips" {
  description = "IP addresses of S3/Ceph nodes"
  value       = { for k, v in var.s3_nodes : v.name => v.ip }
}

output "s3_node_ids" {
  description = "Generated UUIDs for S3 nodes"
  value       = { for k, v in random_uuid.s3_node_id : var.s3_nodes[k].name => v.result }
}

output "cluster_yaml" {
  description = "Generated cluster YAML for hostctl cluster apply"
  value       = local.cluster_yaml
}
