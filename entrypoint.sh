#!/bin/sh

set -x
set -e

[ -z "${SIGNAL_CLI_CONFIG_DIR}" ] && echo "SIGNAL_CLI_CONFIG_DIR environmental variable needs to be set! Aborting!" && exit 1;
[ -z "${SIGNAL_CLI_UID}" ] && echo "SIGNAL_CLI_UID environmental variable needs to be set! Aborting!" && exit 1;
[ -z "${SIGNAL_CLI_GID}" ] && echo "SIGNAL_CLI_GID environmental variable needs to be set! Aborting!" && exit 1;

# Check if we are already running as the target user and group
RUNNING_AS_TARGET_USER=false
if [ "$(id -u)" -eq "${SIGNAL_CLI_UID}" ] && [ "$(id -g)" -eq "${SIGNAL_CLI_GID}" ]; then
  RUNNING_AS_TARGET_USER=true
fi

if [ "$RUNNING_AS_TARGET_USER" = "true" ]; then
  echo "Already running as UID ${SIGNAL_CLI_UID} and GID ${SIGNAL_CLI_GID}. Skipping privileged operations."
else
  echo "Adjusting user and group IDs to ${SIGNAL_CLI_UID}:${SIGNAL_CLI_GID}"
  usermod -u "${SIGNAL_CLI_UID}" signal-api
  groupmod -o -g "${SIGNAL_CLI_GID}" signal-api

  # Fix permissions to ensure backward compatibility if SIGNAL_CLI_CHOWN_ON_STARTUP is not set to "false"
  if [ "$SIGNAL_CLI_CHOWN_ON_STARTUP" != "false" ]; then
    echo "Changing ownership of ${SIGNAL_CLI_CONFIG_DIR} to ${SIGNAL_CLI_UID}:${SIGNAL_CLI_GID}"
    chown "${SIGNAL_CLI_UID}":"${SIGNAL_CLI_GID}" -R "${SIGNAL_CLI_CONFIG_DIR}"
  else
    echo "Skipping chown on startup since SIGNAL_CLI_CHOWN_ON_STARTUP is set to 'false'"
  fi

  # Show warning on docker exec
  cat <<EOF >> /root/.bashrc
echo "WARNING: signal-cli-rest-api runs as signal-api (not as root!)" 
echo "Run 'su signal-api' before using signal-cli!"
echo "If you want to use signal-cli directly, don't forget to specify the config directory. e.g: \"signal-cli --config ${SIGNAL_CLI_CONFIG_DIR}\""
EOF
fi

# TODO: check mode
if [ "$MODE" = "json-rpc" ]; then
  /usr/bin/jsonrpc2-helper
  if [ -n "$JAVA_OPTS" ]; then
      echo "export JAVA_OPTS='$JAVA_OPTS'" >> /etc/default/supervisor
  fi
  service supervisor start
  supervisorctl start all
fi

export HOST_IP=$(hostname -I | awk '{print $1}')

# Start API as signal-api user
if [ "$RUNNING_AS_TARGET_USER" = "true" ]; then
  # Already running as the target user, start directly
  exec signal-cli-rest-api -signal-cli-config="${SIGNAL_CLI_CONFIG_DIR}"
else
  # Use setpriv to switch to target user
  cap_prefix="-cap_"
  caps="$cap_prefix$(seq -s ",$cap_prefix" 0 $(cat /proc/sys/kernel/cap_last_cap))"
  exec setpriv --reuid="${SIGNAL_CLI_UID}" --regid="${SIGNAL_CLI_GID}" --init-groups --inh-caps="$caps" signal-cli-rest-api -signal-cli-config="${SIGNAL_CLI_CONFIG_DIR}"
fi
