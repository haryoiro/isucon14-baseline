#!/bin/bash
cd "$(dirname "$0")/.."
H=${1:-isu1}
ssh -F ssh_config $H 'sudo rm -f /var/log/mysql/slow.log; sudo mysql -e "set global slow_query_log_file=\"/var/log/mysql/slow.log\"; set global long_query_time=0; set global slow_query_log=1;"; sudo systemctl restart isuride-go 2>/dev/null || true'
