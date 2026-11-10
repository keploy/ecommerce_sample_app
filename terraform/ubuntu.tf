# Latest Ubuntu 24.04 LTS AMI
data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"]

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }

  filter {
    name   = "state"
    values = ["available"]
  }
}

# IAM trust policy for EC2
data "aws_iam_policy_document" "ubuntu_assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "ubuntu" {
  name               = "EcommerceUbuntuEC2Role"
  assume_role_policy = data.aws_iam_policy_document.ubuntu_assume_role.json
}

# Systems Manager access
resource "aws_iam_role_policy_attachment" "ubuntu_ssm" {
  role       = aws_iam_role.ubuntu.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

# ECR image pull access
resource "aws_iam_role_policy" "ubuntu_ecr" {
  name = "EcommerceUbuntuECRPull"
  role = aws_iam_role.ubuntu.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["ecr:GetAuthorizationToken"]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage"
        ]
        Resource = [
          "arn:aws:ecr:${var.aws_region}:305846488272:repository/user-service",
          "arn:aws:ecr:${var.aws_region}:305846488272:repository/product-service",
          "arn:aws:ecr:${var.aws_region}:305846488272:repository/order-service",
          "arn:aws:ecr:${var.aws_region}:305846488272:repository/apigateway"
        ]
      }
    ]
  })
}

# Application SQS send access
resource "aws_iam_role_policy" "ubuntu_sqs" {
  name = "EcommerceUbuntuSQSSend"
  role = aws_iam_role.ubuntu.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["sqs:SendMessage"]
        Resource = var.sqs_queue_arn
      }
    ]
  })
}

resource "aws_iam_instance_profile" "ubuntu" {
  name = "EcommerceUbuntuInstanceProfile"
  role = aws_iam_role.ubuntu.name
}

# Dedicated security group for Ubuntu application server
resource "aws_security_group" "ubuntu" {
  name        = "ecommerce-ubuntu-sg"
  description = "Dedicated security group for Ubuntu ecommerce server"
  vpc_id      = var.vpc_id

  ingress {
    description = "HTTP application gateway"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "ecommerce-ubuntu-sg"
  }
}

# Ubuntu 24.04 application EC2
resource "aws_instance" "ubuntu" {
  ami                         = data.aws_ami.ubuntu.id
  instance_type               = var.instance_type
  subnet_id                   = var.subnet_id
  vpc_security_group_ids      = [aws_security_group.ubuntu.id]
  iam_instance_profile        = aws_iam_instance_profile.ubuntu.name
  associate_public_ip_address = true

  root_block_device {
    volume_size           = var.root_volume_size
    volume_type           = "gp3"
    encrypted             = true
    delete_on_termination = true
  }

  metadata_options {
    http_endpoint = "enabled"
    http_tokens   = "required"
  }

  user_data = <<-USERDATA
    #!/bin/bash
    set -eux
    apt-get update
    DEBIAN_FRONTEND=noninteractive apt-get install -y docker.io docker-compose-v2 awscli snapd
    systemctl enable --now docker
    if ! snap list amazon-ssm-agent >/dev/null 2>&1; then
      snap install amazon-ssm-agent --classic
    fi
    systemctl enable --now snap.amazon-ssm-agent.amazon-ssm-agent.service
    usermod -aG docker ubuntu
  USERDATA

  tags = {
    Name        = var.instance_name
    Environment = "assignment"
    ManagedBy   = "Terraform"
    OS          = "Ubuntu 24.04"
  }

  depends_on = [
    aws_iam_role_policy_attachment.ubuntu_ssm,
    aws_iam_role_policy.ubuntu_ecr,
    aws_iam_role_policy.ubuntu_sqs
  ]

  lifecycle {
    prevent_destroy = true
  }
}
