#!/usr/bin/env bash

WDIR=$(dirname $(readlink -f $0))


function test_run() {
    local target=$1
    local run_mode=$2

    # Create the .env file
    # Build and run all
    # Create the .env file
    cat <<EOF > $WDIR/.env
BUILD_TARGET=${target} # all, jre, native
RUN_MODE=${run_mode} # normal, json-rpc, native, json-rpc-native
EOF

        docker compose \
            --project-directory $WDIR \
            up \
            --build --abort-on-container-exit --exit-code-from signal-cli-rest-api &
        compose_pid=$!
}

function test_wait() {
    local target=$1
    local run_mode=$2
    local timeout=$3
    local start_time=$(date +%s)

    for i in $(seq 1 30); do
        if curl -sf http://localhost:8080/v1/health >/dev/null 2>&1; then
            echo "Health check passed"
            echo Killing $compose_pid
            kill -9 $compose_pid 2>/dev/null
            wait $compose_pid 2>/dev/null
            docker compose down
            if [ $? -ne 0 ]; then
                echo "ERROR: docker compose down failed for ${target} ${run_mode}"
                exit 1
            fi
            return 0
        fi
        sleep 2
    done
    echo "ERROR: docker compose build failed for ${target} ${run_mode}"
}

function test_run_and_wait() {
    local target=$1
    local run_mode=$2
    test_run $target $run_mode
    test_wait $target $run_mode
}

# Try to run all three build targets and wait for them to finish
# normal, json-rpc, native, json-rpc-native
test_run_and_wait "all" "normal"
test_run_and_wait "all" "json-rpc"
test_run_and_wait "all" "native"
test_run_and_wait "all" "json-rpc-native"

test_run_and_wait "jre" "normal"
test_run_and_wait "jre" "json-rpc"

test_run_and_wait "native" "native"
test_run_and_wait "native" "json-rpc-native"
