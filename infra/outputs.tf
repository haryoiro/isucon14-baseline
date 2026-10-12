output "app" {
  value = { for i, inst in aws_instance.app : "isu${i + 1}" => { public = inst.public_ip, private = inst.private_ip } }
}

output "bench" {
  value = { public = aws_instance.bench.public_ip, private = aws_instance.bench.private_ip }
}

output "hourly_cost_hint" {
  value = "${local.app_count} x ${local.app_type} + 1 x ${local.bench_type} が起動中。終わったら mise run aws:down"
}
