#!/bin/bash
# Copyright (c) Huawei Technologies Co., Ltd. 2025. All rights reserved.
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

set -o pipefail
# Permission control: remove others' permissions of directories and files
umask -p 027

BASE_DIR=$(dirname "$(readlink -f "$0")")
[[ ! -f "${BASE_DIR}/utils.sh" ]] && echo "${BASE_DIR}/utils.sh does not exist" && exit 1
. ${BASE_DIR}/utils.sh

CURRENT_PID=$$

if [ "X$YR_NODE_ID" != "X" ]; then
  NODE_ID=${YR_NODE_ID:0-20}
fi

function wait_times_from_system_timeout() {
  local default_wait_times=30
  if [[ "${SYSTEM_TIMEOUT}" =~ ^[0-9]+$ ]]; then
    local wait_times=$(((SYSTEM_TIMEOUT + 1999) / 2000))
    if [ ${wait_times} -gt ${default_wait_times} ]; then
      echo ${wait_times}
      return
    fi
  fi
  echo ${default_wait_times}
}

function dump_deploy_log_tail() {
  local deploy_log=$1
  if [ -f "${deploy_log}" ]; then
    log_error "tail deploy log: ${deploy_log}"
    tail -100 "${deploy_log}"
  fi
}

function main() {
  log_info "start yr data plane..."
  source ${BASE_DIR}/config.sh --block=true "$@" --only_check_param

  if [ "$BLOCK" == "false" ]; then
    local deploy_log="${LOG_ROOT}/${NODE_ID}_deploy${STD_LOG_SUFFIX}"
    nohup bash ${BASE_DIR}/deploy.sh "$@" -a ${IP_ADDRESS} \
      > "${deploy_log}" 2>&1 &
    local deploy_pid=$!
    local master_info_string
    local data_plane_wait_times
    data_plane_wait_times=$(wait_times_from_system_timeout)
    for ((t = 1; t < data_plane_wait_times; t++ )); do
      sleep 2
      if [ -f "${MASTER_INFO_OUT_FILE}" ]; then
        master_info_string=$( head -n 1 $MASTER_INFO_OUT_FILE )
        echo "${master_info_string}"
        return 0
      fi
      if ! kill -0 ${deploy_pid} 2>/dev/null; then
        wait ${deploy_pid}
        log_error "deploy process exited before data plane ready"
        dump_deploy_log_tail "${deploy_log}"
        return 98
      fi
    done
    log_error "wait start data plane timeout."
    dump_deploy_log_tail "${deploy_log}"
    return 98
  else
    bash ${BASE_DIR}/deploy.sh "$@" -a ${IP_ADDRESS}
  fi
}

main "$@"

if [ "$BLOCK" == "false" ]; then
  ret_code=$?
  if [ ${ret_code} -ne 0 ]; then
      log_error "yr-agent deploy failed, code: $ret_code"
      exit ${ret_code}
  fi
  exit 0
fi
