#!/bin/bash
cd "$(dirname "$0")/.."
H=${1:-isu1}
ssh -F ssh_config $H 'sudo mysql -e "set global slow_query_log=0"; sudo pt-query-digest --limit 20 /var/log/mysql/slow.log 2>/dev/null' > notes/bench/digest.txt
grep -A 26 "# Profile" notes/bench/digest.txt
