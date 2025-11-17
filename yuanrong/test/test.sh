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

set -e

CUR_DIR=$(dirname "$(readlink -f "$0")")
PROJECT_DIR=$(cd "${CUR_DIR}/.."; pwd)
ROOT_PATH=$PROJECT_DIR

# go module prepare
export GO111MODULE=on
export GONOSUMDB=*
export CGO_ENABLED=1

# resolve missing go.sum entry
go env -w "GOFLAGS"="-mod=mod"

# coverage mode
# set: 每个语句是否执行？
# count: 每个语句执行了几次？
# atomic: 类似于count, 但表示的是并行程序中的精确计数
export GOCOVER_MODE="set"

# test module name
MODULE_LIST=(\
"dashboard"
)

. "${PROJECT_DIR}"/build/compile_functions.sh

# $1: source file name, In the format of xxx.go
# $2: target file name, In the format of xxx_mock.go
function generate_mock()
{
    if ! mockgen -destination "$2" -source "$1" -package mock; then
        log_error "Failed to generate mock file."
        return 1;
    fi
}
export -f generate_mock

# create source code link, go cover report dependent on GOPATH src
link_source_code()
{
    rm -rf "${GOPATH}/pkg"
    rm -rf "${GOPATH}/src/dashboard"

    mkdir -p "${GOPATH}"/src/
    ln -s "${ROOT_PATH}" "${GOPATH}"/src/dashboard
}

link_source_code

if [[ -z "${1}" ]]; then
    for i in "${!MODULE_LIST[@]}";
    do
        if ! sh -x "${CUR_DIR}/${MODULE_LIST[$i]}/test.sh"; then
            echo "Failed to test ${MODULE_LIST[$i]}"
            exit 1
        fi
        echo "Succeed to test ${MODULE_LIST[$i]}"
    done
    echo "Succeed to test all module"
elif [[ "${MODULE_LIST[@]}" =~ "${1}" ]]; then
    if ! sh -x "${CUR_DIR}/${1}/test.sh"; then
        echo "Failed to test ${1}"
        exit 1
    fi
    echo "Succeed to test ${1}"
else
    echo "Please input parameters 'module name: ${MODULE_LIST[@]}'"
    exit 1
fi

exit 0