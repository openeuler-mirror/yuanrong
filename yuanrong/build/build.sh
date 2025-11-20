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

BASE_DIR=$(cd "$(dirname "$0")"; pwd)
PROJECT_DIR=$(cd "$(dirname "$0")"/..; pwd)
OUTPUT_DIR="${PROJECT_DIR}/output"
RUNTIME_OUTPUT_DIR="${PROJECT_DIR}/../output"
POSIX_DIR="${PROJECT_DIR}/proto/posix"
BUILD_TAGS=""
FLAGS='-extldflags "-fPIC -fstack-protector-strong -Wl,-z,now,-z,relro,-z,noexecstack,-s -Wall -Werror"'

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# go module prepare
export GO111MODULE=on
export GONOSUMDB=*
export CGO_ENABLED=1

# resolve missing go.sum entry
go env -w "GOFLAGS"="-mod=mod"

echo "generating fs proto pb objects"
protoc --proto_path=${POSIX_DIR} --go_out=${PROJECT_DIR}/../ --go-grpc_out=${PROJECT_DIR}/../ ${POSIX_DIR}/*.proto

echo "start to compile dashboard -s ${SCC_BUILD_ENABLED}"
mkdir -p "${OUTPUT_DIR}/bin/"
rm -rf "${OUTPUT_DIR}/bin/dashboard"

CC='gcc -fstack-protector-strong -D_FORTIFY_SOURCE=2 -O2' go build -tags="${BUILD_TAGS}" -buildmode=pie -ldflags "${FLAGS}"  -o \
"${OUTPUT_DIR}"/bin/dashboard "${PROJECT_DIR}"/cmd/dashboard/main.go

mkdir -p "${OUTPUT_DIR}/config/"
rm -rf "${OUTPUT_DIR}/config/dashboard*"
cp -ar "${BASE_DIR}/dashboard/config/" "${OUTPUT_DIR}/"

npm config set strict-ssl false
echo "start to compile dashboard client"
mkdir -p "${OUTPUT_DIR}/bin/"
rm -rf "${OUTPUT_DIR}/bin/client"
cd "${PROJECT_DIR}/pkg/dashboard/client"
npm install || die "dashboard client install failed"
npm run build || die "dashboard client build failed"
mkdir -p "${OUTPUT_DIR}/bin/client"
cp -ar ./dist "${OUTPUT_DIR}/bin/client/"
cd "${BASE_DIR}"

echo "start to compile collector"
mkdir -p "${OUTPUT_DIR}/bin/"
rm -rf "${OUTPUT_DIR}/bin/collector"
echo LD_LIBRARY_PATH=$LD_LIBRARY_PATH
CC='gcc -fstack-protector-strong -D_FORTIFY_SOURCE=2 -O2' go build -tags="${BUILD_TAGS}" -buildmode=pie -ldflags "${FLAGS}"  -o \
"${OUTPUT_DIR}"/bin/collector "${PROJECT_DIR}"/cmd/collector/main.go

cd "${OUTPUT_DIR}"
tar -czvf yr-dashboard-v0.0.1.tar.gz ./*
mkdir -p "${RUNTIME_OUTPUT_DIR}"
rm -rf "${RUNTIME_OUTPUT_DIR}/yr-dashboard-v0.0.1.tar.gz"
cp yr-dashboard-v0.0.1.tar.gz "${RUNTIME_OUTPUT_DIR}"
cd "${BASE_DIR}"