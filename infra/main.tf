locals {
  # 問題ごとの AMI と本番に近い構成
  #   matsuu/aws-isucon: https://github.com/matsuu/aws-isucon
  #   private-isu:       https://github.com/catatsuy/private-isu
  problems = {
    "isucon14" = { ami = "ami-0fcf9e8e8675a9ee4", app_count = 3, app_type = "c5.large", bench_type = "c5.xlarge" }
  }
  p = local.problems[var.problem]

  app_count  = coalesce(var.app_count, local.p.app_count)
  app_type   = coalesce(var.app_instance_type, local.p.app_type)
  bench_type = coalesce(var.bench_instance_type, local.p.bench_type)

  my_cidrs = length(var.allowed_cidrs) > 0 ? var.allowed_cidrs : ["${chomp(data.http.myip.response_body)}/32"]

  # 本番と同じく固定プライベートIP: isu1=192.168.0.11, isu2=.12, ... bench=.100
  app_ips  = [for i in range(local.app_count) : cidrhost(aws_subnet.this.cidr_block, 11 + i)]
  bench_ip = cidrhost(aws_subnet.this.cidr_block, 100)
}

data "http" "myip" {
  url = "https://checkip.amazonaws.com/"
}

data "aws_availability_zones" "this" {
  state = "available"
}

resource "aws_vpc" "this" {
  cidr_block           = "192.168.0.0/16"
  enable_dns_hostnames = true
  tags                 = { Name = "baseline-${var.problem}" }
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id
}

resource "aws_subnet" "this" {
  vpc_id                  = aws_vpc.this.id
  cidr_block              = "192.168.0.0/24"
  availability_zone       = data.aws_availability_zones.this.names[0]
  map_public_ip_on_launch = true
}

resource "aws_route_table" "this" {
  vpc_id = aws_vpc.this.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }
}

resource "aws_route_table_association" "this" {
  subnet_id      = aws_subnet.this.id
  route_table_id = aws_route_table.this.id
}

resource "aws_security_group" "this" {
  name   = "baseline-${var.problem}"
  vpc_id = aws_vpc.this.id

  ingress {
    description = "all traffic inside VPC (between servers and bench)" # SG の description は ASCII のみ
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = [aws_vpc.this.cidr_block]
  }

  dynamic "ingress" {
    for_each = toset([22, 80, 443]) # pprof は ssh の LocalForward で見る
    content {
      description = "from my IP"
      from_port   = ingress.value
      to_port     = ingress.value
      protocol    = "tcp"
      cidr_blocks = local.my_cidrs
    }
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_key_pair" "this" {
  key_name   = "baseline-${var.problem}"
  public_key = file(pathexpand(var.ssh_public_key_path))
}

locals {
  user_data = <<-EOT
    #!/bin/bash
    # isucon ユーザーでも同じ鍵で入れるようにする
    if id isucon >/dev/null 2>&1; then
      install -d -m 700 -o isucon -g isucon /home/isucon/.ssh
      cat /home/ubuntu/.ssh/authorized_keys >> /home/isucon/.ssh/authorized_keys
      chown isucon:isucon /home/isucon/.ssh/authorized_keys
      chmod 600 /home/isucon/.ssh/authorized_keys
    fi
    %{if var.auto_shutdown_minutes > 0~}
    shutdown -h +${var.auto_shutdown_minutes} "isucon-suburi auto shutdown"
    %{endif~}
  EOT
}

resource "aws_instance" "app" {
  count                                = local.app_count
  ami                                  = local.p.ami
  instance_type                        = local.app_type
  subnet_id                            = aws_subnet.this.id
  private_ip                           = local.app_ips[count.index]
  vpc_security_group_ids               = [aws_security_group.this.id]
  key_name                             = aws_key_pair.this.key_name
  user_data                            = local.user_data
  instance_initiated_shutdown_behavior = "stop"
  tags                                 = { Name = "isu${count.index + 1}" }
}

resource "aws_instance" "bench" {
  ami                                  = local.p.ami # 配布 AMI はベンチ同梱
  instance_type                        = local.bench_type
  subnet_id                            = aws_subnet.this.id
  private_ip                           = local.bench_ip
  vpc_security_group_ids               = [aws_security_group.this.id]
  key_name                             = aws_key_pair.this.key_name
  user_data                            = local.user_data
  instance_initiated_shutdown_behavior = "stop"
  tags                                 = { Name = "bench" }
}

resource "local_file" "ssh_config" {
  filename        = "${path.module}/../ssh_config"
  file_permission = "0600"
  content = join("\n", concat(
    [for i, inst in aws_instance.app : <<-EOT
      Host isu${i + 1}
        HostName ${inst.public_ip}
        User isucon
        IdentityFile ${pathexpand(var.ssh_private_key_path)}
        StrictHostKeyChecking no
        UserKnownHostsFile /dev/null
        LocalForward ${9000 + i} localhost:6060
      EOT
    ],
    [<<-EOT
      Host bench
        HostName ${aws_instance.bench.public_ip}
        User isucon
        IdentityFile ${pathexpand(var.ssh_private_key_path)}
        StrictHostKeyChecking no
        UserKnownHostsFile /dev/null
      EOT
    ]
  ))
}
