#!/bin/bash
# usage: scripts/deploy.sh [hosts...]  (default: isu1)
set -eu
cd "$(dirname "$0")/.."
(cd webapp/go && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o isuride .)
HOSTS=${@:-isu1}
for h in $HOSTS; do
  rsync -az -e "ssh -F ssh_config" webapp/sql/ $h:webapp/sql/
  ssh -F ssh_config $h 'sudo systemctl stop isuride-go' || true
  rsync -az -e "ssh -F ssh_config" webapp/go/isuride $h:webapp/go/isuride
  ssh -F ssh_config $h 'sudo systemctl start isuride-go'
done
