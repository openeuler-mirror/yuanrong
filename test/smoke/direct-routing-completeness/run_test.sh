#!/usr/bin/env bash
# Copyright (c) Huawei Technologies Co., Ltd. 2026. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Direct Routing completeness smoke runner.
# Starts YuanRong with enable_direct_routing=true and a tiny route cache, then
# exercises named instances, route-cache eviction, multi-node scheduling surface,
# and optional proxy-failure behavior.

set -euo pipefail

unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY
export ENABLE_DATASYSTEM="${ENABLE_DATASYSTEM:-true}"
export DATA_SYSTEM_ENABLE="${DATA_SYSTEM_ENABLE:-true}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
DEPLOY_PATH="${DEPLOY_PATH:-/tmp/yr-direct-routing-completeness-smoke}"
STARTUP_LOG="${DEPLOY_PATH}/master-startup.log"
AGENT_LOG="${DEPLOY_PATH}/agent-startup.log"
MASTER_INFO="${DEPLOY_PATH}/master.info"

FRONTEND_PORT="${FRONTEND_PORT:-19888}"
FRONTEND_GRPC_PORT="${FRONTEND_GRPC_PORT:-19889}"
META_SERVICE_PORT="${META_SERVICE_PORT:-19890}"
FUNCTION_AGENT_PORT="${FUNCTION_AGENT_PORT:-19891}"
FUNCTION_PROXY_PORT="${FUNCTION_PROXY_PORT:-19892}"
FUNCTION_PROXY_GRPC_PORT="${FUNCTION_PROXY_GRPC_PORT:-19893}"
GLOBAL_SCHEDULER_PORT="${GLOBAL_SCHEDULER_PORT:-19894}"
DS_MASTER_PORT="${DS_MASTER_PORT:-19895}"
DS_WORKER_PORT="${DS_WORKER_PORT:-19896}"
ETCD_PORT="${ETCD_PORT:-19897}"
ETCD_PEER_PORT="${ETCD_PEER_PORT:-19898}"

YR_TEST_MULTI_NODE="${YR_TEST_MULTI_NODE:-2}"
YR_REQUIRE_MULTI_NODE="${YR_REQUIRE_MULTI_NODE:-true}"
YR_DIRECT_ROUTE_CACHE_CAPACITY="${YR_DIRECT_ROUTE_CACHE_CAPACITY:-4}"
YR_TEST_PROXY_FAILURE="${YR_TEST_PROXY_FAILURE:-true}"
YR_PROXY_FAILURE_COMMAND="${YR_PROXY_FAILURE_COMMAND:-}"
PYTHON_BIN="${PYTHON_BIN:-}"
YR_BIN="${YR_BIN:-}"
WHEEL_PATH=""
SDK_WHEEL_PATH=""
TEST_FAILED=0
STARTED_RUNTIME=0

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${YELLOW}[INFO]${NC} $1"; }
log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; TEST_FAILED=1; }

cleanup() {
    if [ "${STARTED_RUNTIME}" -ne 1 ]; then
        return 0
    fi
    log_info "Cleaning up YuanRong smoke runtime..."
    if [ -n "${YR_BIN}" ] && [ -x "${YR_BIN}" ]; then
        "${YR_BIN}" stop 2>/dev/null || true
    fi
}
trap cleanup EXIT

resolve_python() {
    if [ -n "${PYTHON_BIN}" ] && [ -x "${PYTHON_BIN}" ]; then
        return 0
    fi
    local candidate
    for candidate in \
        "${CONDA_PREFIX:-}/bin/python" \
        "${VIRTUAL_ENV:-}/bin/python" \
        /opt/buildtools/python3.11/bin/python3 \
        /opt/buildtools/python3.9/bin/python3 \
        "$(command -v python3.11 2>/dev/null || true)" \
        "$(command -v python3.9 2>/dev/null || true)" \
        "$(command -v python3 2>/dev/null || true)"; do
        if [ -n "${candidate}" ] && [ -x "${candidate}" ]; then
            PYTHON_BIN="${candidate}"
            return 0
        fi
    done
    log_fail "No usable Python interpreter found"
    exit 1
}

