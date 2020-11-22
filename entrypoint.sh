#!/bin/sh

set -x
set -e

# Fix permissions to ensure backward compatibility
chown 1000:1000 -R /home/.local/share/signal-cli

# Start API
exec su -s /bin/sh -c "exec signal-cli-rest-api" signal-api
