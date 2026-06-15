#!/usr/bin/env bash
set -euo pipefail

normalize_proto_line_awk='function norm(s) {
  sub(/\/\/.*/, "", s)
  gsub(/[[:space:]]+/, " ", s)
  sub(/^ /, "", s)
  sub(/ $/, "", s)
  return s
}'

check_message() {
  local file=$1
  local message=$2
  awk -v msg="$message" "${normalize_proto_line_awk}
    norm(\$0) == \"message \" msg \" {\" { found = 1 }
    END { exit found ? 0 : 1 }
  " "$file" || {
    echo "missing message ${message} in ${file}" >&2
    exit 1
  }
}

check_message_field() {
  local file=$1
  local message=$2
  local expected=$3
  awk -v msg="$message" -v expected="$expected" "${normalize_proto_line_awk}
    !in_msg && norm(\$0) == \"message \" msg \" {\" { in_msg = 1; next }
    in_msg {
      line = norm(\$0)
      if (line == expected) { found = 1 }
      if (line == \"}\") { in_msg = 0 }
    }
    END { exit found ? 0 : 1 }
  " "$file" || {
    echo "missing field '${expected}' in message ${message} (${file})" >&2
    exit 1
  }
}

check_alias() {
  local file=$1
  local expected="using RouteUpdateHint = common::RouteUpdateHint;"
  awk -v expected="$expected" "${normalize_proto_line_awk}
    norm(\$0) == expected { found = 1 }
    END { exit found ? 0 : 1 }
  " "$file" || {
    echo "missing alias '${expected}' in ${file}" >&2
    exit 1
  }
}

for f in \
  functionsystem/proto/posix/common.proto \
  go/proto/posix/common.proto; do
  check_message "$f" "RouteUpdateHint"
  check_message_field "$f" "RouteUpdateHint" "string instanceID = 1;"
  check_message_field "$f" "RouteUpdateHint" "string routeAddress = 2;"
  check_message_field "$f" "RouteUpdateHint" "string proxyID = 3;"
  check_message_field "$f" "RouteUpdateHint" "bool retryable = 4;"
  check_message_field "$f" "RouteUpdateHint" "string reason = 5;"
  check_message_field "$f" "RouteUpdateHint" "int64 modRevision = 6;"
done

for f in \
  functionsystem/proto/posix/runtime_service.proto \
  go/proto/posix/runtime_service.proto; do
  check_message_field "$f" "CallResponse" "common.ErrorCode code = 1;"
  check_message_field "$f" "CallResponse" "string message = 2;"
  check_message_field "$f" "CallResponse" "common.RouteUpdateHint routeUpdateHint = 3;"
done

for f in \
  functionsystem/proto/posix/core_service.proto \
  go/proto/posix/core_service.proto; do
  check_message_field "$f" "InvokeResponse" "common.ErrorCode code = 1;"
  check_message_field "$f" "InvokeResponse" "string message = 2;"
  check_message_field "$f" "InvokeResponse" "string returnObjectID = 3;"
  check_message_field "$f" "InvokeResponse" "common.RouteUpdateHint routeUpdateHint = 4;"

  check_message_field "$f" "CallResultAck" "common.ErrorCode code = 1;"
  check_message_field "$f" "CallResultAck" "string message = 2;"
  check_message_field "$f" "CallResultAck" "common.RouteUpdateHint routeUpdateHint = 3;"

  check_message_field "$f" "KillResponse" "common.ErrorCode code = 1;"
  check_message_field "$f" "KillResponse" "string message = 2;"
  check_message_field "$f" "KillResponse" "bytes payload = 3;"
  check_message_field "$f" "KillResponse" "common.RouteUpdateHint routeUpdateHint = 4;"
done

check_alias "go/proto/pb/posix_pb.h"
check_alias "functionsystem/functionsystem/src/common/proto/pb/posix_pb.h"