resolve_wheel() {
    local runtime_matches=()
    local sdk_matches=()
    local wheel
    while IFS= read -r wheel; do
        [ -n "${wheel}" ] && runtime_matches+=("${wheel}")
    done < <(find "${PROJECT_ROOT}/output" -maxdepth 1 -type f -name 'openyuanrong-*.whl' 2>/dev/null | sort)
    while IFS= read -r wheel; do
        [ -n "${wheel}" ] && sdk_matches+=("${wheel}")
    done < <(find "${PROJECT_ROOT}/output" -maxdepth 1 -type f -name 'openyuanrong_sdk-*.whl' 2>/dev/null | sort)
    if [ "${#runtime_matches[@]}" -eq 0 ] || [ "${#sdk_matches[@]}" -eq 0 ]; then
        log_fail "Missing openyuanrong/openyuanrong_sdk wheels in ${PROJECT_ROOT}/output"
        exit 1
    fi
    WHEEL_PATH="${runtime_matches[0]}"
    SDK_WHEEL_PATH="${sdk_matches[0]}"
    log_pass "Wheels: $(basename "${WHEEL_PATH}"), $(basename "${SDK_WHEEL_PATH}")"
}

resolve_yr() {
    local bin_dir
    bin_dir="$(dirname "${PYTHON_BIN}")"
    if [ -x "${bin_dir}/yr" ]; then
        YR_BIN="${bin_dir}/yr"
        return 0
    fi
    if command -v yr >/dev/null 2>&1; then
        YR_BIN="$(command -v yr)"
        return 0
    fi
    log_fail "yr CLI not found after wheel installation"
    exit 1
}

resolve_runtime_host() {
    local host
    host="$(hostname -i 2>/dev/null | awk '{print $1}')" || true
    if [ -z "${host}" ]; then
        host="$("${PYTHON_BIN}" -c 'import socket; print(socket.gethostbyname(socket.gethostname()))' 2>/dev/null)" || true
    fi
    if [ -z "${host}" ] || [[ "${host}" == 127.* ]]; then
        host="127.0.0.1"
    fi
    echo "${host}"
}

install_wheels() {
    log_info "Installing smoke wheels..."
    "${PYTHON_BIN}" -m pip uninstall openyuanrong openyuanrong_sdk -y >/dev/null 2>&1 || true
    "${PYTHON_BIN}" -m pip install --force-reinstall "${SDK_WHEEL_PATH}" "${WHEEL_PATH}" 2>&1 | tail -10
    resolve_yr
    log_pass "Installed wheels"
}

check_ports() {
    local ports=(
        "${FRONTEND_PORT}" "${FRONTEND_GRPC_PORT}" "${META_SERVICE_PORT}"
        "${FUNCTION_AGENT_PORT}" "${FUNCTION_PROXY_PORT}" "${FUNCTION_PROXY_GRPC_PORT}"
        "${GLOBAL_SCHEDULER_PORT}" "${DS_MASTER_PORT}" "${DS_WORKER_PORT}"
        "${ETCD_PORT}" "${ETCD_PEER_PORT}"
    )
    local port
    for port in "${ports[@]}"; do
        if (command -v ss >/dev/null 2>&1 && ss -ltnp 2>/dev/null | grep -q ":${port} ") || \
           (command -v netstat >/dev/null 2>&1 && netstat -ltnp 2>/dev/null | grep -q ":${port} "); then
            log_fail "Port ${port} already in use"
            exit 1
        fi
    done
    log_pass "Required ports are free"
}

