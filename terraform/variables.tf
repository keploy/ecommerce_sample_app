variable "aws_region" {
  description = "AWS deployment region"
  type        = string
  default     = "ap-southeast-2"
}

variable "vpc_id" {
  description = "Existing VPC ID"
  type        = string
  default     = "vpc-028b727228977b177"
}

variable "subnet_id" {
  description = "Existing public subnet ID"
  type        = string
  default     = "subnet-05edf9f026baf6256"
}

variable "instance_type" {
  description = "Ubuntu EC2 instance type"
  type        = string
  default     = "t3.small"
}

variable "root_volume_size" {
  description = "Ubuntu root disk size in GB"
  type        = number
  default     = 16
}

variable "instance_name" {
  description = "Ubuntu application server name"
  type        = string
  default     = "ecommerce-ubuntu-server"
}

variable "sqs_queue_arn" {
  description = "Application SQS queue ARN"
  type        = string
  default     = "arn:aws:sqs:ap-southeast-2:305846488272:order-events"
}
