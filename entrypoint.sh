#!/bin/sh

set -x
set -e

# Fix permissions to ensure backward compatibility
chown 1000:1000 -R /home/.local/share/signal-cli

# Show warning on docker exec
cat <<EOF >> /root/.bashrc
echo "WARNING: signal-cli-rest-api runs as signal-api (not as root!)" 
echo "Run 'su signal-api' before using signal-cli!"
EOF

cap_prefix="-cap_"
caps="$cap_prefix$(seq -s ",$cap_prefix" 0 $(cat /proc/sys/kernel/cap_last_cap))"

# Start API as signal-api user
exec setpriv --reuid=1000 --regid=1000 --init-groups --inh-caps=$caps signal-cli-rest-api $@
