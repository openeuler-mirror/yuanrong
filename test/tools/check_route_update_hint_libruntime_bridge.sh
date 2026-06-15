#!/usr/bin/env bash
set -euo pipefail

grep -q "message RouteUpdateHint" src/libruntime/fsclient/protobuf/common.proto
grep -q "common.RouteUpdateHint routeUpdateHint" src/libruntime/fsclient/protobuf/core_service.proto
grep -q "common.RouteUpdateHint routeUpdateHint" src/libruntime/fsclient/protobuf/runtime_service.proto
grep -q "bytes[[:space:]]\+payload[[:space:]]*= 4" functionsystem/proto/posix/inner_service.proto
grep -q "common.RouteUpdateHint[[:space:]]\+routeUpdateHint[[:space:]]*= 5" functionsystem/proto/posix/inner_service.proto
grep -q "bytes[[:space:]]\+payload[[:space:]]*= 4" go/proto/posix/inner_service.proto
grep -q "common.RouteUpdateHint[[:space:]]\+routeUpdateHint[[:space:]]*= 5" go/proto/posix/inner_service.proto
grep -q "mutable_routeupdatehint" functionsystem/functionsystem/src/common/utils/generate_message.h
grep -q "GenKillResponseWithRouteUpdate" functionsystem/functionsystem/src/function_proxy/local_scheduler/instance_control/instance_ctrl_actor.cpp
grep -q "mutable_routeupdatehint()->CopyFrom(killResponse.routeupdatehint())" functionsystem/functionsystem/src/function_proxy/local_scheduler/instance_control/instance_ctrl_actor.cpp
grep -q "killResponse.mutable_routeupdatehint()->CopyFrom(forwardKillResponse.routeupdatehint())" functionsystem/functionsystem/src/function_proxy/local_scheduler/instance_control/instance_ctrl_actor.cpp
grep -q "killResp.mutable_routeupdatehint()->CopyFrom(response.routeupdatehint())" functionsystem/functionsystem/src/function_proxy/local_scheduler/instance_control/instance_ctrl_actor.cpp
grep -q "has_routeupdatehint" src/libruntime/invokeadaptor/invoke_adaptor.cpp
grep -q "SetRouteUpdateHint" src/libruntime/invokeadaptor/invoke_adaptor.cpp
grep -q "routeHintRouteAddress" api/go/libruntime/cpplibruntime/clibruntime.h
grep -q "routeHintRouteAddress" api/go/libruntime/cpplibruntime/cpplibruntime.cpp
grep -q "routeUpdateErrorFromCError" api/go/libruntime/clibruntime/clibruntime.go
grep -q "api.RouteUpdateError" api/go/libruntime/clibruntime/clibruntime.go