prepare_proxy_failure_injector() {
    if [ -n "${YR_PROXY_FAILURE_COMMAND}" ]; then
        return 0
    fi
    local injector="${DEPLOY_PATH}/kill_proxy_for_node.sh"
    cat > "${injector}" <<'EOS'
#!/usr/bin/env bash
set -euo pipefail
node_id="$1"
services_path="$2"
line="$(pgrep -af "[f]unction_proxy" | grep -- "--node_id=${node_id}" | grep -F -- "--services_path=${services_path}" | head -1 || true)"
if [ -z "${line}" ]; then
    echo "function_proxy for node ${node_id} and services ${services_path} not found" >&2
    exit 2
fi
pid="$(printf '%s\n' "${line}" | awk '{print $1}')"
echo "killing function_proxy node=${node_id} pid=${pid}" >&2
kill -TERM "${pid}"
for _ in $(seq 1 20); do
    if ! kill -0 "${pid}" 2>/dev/null; then
        exit 0
    fi
    sleep 0.5
done
echo "function_proxy pid ${pid} still alive after TERM" >&2
exit 3
EOS
    chmod +x "${injector}"
    YR_PROXY_FAILURE_COMMAND="${injector} {node_id} ${SCRIPT_DIR}/services.yaml"
}

start_master() {
    log_info "Starting Direct Routing master node..."
    mkdir -p "${DEPLOY_PATH}"
    export LITEBUS_DATA_KEY=6D792D7365637265742D6B65792D666F722D6A77742D64656D6F
    export YR_ENABLE_DIRECT_ROUTING=true
    export YR_DIRECT_ROUTE_CACHE_CAPACITY="${YR_DIRECT_ROUTE_CACHE_CAPACITY}"

    "${YR_BIN}" start --master \
        -d "${DEPLOY_PATH}/master" \
        -n dr-master \
        -l DEBUG \
        --enable_direct_routing true \
        --direct_route_cache_capacity "${YR_DIRECT_ROUTE_CACHE_CAPACITY}" \
        --runtime_home_dir "${HOME}" \
        --enable_faas_frontend=true \
        --enable_function_scheduler true \
        --enable_meta_service true \
        --enable_iam_server false \
        --enable_function_token_auth false \
        --port_policy FIX \
        --faas_frontend_http_port "${FRONTEND_PORT}" \
        --faas_frontend_grpc_port "${FRONTEND_GRPC_PORT}" \
        --meta_service_port "${META_SERVICE_PORT}" \
        --function_agent_port "${FUNCTION_AGENT_PORT}" \
        --function_proxy_port "${FUNCTION_PROXY_PORT}" \
        --function_proxy_grpc_port "${FUNCTION_PROXY_GRPC_PORT}" \
        --global_scheduler_port "${GLOBAL_SCHEDULER_PORT}" \
        --ds_master_port "${DS_MASTER_PORT}" \
        --ds_worker_port "${DS_WORKER_PORT}" \
        --etcd_port "${ETCD_PORT}" \
        --etcd_peer_port "${ETCD_PEER_PORT}" \
        -p "${SCRIPT_DIR}/services.yaml" \
        > "${STARTUP_LOG}" 2>&1

    find "${DEPLOY_PATH}" -name master.info -type f -print -quit > "${MASTER_INFO}.path" || true
    local master_info_path
    master_info_path="$(cat "${MASTER_INFO}.path" 2>/dev/null || true)"
    if [ -n "${master_info_path}" ] && [ -f "${master_info_path}" ]; then
        cp "${master_info_path}" "${MASTER_INFO}"
    fi
    STARTED_RUNTIME=1
    prepare_proxy_failure_injector
    log_pass "Master started (log: ${STARTUP_LOG})"
}

start_agents_if_requested() {
    if [ "${YR_TEST_MULTI_NODE}" -le 1 ]; then
        log_info "YR_TEST_MULTI_NODE=${YR_TEST_MULTI_NODE}; skipping agent startup"
        return 0
    fi
    if [ ! -s "${MASTER_INFO}" ]; then
        log_fail "Missing master.info; cannot start agent nodes"
        exit 1
    fi
    local count=$((YR_TEST_MULTI_NODE - 1))
    local idx
    for idx in $(seq 1 "${count}"); do
        log_info "Starting Direct Routing agent ${idx}/${count}..."
        "${YR_BIN}" start \
            -d "${DEPLOY_PATH}/agent-${idx}" \
            -n "dr-agent-${idx}" \
            -l DEBUG \
            --master_info "$(cat "${MASTER_INFO}")" \
            --enable_direct_routing true \
            --direct_route_cache_capacity "${YR_DIRECT_ROUTE_CACHE_CAPACITY}" \
            --runtime_home_dir "${HOME}" \
            --port_policy RANDOM \
            -p "${SCRIPT_DIR}/services.yaml" \
            >> "${AGENT_LOG}" 2>&1 || {
                log_fail "Agent ${idx} failed to start; see ${AGENT_LOG}"
                exit 1
            }
    done
    log_pass "Agent nodes started"
}

