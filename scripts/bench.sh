#!/bin/bash
# usage: scripts/bench.sh "memo"
cd "$(dirname "$0")/.."
mkdir -p notes/bench
f=notes/bench/$(date +%H%M%S).txt
ssh -F ssh_config bench './bench run --addr 192.168.0.11:443 --target https://isuride.xiv.isucon.net --payment-url http://192.168.0.100:12346 --payment-bind-port 12346' > $f 2>&1
grep -v DEBUG $f | tail -25
r=$(grep '結果 pass' $f | sed -E 's/.*pass=([a-z]+) スコア=([0-9-]+).*/\2 | \1/')
echo "| $(date +%H:%M) | $r | $1 |" >> notes/scores.md
tail -1 notes/scores.md
