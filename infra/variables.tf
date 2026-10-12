variable "problem" {
  type    = string
  default = "isucon14"
}

variable "region" {
  type    = string
  default = "ap-northeast-1" # 配布 AMI はすべて東京
}

variable "aws_profile" {
  type    = string
  default = null
}

variable "ssh_public_key_path" {
  description = "EC2 に入れる公開鍵"
  type        = string
}

variable "ssh_private_key_path" {
  description = "生成する ssh_config に書く秘密鍵"
  type        = string
}

variable "app_count" {
  description = "競技サーバーの台数 (null なら問題ごとの本番台数)"
  type        = number
  default     = null
}

variable "app_instance_type" {
  type    = string
  default = null
}

variable "bench_instance_type" {
  type    = string
  default = null
}

variable "auto_shutdown_minutes" {
  description = "消し忘れ対策: 起動からこの分数で OS ごと停止する (0 で無効)。停止後も EBS 代はかかるので tf-down は必須"
  type        = number
  default     = 600
}

variable "allowed_cidrs" {
  description = "SSH/HTTP(S) を許可する CIDR。空なら実行元のグローバルIP/32"
  type        = list(string)
  default     = []
}
