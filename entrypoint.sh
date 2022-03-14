#!/bin/sh

set -x
set -e

[ -z "${SIGNAL_CLI_CONFIG_DIR}" ] && echo "SIGNAL_CLI_CONFIG_DIR environmental variable needs to be set! Aborting!" && exit 1;

usermod -u ${SIGNAL_CLI_UID} signal-api
groupmod -g ${SIGNAL_CLI_GID} signal-api

# Fix permissions to ensure backward compatibility
chown ${SIGNAL_CLI_UID}:${SIGNAL_CLI_GID} -R ${SIGNAL_CLI_CONFIG_DIR}

# Show warning on docker exec
cat <<EOF >> /root/.bashrc
echo "WARNING: signal-cli-rest-api runs as signal-api (not as root!)" 
echo "Run 'su signal-api' before using signal-cli!"
echo "If you want to use signal-cli directly, don't forget to specify the config directory. e.g: \"signal-cli --config ${SIGNAL_CLI_CONFIG_DIR}\""
EOF

cap_prefix="-cap_"
caps="$cap_prefix$(seq -s ",$cap_prefix" 0 $(cat /proc/sys/kernel/cap_last_cap))"

# TODO: check mode
if [ "$MODE" = "json-rpc" ]
then
/usr/bin/jsonrpc2-helper
service supervisor start
supervisorctl start all
fi

export HOST_IP=$(hostname -I | awk '{print $1}')

# Start API as signal-api user
exec setpriv --reuid=${SIGNAL_CLI_UID} --regid=${SIGNAL_CLI_GID} --init-groups --inh-caps=$caps signal-cli-rest-api -signal-cli-config=${SIGNAL_CLI_CONFIG_DIR}
