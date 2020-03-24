#!/bin/bash

set -x
set -euo pipefail

# SIGNAL_CLI_DIR="${DATA_DIR}/signal-cli"

ENCODING_DEFAULT="UTF-8"
LANGUAGE_DEFAULT="en_US"

create_signal_cli_dir() {
  mkdir -p ${SIGNAL_CLI_DIR}
  
  chmod -R 0775 ${SIGNAL_CLI_DIR}
}

install_packages() {
  if [ $# -ge 1 ] && [ -n "${1}" ]; then
    apt-get update
    apt-get install --no-install-recommends -y ${@}
    rm -rf "/var/lib/apt/lists/*"
  else
    exit 1
  fi
}

set_language() {
  # Set encoding inside container (to avoid accent problems in messages)
  # Only for Debian-based distribution

  set +u
  # If not set or empty, set default variables
  [[ -z "${CONTAINER_ENCODING}" ]] && CONTAINER_ENCODING=${ENCODING_DEFAULT}
  [[ -z "${CONTAINER_LANGUAGE}" ]] && CONTAINER_LANGUAGE=${LANGUAGE_DEFAULT}

  # Test if locale.gen exists
  [[ ! -f /etc/locale.gen ]] && touch /etc/locale.gen
  set -u

  sed -i -e "s/# ${CONTAINER_LANGUAGE}.${CONTAINER_ENCODING} ${CONTAINER_ENCODING}/${CONTAINER_LANGUAGE}.${CONTAINER_ENCODING} ${CONTAINER_ENCODING}/" /etc/locale.gen

  dpkg-reconfigure --frontend=noninteractive locales
  update-locale LANG="${CONTAINER_LANGUAGE}.${CONTAINER_ENCODING}"
}

create_symlinks() {
  [[ ! -d ${SIGNAL_CLI_DIR} ]] && mkdir -p "${SIGNAL_CLI_DIR}"

  ln -s ${SIGNAL_CLI_DIR} /
}


#main

# install_packages locales #set in dockerfile instead of inside entrypoint
set_language



# extra args for main (go API)
if [[ ${1:0:1} = '-' ]]; then
  EXTRA_ARGS="$@"
  set --
elif [[ ${1} == main || ${1} == $(which main) ]]; then
  EXTRA_ARGS="${@:2}"
  set --
fi

# default behaviour is to launch main (go API)
if [[ ${#} -lt 1 ]]; then
  echo "Starting signal-cli-rest-api (main)..."
  exec $(which main)
else
  exec "$@"
fi