output "ubuntu_instance_id" {
  description = "Ubuntu EC2 instance ID"
  value       = aws_instance.ubuntu.id
}

output "ubuntu_public_ip" {
  description = "Ubuntu EC2 public IP"
  value       = aws_instance.ubuntu.public_ip
}

output "ubuntu_private_ip" {
  description = "Ubuntu EC2 private IP"
  value       = aws_instance.ubuntu.private_ip
}

output "ubuntu_security_group_id" {
  description = "Ubuntu security group ID"
  value       = aws_security_group.ubuntu.id
}

output "ubuntu_iam_role_name" {
  description = "Ubuntu IAM role name"
  value       = aws_iam_role.ubuntu.name
}
