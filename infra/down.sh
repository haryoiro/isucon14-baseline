#!/usr/bin/env bash
# 人間が実行する。EC2 を破棄する。
set -euo pipefail
cd "$(dirname "$0")"
terraform destroy -auto-approve -no-color | tail -1
rm -f ../ssh_config ../BENCH.md
