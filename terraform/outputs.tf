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

output "storage_node_ips" {
  description = "IP addresses of storage/Ceph nodes"
  value       = { for k, v in var.storage_nodes : v.name => v.ip }
}

output "dbadmin_node_ips" {
  description = "IP addresses of DB Admin nodes"
  value       = { for k, v in var.dbadmin_nodes : v.name => v.ip }
}

output "lb_node_ips" {
  description = "IP addresses of LB nodes"
  value       = { for k, v in var.lb_nodes : v.name => v.ip }
}

output "email_node_ips" {
  description = "IP addresses of email nodes"
  value       = { for k, v in var.email_nodes : v.name => v.ip }
}

output "gateway_node_ips" {
  description = "IP addresses of gateway nodes"
  value       = { for k, v in var.gateway_nodes : v.name => v.ip }
}

output "single_node_ips" {
  description = "IP addresses of single-node (all-in-one) VMs"
  value       = { for k, v in var.single_nodes : v.name => v.ip }
}

output "controlplane_ip" {
  description = "IP address of the k3s control plane VM"
  value       = var.controlplane_ip
}

output "cluster_yaml" {
  description = "Generated cluster YAML for hostctl cluster apply"
  value       = local.cluster_yaml
}