wait_frontend_ready() {
    local host="$1"
    local http_url="http://${host}:${FRONTEND_PORT}/healthz"
    local https_url="https://${host}:${FRONTEND_PORT}/healthz"
    log_info "Waiting for frontend readiness at ${http_url} or ${https_url} ..."
    local retry=0
    while [ "${retry}" -lt 60 ]; do
        if curl -s -o /dev/null -w "%{http_code}" "${http_url}" 2>/dev/null | grep -q "200"; then
            log_pass "Frontend ready (${http_url})"
            return 0
        fi
        if curl -k -s -o /dev/null -w "%{http_code}" "${https_url}" 2>/dev/null | grep -q "200"; then
            log_pass "Frontend ready (${https_url})"
            return 0
        fi
        retry=$((retry + 1))
        sleep 2
    done
    log_fail "Frontend not ready"
    tail -40 "${STARTUP_LOG}" 2>/dev/null || true
    tail -40 "${AGENT_LOG}" 2>/dev/null || true
    exit 1
}

run_python_tests() {
    local host="$1"
    local server_addr="${host}:${FRONTEND_PORT}"
    log_info "Running Direct Routing completeness assertions against ${server_addr}..."
    local output
    output=$(
        ENABLE_DATASYSTEM="${ENABLE_DATASYSTEM}" \
        DATA_SYSTEM_ENABLE="${DATA_SYSTEM_ENABLE}" \
        YR_SERVER_ADDRESS="${server_addr}" \
        YR_REQUIRE_MULTI_NODE="${YR_REQUIRE_MULTI_NODE}" \
        YR_TEST_PROXY_FAILURE="${YR_TEST_PROXY_FAILURE}" \
        YR_DIRECT_ROUTE_CACHE_CAPACITY="${YR_DIRECT_ROUTE_CACHE_CAPACITY}" \
        YR_PROXY_FAILURE_COMMAND="${YR_PROXY_FAILURE_COMMAND}" \
        DEPLOY_PATH="${DEPLOY_PATH}" \
        timeout 900 "${PYTHON_BIN}" "${SCRIPT_DIR}/test_direct_routing_completeness.py" 2>&1
    ) || {
        log_fail "Direct Routing completeness assertions failed"
        echo "${output}"
        exit 1
    }
    echo "${output}"
    if echo "${output}" | grep -q "Direct Routing Completeness Smoke: ALL PASS"; then
        log_pass "Direct Routing completeness smoke passed"
        return 0
    fi
    log_fail "Direct Routing completeness smoke did not report ALL PASS"
    exit 1
}

main() {
    echo "=== Direct Routing Completeness Smoke Test / Direct Routing 完备性冒烟测试 ==="
    echo "YR_TEST_MULTI_NODE=${YR_TEST_MULTI_NODE}"
    echo "YR_DIRECT_ROUTE_CACHE_CAPACITY=${YR_DIRECT_ROUTE_CACHE_CAPACITY}"
    echo "YR_REQUIRE_MULTI_NODE=${YR_REQUIRE_MULTI_NODE}"
    echo "YR_TEST_PROXY_FAILURE=${YR_TEST_PROXY_FAILURE}"

    resolve_python
    resolve_wheel
    install_wheels
    check_ports
    start_master
    start_agents_if_requested
    local host
    host="$(resolve_runtime_host)"
    wait_frontend_ready "${host}"
    run_python_tests "${host}"

    if [ "${TEST_FAILED}" -eq 0 ]; then
        echo -e "${GREEN}DIRECT ROUTING COMPLETENESS SMOKE: PASS${NC}"
    else
        echo -e "${RED}DIRECT ROUTING COMPLETENESS SMOKE: FAIL${NC}"
    fi
    exit "${TEST_FAILED}"
}

main "$@"
