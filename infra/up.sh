#!/usr/bin/env bash
# 人間（bypass ではないセッション or ターミナル）が実行する。EC2 を作り、ssh_config と BENCH.md を書く。
set -euo pipefail
cd "$(dirname "$0")"
terraform init -upgrade >/dev/null
terraform apply -auto-approve -no-color | tail -3
CFG=../ssh_config
for i in $(seq 1 40); do
  ok=0
  for h in isu1 isu2 isu3 bench; do ssh -F $CFG -o ConnectTimeout=5 -o LogLevel=ERROR $h true 2>/dev/null && ok=$((ok+1)); done
  [ $ok = 4 ] && break
  sleep 10
done
# 環境のノイズを減らす（ベンチ中に OS の自動アップデートが CPU を食うことがある）。前回の素振りと条件を揃えるため
for h in isu1 isu2 isu3 bench; do
  ssh -F $CFG -o LogLevel=ERROR $h 'sudo systemctl disable --now unattended-upgrades.service apt-daily.timer apt-daily-upgrade.timer >/dev/null 2>&1; true'
done
APP1=$(terraform output -json app | python3 -c 'import json,sys; print(json.load(sys.stdin)["isu1"]["private"])')
BENCH=$(terraform output -json bench | python3 -c 'import json,sys; print(json.load(sys.stdin)["private"])')
cat > ../BENCH.md <<MD
# ベンチの回し方（自動生成）

bench サーバーで次を実行する（ベンチサーバーではこれ以外のことをしない）。

\`\`\`sh
ssh -F ssh_config bench './bench run --addr ${APP1}:443 --target https://isuride.xiv.isucon.net --payment-url http://${BENCH}:12346 --payment-bind-port 12346'
\`\`\`

- 負荷の向き先は isu1（${APP1}）。向き先を変えたいときは --addr を別サーバーの private IP にする
- サーバーの private IP: $(terraform output -json app | python3 -c 'import json,sys; d=json.load(sys.stdin); print(", ".join(f"{k}={v[\"private\"]}" for k,v in sorted(d.items())))')
MD
echo "ready: ssh -F ssh_config isu1 / BENCH.md"
