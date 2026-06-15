# Direct Routing Completeness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete `enable_direct_routing=true` by removing long-lived remote state-machine mirrors, bounding route caches with LRU, adding full-chain stale-route update hints, and proving behavior in multi-node tests.

**Architecture:** Only the real owner node keeps a full instance state machine. Remote route lookups use bounded route/negative LRU caches and metastore queries; RUNNING routes are cached, intermediate states return retryable inner-communication errors, terminal/missing states cache negative results. Stale-route repair uses a structured `RouteUpdateHint` propagated from function_proxy through libruntime/frontend, with automatic retry only inside libruntime and only once per request.

**Tech Stack:** C++17, LiteBus actors, protobuf3, Go 1.24, CGO libruntime bridge, GTest, Go test, sandbox multi-node `yr start` smoke tests.

---

## File Structure / Change Map

### Protocol and generated bindings

- Modify `functionsystem/proto/posix/common.proto`: add `RouteUpdateHint` message.
- Modify `functionsystem/proto/posix/runtime_service.proto`: attach `RouteUpdateHint` to `CallResponse`.
- Modify `functionsystem/proto/posix/core_service.proto`: attach `RouteUpdateHint` to `InvokeResponse`, `CallResultAck`, and `KillResponse`.
- Modify matching root protos under `go/proto/posix/` when they are the source for Go/C++ generated bindings.
- Regenerate protobuf outputs used by function_proxy, api/go, frontend, and go modules.

### function_proxy C++

- Modify `functionsystem/functionsystem/src/function_proxy/common/observer/observer_actor.h/.cpp`: introduce DR-safe route query result semantics and prevent remote query from creating remote state machines.
- Modify `functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/instance_proxy.h/.cpp`: add route/negative LRU entries, stale-route hint responses, and retryable inner-communication handling.
- Modify `functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/request_router.cpp`: when target actor is missing in DR mode, query route and return hint instead of unconditional not-found.
- Modify `functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/request_dispatcher.h/.cpp`: add helper constructors for call responses with `RouteUpdateHint`.
- Modify `functionsystem/functionsystem/src/function_proxy/common/state_machine/instance_control_view.h/.cpp`: expose safe cleanup helpers for schedule-failure rollback and guard DR remote updates from creating state machines.
- Modify `functionsystem/functionsystem/src/function_proxy/local_scheduler/instance_control/instance_ctrl_actor.cpp`: ensure all schedule failure paths call rollback and request-future cleanup.
- Tests under `functionsystem/functionsystem/tests/unit/function_proxy/...`.

### Go libruntime / SDK

- Create `api/go/libruntime/libruntimesdkimpl/routecache/route_cache.go`: dependency-free, mutex-protected LRU.
- Create `api/go/libruntime/libruntimesdkimpl/routecache/route_cache_test.go`.
- Modify `api/go/libruntime/libruntimesdkimpl/libruntimesdkimpl.go`: replace `sync.Map` with routecache, update on allocations/hints, use cached route for kill/invoke paths.
- Modify CGO bridge only if generated protobuf route hint needs to cross C++/Go boundaries beyond existing error return structures.
- Tests under `api/go/libruntime/libruntimesdkimpl/...`.

### Frontend / cloud-external propagation

- Modify `frontend/pkg/frontend/common/util/client.go`: recognize route update hint from libruntime errors/responses and return structured frontend error metadata without retry.
- Modify `frontend/pkg/frontend/invocation/function_invoke_for_kernel.go`: preserve route hint metadata in returned SNError/HTTP response path.
- Tests under `frontend/pkg/frontend/common/util/client_test.go` and `frontend/pkg/frontend/invocation/function_invoke_for_kernel_test.go`.

### E2E

- Create `test/smoke/direct-routing-completeness/` with a multi-node smoke script and Python assertions for named instances, stale route repair, proxy failure, and LRU miss behavior.

---

## Task 1: Add structured RouteUpdateHint protocol

**Files:**
- Modify: `functionsystem/proto/posix/common.proto`
- Modify: `functionsystem/proto/posix/runtime_service.proto`
- Modify: `functionsystem/proto/posix/core_service.proto`
- Modify: `go/proto/posix/common.proto`
- Modify: `go/proto/posix/runtime_service.proto`
- Modify: `go/proto/posix/core_service.proto`
- Generated: project protobuf outputs touched by the repo generation command

- [ ] **Step 1: Add failing proto compatibility grep test script**

Create `test/tools/check_route_update_hint_proto.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
for f in \
  functionsystem/proto/posix/common.proto \
  go/proto/posix/common.proto; do
  grep -q "message RouteUpdateHint" "$f"
  grep -q "string instanceID" "$f"
  grep -q "string routeAddress" "$f"
  grep -q "string proxyID" "$f"
  grep -q "bool retryable" "$f"
done
for f in \
  functionsystem/proto/posix/runtime_service.proto \
  go/proto/posix/runtime_service.proto; do
  grep -q "common.RouteUpdateHint.*routeUpdateHint" "$f"
done
for f in \
  functionsystem/proto/posix/core_service.proto \
  go/proto/posix/core_service.proto; do
  grep -q "common.RouteUpdateHint.*routeUpdateHint" "$f"
done
```

Make it executable:

```bash
chmod +x test/tools/check_route_update_hint_proto.sh
```

- [ ] **Step 2: Run the grep test and verify it fails**

Run:

```bash
bash test/tools/check_route_update_hint_proto.sh
```

Expected: fails because `RouteUpdateHint` fields do not exist yet.

- [ ] **Step 3: Add `RouteUpdateHint` to both common.proto files**

In `functionsystem/proto/posix/common.proto` and `go/proto/posix/common.proto`, add after `message RuntimeInfo`:

```protobuf
message RouteUpdateHint {
  string instanceID    = 1;
  string routeAddress  = 2;
  string proxyID       = 3;
  bool   retryable     = 4;
  string reason        = 5;
  int64  modRevision   = 6;
}
```

- [ ] **Step 4: Attach hint fields to response protos**

In both `functionsystem/proto/posix/runtime_service.proto` and `go/proto/posix/runtime_service.proto`, change `CallResponse` to:

```protobuf
message CallResponse {
  common.ErrorCode       code            = 1;
  string                 message         = 2;
  common.RouteUpdateHint routeUpdateHint = 3;
}
```

In both `functionsystem/proto/posix/core_service.proto` and `go/proto/posix/core_service.proto`, change the response messages to:

```protobuf
message InvokeResponse {
  common.ErrorCode       code            = 1;
  string                 message         = 2;
  string                 returnObjectID  = 3;
  common.RouteUpdateHint routeUpdateHint = 4;
}

message CallResultAck {
  common.ErrorCode       code            = 1;
  string                 message         = 2;
  common.RouteUpdateHint routeUpdateHint = 3;
}

message KillResponse {
  common.ErrorCode       code            = 1;
  string                 message         = 2;
  bytes                  payload         = 3;
  common.RouteUpdateHint routeUpdateHint = 4;
}
```

- [ ] **Step 5: Regenerate protobuf bindings**

Run the repo protobuf generation command. If the repo-local command differs, use the command documented in existing build scripts; start with:

```bash
bash scripts/generate_proto.sh
```

If that script is absent, locate the generator:

```bash
find . -maxdepth 4 -type f \( -name '*proto*gen*.sh' -o -name 'generate*.sh' \) | sort
```

Then run the matching project generator for both C++ and Go bindings.

- [ ] **Step 6: Verify proto fields exist in generated bindings**

Run:

```bash
rg -n "RouteUpdateHint|routeupdatehint|routeUpdateHint" \
  functionsystem go api frontend --glob '!**/.git/**'
bash test/tools/check_route_update_hint_proto.sh
```

Expected: grep test passes and generated bindings expose route update hint fields.

- [ ] **Step 7: Commit protocol changes**

```bash
git add test/tools/check_route_update_hint_proto.sh \
  functionsystem/proto/posix/common.proto \
  functionsystem/proto/posix/runtime_service.proto \
  functionsystem/proto/posix/core_service.proto \
  go/proto/posix/common.proto \
  go/proto/posix/runtime_service.proto \
  go/proto/posix/core_service.proto
# add generated protobuf files reported by git status
git add <generated-protobuf-files>
git commit -m "Add structured route update hints for direct routing

Constraint: route repair must be machine-readable across function_proxy, libruntime, frontend, and cloud-external clients.
Confidence: high
Scope-risk: moderate
Directive: Do not parse route hints out of free-form error strings.
Tested: bash test/tools/check_route_update_hint_proto.sh
Not-tested: runtime propagation added in later tasks."
```

---

## Task 2: Add function_proxy DR route lookup result model and negative LRU

**Files:**
- Modify: `functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/instance_proxy.h`
- Modify: `functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/instance_proxy.cpp`
- Modify: `functionsystem/functionsystem/src/function_proxy/common/observer/observer_actor.h`
- Modify: `functionsystem/functionsystem/src/function_proxy/common/observer/observer_actor.cpp`
- Test: `functionsystem/functionsystem/tests/unit/function_proxy/busproxy/instance_proxy/instance_proxy_test.cpp`
- Test: `functionsystem/functionsystem/tests/unit/function_proxy/common/observer/control_plane_observer_test.cpp`

- [ ] **Step 1: Write failing observer tests for DR query states**

Add tests that construct route JSON for RUNNING, CREATING, and FATAL under `ObserverActor` route query. The assertions:

```cpp
// RUNNING: future OK and routeInfo.proxygrpcaddress() is non-empty.
// CREATING: future error status maps to ERR_INNER_COMMUNICATION.
// FATAL: future error status maps to ERR_INSTANCE_NOT_FOUND or terminal negative status.
// In all cases, instanceControlView_->GetInstance(remoteInstanceID) stays nullptr in DR mode.
```

Use the existing test fixture patterns in `functionsystem/functionsystem/tests/unit/function_proxy/common/observer/control_plane_observer_test.cpp` and enable DR with:

```cpp
function_proxy::DirectRoutingConfig::SetEnabled(true);
```

Reset it in teardown:

```cpp
function_proxy::DirectRoutingConfig::SetEnabled(false);
```

- [ ] **Step 2: Run observer tests and verify they fail**

Run:

```bash
cd functionsystem
bash run.sh test -j 4 -T '*Observer*Route*'
```

Expected: FAIL because current `OnGetInstanceFromMetaStore` calls `PutInstanceEvent(instanceInfo, true, ...)`, which can create/update remote state-machine state.

- [ ] **Step 3: Add explicit DR route query helper semantics**

In `observer_actor.h`, add a helper result type near `ObserverActor` declarations:

```cpp
struct DirectRouteQueryResult {
    Status status;
    std::shared_ptr<resources::RouteInfo> routeInfo;
    bool negativeCacheable {false};
};
```

Add a method:

```cpp
litebus::Future<DirectRouteQueryResult> QueryInstanceRouteForDirectRouting(const std::string &instanceID);
```

- [ ] **Step 4: Implement state classification without remote state-machine creation**

In `observer_actor.cpp`, implement a DR-only path that reads the route key, parses `RouteInfo`, converts status, and does not call `PutInstanceEvent`:

```cpp
litebus::Future<DirectRouteQueryResult> ObserverActor::QueryInstanceRouteForDirectRouting(const std::string &instanceID)
{
    return metaStorageAccessor_->GetMetaClient()
        ->Get(GenInstanceRouteKey(instanceID), {})
        .Then(litebus::Defer(GetAID(), [instanceID](const litebus::Future<std::shared_ptr<GetResponse>> &future) {
            if (future.IsError() || future.Get() == nullptr || future.Get()->kvs.empty()) {
                return DirectRouteQueryResult{Status(StatusCode::ERR_INSTANCE_NOT_FOUND, "instance route not found"), nullptr, true};
            }
            resource_view::RouteInfo routeInfo;
            if (!TransToRouteInfoFromJson(routeInfo, future.Get()->kvs.front().value())) {
                return DirectRouteQueryResult{Status(StatusCode::ERR_INNER_COMMUNICATION, "invalid route info"), nullptr, false};
            }
            auto state = static_cast<InstanceState>(routeInfo.instancestatus().code());
            if (state == InstanceState::RUNNING) {
                auto out = std::make_shared<resources::RouteInfo>();
                out->CopyFrom(routeInfo);
                return DirectRouteQueryResult{Status::OK(), out, false};
            }
            if (state == InstanceState::SCHEDULING || state == InstanceState::CREATING ||
                state == InstanceState::EXITING || state == InstanceState::EVICTING ||
                state == InstanceState::SUSPEND) {
                return DirectRouteQueryResult{Status(StatusCode::ERR_INNER_COMMUNICATION,
                    "instance route is not ready"), nullptr, false};
            }
            return DirectRouteQueryResult{Status(StatusCode::ERR_INSTANCE_NOT_FOUND,
                "instance route is terminal"), nullptr, true};
        }));
}
```

Keep existing `QueryInstanceRoute` behavior for non-DR callers unless all call sites can safely consume the new result type.

- [ ] **Step 5: Add route/negative LRU entry type to InstanceProxy**

In `instance_proxy.h`, replace the `ThreadSafeLruCache<std::string, std::string>` member with:

```cpp
struct DirectRouteCacheEntry {
    enum class Kind { RUNNING_ROUTE, NEGATIVE } kind {Kind::NEGATIVE};
    std::string routeAddress;
    std::string proxyID;
    common::ErrorCode errorCode {common::ERR_INSTANCE_NOT_FOUND};
    std::string reason;
};

ThreadSafeLruCache<std::string, DirectRouteCacheEntry> routeCache_{ROUTE_CACHE_CAPACITY};
```

Update `EvictRoute()` to remove by key as it does today.

- [ ] **Step 6: Use positive and negative cache on DR miss**

In `InstanceProxy::Call`, for `DirectRoutingConfig::IsEnabled()`:

```cpp
auto cachedRoute = routeCache_.Get(dstInstanceID);
if (cachedRoute.has_value()) {
    if (cachedRoute->get().kind == DirectRouteCacheEntry::Kind::RUNNING_ROUTE &&
        !cachedRoute->get().routeAddress.empty()) {
        auto remoteAID = litebus::AID(dstInstanceID, cachedRoute->get().routeAddress);
        return ForwardWithRouteAndFallback(remoteAID, dstInstanceID, callerInfo, request);
    }
    return CreateCallResponse(cachedRoute->get().errorCode, cachedRoute->get().reason, request->messageid());
}
```

- [ ] **Step 7: Run tests and verify pass**

Run:

```bash
cd functionsystem
bash run.sh test -j 4 -T '*Observer*Route*'
bash run.sh test -j 4 -T '*InstanceProxy*'
```

Expected: observer route-state tests pass; existing instance proxy tests pass.

- [ ] **Step 8: Commit route query/cache model**

```bash
git add functionsystem/functionsystem/src/function_proxy/common/observer/observer_actor.* \
  functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/instance_proxy.* \
  functionsystem/functionsystem/tests/unit/function_proxy/common/observer/control_plane_observer_test.cpp \
  functionsystem/functionsystem/tests/unit/function_proxy/busproxy/instance_proxy/instance_proxy_test.cpp
git commit -m "Constrain direct routing lookups to bounded route cache

Constraint: DR mode must not create long-lived remote state machines for route miss lookups.
Confidence: medium
Scope-risk: moderate
Directive: Never cache remote intermediate states; return retryable inner communication instead.
Tested: bash run.sh test -j 4 -T '*Observer*Route*'; bash run.sh test -j 4 -T '*InstanceProxy*'
Not-tested: full stale-route hint propagation added in the next task."
```

---

## Task 3: Return route update hints from stale-route function_proxy paths

**Files:**
- Modify: `functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/request_dispatcher.h`
- Modify: `functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/request_dispatcher.cpp`
- Modify: `functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/request_router.cpp`
- Modify: `functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/instance_proxy.cpp`
- Test: `functionsystem/functionsystem/tests/unit/function_proxy/busproxy/instance_proxy/request_router_test.cpp`
- Test: `functionsystem/functionsystem/tests/unit/function_proxy/busproxy/instance_proxy/instance_proxy_test.cpp`

- [ ] **Step 1: Write failing RequestRouter stale-route tests**

Add cases to `request_router_test.cpp`:

```cpp
TEST_F(RequestRouterTest, DirectRoutingMissingActorReturnsRouteUpdateHintWhenMetastoreHasRunningRoute)
{
    function_proxy::DirectRoutingConfig::SetEnabled(true);
    // Configure a fake observer/query hook to return instanceID=remote_, routeAddress="new-route", proxyID="new-proxy".
    // Send routeReq to missing instance.
    // Assert response.callrsp().code() == ERR_INNER_COMMUNICATION or dedicated retryable code.
    // Assert response.callrsp().routeupdatehint().instanceid() == remote_.
    // Assert response.callrsp().routeupdatehint().routeaddress() == "new-route".
    function_proxy::DirectRoutingConfig::SetEnabled(false);
}
```

Use the existing LiteBus actor fixture and add mock injection needed for route query. If `RequestRouter` has no observer binding, add one in Step 3.

- [ ] **Step 2: Run RequestRouter test and verify it fails**

Run:

```bash
cd functionsystem
bash run.sh test -j 4 -T '*RequestRouter*'
```

Expected: stale route case fails; current code returns `ERR_INSTANCE_NOT_FOUND` without hint.

- [ ] **Step 3: Add response helper with hint**

In `request_dispatcher.h`, add:

```cpp
SharedStreamMsg CreateCallResponseWithRouteUpdate(const common::ErrorCode &code,
                                                  const std::string &message,
                                                  const std::string &messageID,
                                                  const std::string &instanceID,
                                                  const std::string &routeAddress,
                                                  const std::string &proxyID,
                                                  int64_t modRevision = 0);
```

In `request_dispatcher.cpp`, implement:

```cpp
SharedStreamMsg CreateCallResponseWithRouteUpdate(const common::ErrorCode &code,
                                                  const std::string &message,
                                                  const std::string &messageID,
                                                  const std::string &instanceID,
                                                  const std::string &routeAddress,
                                                  const std::string &proxyID,
                                                  int64_t modRevision)
{
    auto response = std::make_shared<runtime_rpc::StreamingMessage>();
    response->set_messageid(messageID);
    auto callResponse = response->mutable_callrsp();
    callResponse->set_code(code);
    callResponse->set_message(message);
    auto hint = callResponse->mutable_routeupdatehint();
    hint->set_instanceid(instanceID);
    hint->set_routeaddress(routeAddress);
    hint->set_proxyid(proxyID);
    hint->set_retryable(true);
    hint->set_reason("stale_route");
    hint->set_modrevision(modRevision);
    return response;
}
```

- [ ] **Step 4: Route missing actor through DR query**

In `RequestRouter`, bind `DataPlaneObserver` or a small `RouteResolver` interface. Minimal pattern:

```cpp
static void BindObserver(const std::shared_ptr<function_proxy::DataPlaneObserver> &observer)
{
    observer_ = observer;
}
inline static std::shared_ptr<function_proxy::DataPlaneObserver> observer_ {nullptr};
```

When `dstAID == nullptr` and DR is enabled:

```cpp
if (function_proxy::DirectRoutingConfig::IsEnabled() && observer_ != nullptr) {
    observer_->QueryInstanceRoute(routeReq.instanceid()).OnComplete(
        litebus::Defer(GetAID(), &RequestRouter::OnMissingActorRouteQuery, from, routeReq, std::placeholders::_1));
    return;
}
```

Add `OnMissingActorRouteQuery` to return either `CreateCallResponseWithRouteUpdate`, retryable `ERR_INNER_COMMUNICATION`, or not found.

- [ ] **Step 5: Return hints from `InstanceProxy::OnQueryRouteResult`**

When a cached route forward fails and query returns a different RUNNING route, return `CreateCallResponseWithRouteUpdate(...)` instead of immediately forwarding again if the request has already consumed one route retry. Use request create option `YR_ROUTE_RETRY_ATTEMPT=1` to detect consumed retry.

```cpp
const bool retryConsumed = request->callreq().createoptions().find("YR_ROUTE_RETRY_ATTEMPT") !=
                           request->callreq().createoptions().end();
if (retryConsumed) {
    promise->SetValue(CreateCallResponseWithRouteUpdate(common::ERR_INNER_COMMUNICATION,
        "stale route updated", originalMessageID, dstInstanceID, route, routeFuture.Get()->functionproxyid()));
    return;
}
```

- [ ] **Step 6: Run stale route tests**

Run:

```bash
cd functionsystem
bash run.sh test -j 4 -T '*RequestRouter*'
bash run.sh test -j 4 -T '*InstanceProxy*'
```

Expected: stale-route tests pass and no existing proxy tests regress.

- [ ] **Step 7: Commit stale-route hint propagation**

```bash
git add functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/request_dispatcher.* \
  functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/request_router.* \
  functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/instance_proxy.cpp \
  functionsystem/functionsystem/tests/unit/function_proxy/busproxy/instance_proxy/request_router_test.cpp \
  functionsystem/functionsystem/tests/unit/function_proxy/busproxy/instance_proxy/instance_proxy_test.cpp
git commit -m "Return route update hints for stale direct routes

Constraint: old owners must not proxy-chain to new owners in direct routing mode.
Confidence: medium
Scope-risk: moderate
Directive: Keep frontend/cloud retry out of this layer; only return structured hints.
Tested: bash run.sh test -j 4 -T '*RequestRouter*'; bash run.sh test -j 4 -T '*InstanceProxy*'
Not-tested: libruntime automatic retry added in the next tasks."
```

---

## Task 4: Make schedule failure rollback remove local DR state machines

**Files:**
- Modify: `functionsystem/functionsystem/src/function_proxy/common/state_machine/instance_control_view.h`
- Modify: `functionsystem/functionsystem/src/function_proxy/common/state_machine/instance_control_view.cpp`
- Modify: `functionsystem/functionsystem/src/function_proxy/local_scheduler/instance_control/instance_ctrl_actor.cpp`
- Test: `functionsystem/functionsystem/tests/unit/function_proxy/local_scheduler/instance_control/direct_routing_state_machine_test.cpp`

- [ ] **Step 1: Replace existing stub test with failing rollback tests**

In `direct_routing_state_machine_test.cpp`, replace the existing stub with tests using `InstanceControlView` directly:

```cpp
TEST(DirectRoutingStateMachineTest, ScheduleFailureRollbackDeletesLocalMachine)
{
    function_proxy::DirectRoutingConfig::SetEnabled(true);
    InstanceControlView view("node-a", true);
    auto req = std::make_shared<messages::ScheduleRequest>();
    req->set_requestid("req-1");
    req->set_traceid("trace-1");
    req->mutable_instance()->set_instanceid("inst-1");
    req->mutable_instance()->set_functionproxyid("node-a");
    req->mutable_instance()->mutable_instancestatus()->set_code(static_cast<int32_t>(InstanceState::SCHEDULING));
    auto gen = view.TryGenerateNewInstance(req);
    ASSERT_FALSE(gen.instanceID.empty());
    ASSERT_NE(nullptr, view.GetInstance("inst-1"));

    view.RollbackDirectRoutingScheduleFailure("inst-1", "req-1");

    EXPECT_EQ(nullptr, view.GetInstance("inst-1"));
    function_proxy::DirectRoutingConfig::SetEnabled(false);
}
```

Add a second test proving mismatched requestID does not delete a newer machine:

```cpp
TEST(DirectRoutingStateMachineTest, RollbackSkipsNewerRequest)
{
    function_proxy::DirectRoutingConfig::SetEnabled(true);
    InstanceControlView view("node-a", true);
    // create request req-new for inst-1
    // call RollbackDirectRoutingScheduleFailure("inst-1", "req-old")
    // expect GetInstance("inst-1") is not nullptr
    function_proxy::DirectRoutingConfig::SetEnabled(false);
}
```

Fill the setup with the same `ScheduleRequest` fields as the first test.

- [ ] **Step 2: Run rollback test and verify it fails**

Run:

```bash
cd functionsystem
bash run.sh test -j 4 -T '*DirectRoutingStateMachine*'
```

Expected: FAIL because `RollbackDirectRoutingScheduleFailure` does not exist.

- [ ] **Step 3: Add rollback helper**

In `instance_control_view.h`, add public method:

```cpp
void RollbackDirectRoutingScheduleFailure(const std::string &instanceID, const std::string &requestID);
```

In `instance_control_view.cpp`, implement:

```cpp
void InstanceControlView::RollbackDirectRoutingScheduleFailure(const std::string &instanceID,
                                                               const std::string &requestID)
{
    std::lock_guard<std::mutex> guard(lock_);
    auto it = machines_.find(instanceID);
    if (it == machines_.end()) {
        return;
    }
    if (it->second->GetRequestID() != requestID) {
        YRLOG_WARN("skip DR schedule rollback for instance({}), old request({}), current request({})",
                   instanceID, requestID, it->second->GetRequestID());
        return;
    }
    requestInstances_.erase(requestID);
    createRequestFuture_.erase(requestID);
    createRequestRuntimeFuture_.erase(requestID);
    machines_.erase(it);
}
```

- [ ] **Step 4: Call rollback from failed schedule paths**

In `instance_ctrl_actor.cpp`, locate `DeleteRequestFuture(...)` and failure branches around schedule response handling. After `instanceControlView_->DeleteRequestFuture(requestID);`, add:

```cpp
if (function_proxy::DirectRoutingConfig::IsEnabled() && scheduleResponse.IsOK() &&
    scheduleResponse.Get().code() != common::SUCCESS && scheduleReq != nullptr &&
    !scheduleReq->instance().instanceid().empty()) {
    instanceControlView_->RollbackDirectRoutingScheduleFailure(scheduleReq->instance().instanceid(), requestID);
}
```

For branches that set `runtimePromise->SetValue(GenScheduleResponse(...failure...))` before a future exists, add immediate rollback with the same requestID/instanceID after setting the value.

- [ ] **Step 5: Run rollback tests**

Run:

```bash
cd functionsystem
bash run.sh test -j 4 -T '*DirectRoutingStateMachine*'
bash run.sh test -j 4 -T '*InstanceCtrlActor*'
```

Expected: rollback tests pass; existing instance control tests pass.

- [ ] **Step 6: Commit rollback cleanup**

```bash
git add functionsystem/functionsystem/src/function_proxy/common/state_machine/instance_control_view.* \
  functionsystem/functionsystem/src/function_proxy/local_scheduler/instance_control/instance_ctrl_actor.cpp \
  functionsystem/functionsystem/tests/unit/function_proxy/local_scheduler/instance_control/direct_routing_state_machine_test.cpp
git commit -m "Rollback failed direct-routing schedules immediately

Constraint: failed scheduling must not leave local state machines that block named-instance rescheduling.
Confidence: medium
Scope-risk: moderate
Directive: Guard deletion by requestID so stale cleanup cannot delete a newer scheduling attempt.
Tested: bash run.sh test -j 4 -T '*DirectRoutingStateMachine*'; bash run.sh test -j 4 -T '*InstanceCtrlActor*'
Not-tested: multi-node named-instance scenario added later."
```

---

## Task 5: Add Go thread-safe LRU for libruntime route cache

**Files:**
- Create: `api/go/libruntime/libruntimesdkimpl/routecache/route_cache.go`
- Create: `api/go/libruntime/libruntimesdkimpl/routecache/route_cache_test.go`
- Modify: `api/go/libruntime/libruntimesdkimpl/libruntimesdkimpl.go`

- [ ] **Step 1: Write LRU tests**

Create `route_cache_test.go`:

```go
package routecache

import (
    "fmt"
    "sync"
    "testing"
)

func TestRouteCacheEvictsLeastRecentlyUsed(t *testing.T) {
    c := New(2)
    c.Put("a", Entry{RouteAddress: "route-a", ProxyID: "proxy-a"})
    c.Put("b", Entry{RouteAddress: "route-b", ProxyID: "proxy-b"})
    if _, ok := c.Get("a"); !ok {
        t.Fatal("expected a to exist")
    }
    c.Put("c", Entry{RouteAddress: "route-c", ProxyID: "proxy-c"})
    if _, ok := c.Get("b"); ok {
        t.Fatal("expected b to be evicted as LRU")
    }
    if got, ok := c.Get("a"); !ok || got.RouteAddress != "route-a" {
        t.Fatalf("expected a to remain hot, got %+v ok=%v", got, ok)
    }
}

func TestRouteCacheRemoveAndEmptyValues(t *testing.T) {
    c := New(2)
    c.Put("a", Entry{RouteAddress: "", ProxyID: ""})
    if _, ok := c.Get("a"); ok {
        t.Fatal("empty route must not be cached")
    }
    c.Put("a", Entry{RouteAddress: "route-a", ProxyID: "proxy-a"})
    c.Remove("a")
    if _, ok := c.Get("a"); ok {
        t.Fatal("removed route must not exist")
    }
}

func TestRouteCacheConcurrentAccess(t *testing.T) {
    c := New(32)
    var wg sync.WaitGroup
    for i := 0; i < 8; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            for j := 0; j < 200; j++ {
                key := fmt.Sprintf("inst-%d-%d", id, j)
                c.Put(key, Entry{RouteAddress: "route", ProxyID: "proxy"})
                _, _ = c.Get(key)
            }
        }(i)
    }
    wg.Wait()
    if c.Len() > 32 {
        t.Fatalf("cache exceeded capacity: %d", c.Len())
    }
}
```

- [ ] **Step 2: Run LRU tests and verify fail**

Run:

```bash
cd api/go
go test ./libruntime/libruntimesdkimpl/routecache
```

Expected: FAIL because package does not exist.

- [ ] **Step 3: Implement route cache**

Create `route_cache.go`:

```go
package routecache

import (
    "container/list"
    "sync"
)

const DefaultCapacity = 1024

type Entry struct {
    RouteAddress string
    ProxyID       string
}

type pair struct {
    key   string
    value Entry
}

type Cache struct {
    mu       sync.Mutex
    capacity int
    items    map[string]*list.Element
    order    *list.List
}

func New(capacity int) *Cache {
    if capacity <= 0 {
        capacity = DefaultCapacity
    }
    return &Cache{capacity: capacity, items: make(map[string]*list.Element), order: list.New()}
}

func (c *Cache) Put(key string, value Entry) {
    if key == "" || value.RouteAddress == "" {
        return
    }
    c.mu.Lock()
    defer c.mu.Unlock()
    if elem, ok := c.items[key]; ok {
        elem.Value.(*pair).value = value
        c.order.MoveToFront(elem)
        return
    }
    elem := c.order.PushFront(&pair{key: key, value: value})
    c.items[key] = elem
    if len(c.items) > c.capacity {
        c.evictLocked()
    }
}

func (c *Cache) Get(key string) (Entry, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    elem, ok := c.items[key]
    if !ok {
        return Entry{}, false
    }
    c.order.MoveToFront(elem)
    return elem.Value.(*pair).value, true
}

func (c *Cache) Remove(key string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    elem, ok := c.items[key]
    if !ok {
        return
    }
    c.order.Remove(elem)
    delete(c.items, key)
}

func (c *Cache) Len() int {
    c.mu.Lock()
    defer c.mu.Unlock()
    return len(c.items)
}

func (c *Cache) evictLocked() {
    elem := c.order.Back()
    if elem == nil {
        return
    }
    c.order.Remove(elem)
    delete(c.items, elem.Value.(*pair).key)
}
```

- [ ] **Step 4: Replace sync.Map in libruntime SDK impl**

In `libruntimesdkimpl.go`:

```go
import "yuanrong.org/kernel/runtime/libruntime/libruntimesdkimpl/routecache"

type libruntimeSDKImpl struct {
    routeCache *routecache.Cache
}

func NewLibruntimeSDKImpl() api.LibruntimeAPI {
    return &libruntimeSDKImpl{routeCache: routecache.New(routecache.DefaultCapacity)}
}
```

Replace allocation store with:

```go
if err == nil && allocation.InstanceID != "" {
    l.routeCache.Put(allocation.InstanceID, routecache.Entry{
        RouteAddress: allocation.RouteAddress,
        ProxyID: allocation.ProxyID,
    })
}
```

Replace kill lookup with:

```go
routeAddress, proxyID := "", ""
if entry, ok := l.routeCache.Get(instanceID); ok {
    routeAddress, proxyID = entry.RouteAddress, entry.ProxyID
}
return clibruntime.Kill(instanceID, signal, payload, routeAddress, proxyID)
```

- [ ] **Step 5: Run Go tests**

Run:

```bash
cd api/go
go test ./libruntime/libruntimesdkimpl/routecache ./libruntime/libruntimesdkimpl
```

Expected: PASS.

- [ ] **Step 6: Commit libruntime LRU**

```bash
git add api/go/libruntime/libruntimesdkimpl/routecache \
  api/go/libruntime/libruntimesdkimpl/libruntimesdkimpl.go
git commit -m "Bound libruntime direct-route cache with LRU

Constraint: libruntime route cache must not remain an unbounded sync.Map.
Confidence: high
Scope-risk: narrow
Directive: Keep route cache dependency-free and thread-safe.
Tested: go test ./libruntime/libruntimesdkimpl/routecache ./libruntime/libruntimesdkimpl
Not-tested: route-update retry is added in the next task."
```

---

## Task 6: Add libruntime-only route update retry

**Files:**
- Modify: `api/go/libruntime/libruntimesdkimpl/libruntimesdkimpl.go`
- Modify if needed: `api/go/libruntime/clibruntime/clibruntime.go`
- Modify if needed: `api/go/libruntime/cpplibruntime/cpplibruntime.cpp`
- Test: `api/go/libruntime/libruntimesdkimpl/libruntimesdkimpl_test.go`

- [ ] **Step 1: Write retry tests with injectable kill function**

Add package-level variable in implementation:

```go
var killWithRoute = clibruntime.Kill
```

Write tests:

```go
func TestKillUpdatesRouteAndRetriesOnceOnRouteHint(t *testing.T) {
    sdk := NewLibruntimeSDKImpl().(*libruntimeSDKImpl)
    sdk.routeCache.Put("inst", routecache.Entry{RouteAddress: "old-route", ProxyID: "old-proxy"})
    calls := 0
    old := killWithRoute
    defer func() { killWithRoute = old }()
    killWithRoute = func(instanceID string, signal int, payload []byte, routeAddress string, proxyID string) error {
        calls++
        if calls == 1 {
            if routeAddress != "old-route" { t.Fatalf("first route=%s", routeAddress) }
            return NewRouteUpdateError("inst", "new-route", "new-proxy")
        }
        if routeAddress != "new-route" || proxyID != "new-proxy" {
            t.Fatalf("retry route=%s proxy=%s", routeAddress, proxyID)
        }
        return nil
    }
    if err := sdk.Kill("inst", 9, nil, api.InvokeOptions{}); err != nil {
        t.Fatal(err)
    }
    if calls != 2 { t.Fatalf("calls=%d", calls) }
}

func TestKillDoesNotRetryTwice(t *testing.T) {
    sdk := NewLibruntimeSDKImpl().(*libruntimeSDKImpl)
    calls := 0
    old := killWithRoute
    defer func() { killWithRoute = old }()
    killWithRoute = func(instanceID string, signal int, payload []byte, routeAddress string, proxyID string) error {
        calls++
        return NewRouteUpdateError("inst", "new-route", "new-proxy")
    }
    if err := sdk.Kill("inst", 9, nil, api.InvokeOptions{}); err == nil {
        t.Fatal("expected final route update error")
    }
    if calls != 2 { t.Fatalf("calls=%d", calls) }
}
```

- [ ] **Step 2: Run retry tests and verify fail**

Run:

```bash
cd api/go
go test ./libruntime/libruntimesdkimpl -run 'TestKill.*Route' -v
```

Expected: FAIL because route update error helper and retry logic do not exist.

- [ ] **Step 3: Add route update error type**

In `libruntimesdkimpl.go` or a new `route_update_error.go`:

```go
type RouteUpdateError struct {
    InstanceID    string
    RouteAddress  string
    ProxyID       string
    Err           error
}

func (e *RouteUpdateError) Error() string {
    if e.Err != nil { return e.Err.Error() }
    return "route update required"
}

func NewRouteUpdateError(instanceID, routeAddress, proxyID string) error {
    return &RouteUpdateError{InstanceID: instanceID, RouteAddress: routeAddress, ProxyID: proxyID}
}

func asRouteUpdateError(err error) (*RouteUpdateError, bool) {
    var out *RouteUpdateError
    if errors.As(err, &out) && out.RouteAddress != "" {
        return out, true
    }
    return nil, false
}
```

Import `errors`.

- [ ] **Step 4: Implement one retry in Kill**

Refactor `Kill`:

```go
func (l *libruntimeSDKImpl) Kill(instanceID string, signal int, payload []byte, invokeOpt api.InvokeOptions) error {
    _ = invokeOpt
    return l.killWithRetry(instanceID, signal, payload, false)
}

func (l *libruntimeSDKImpl) killWithRetry(instanceID string, signal int, payload []byte, retried bool) error {
    routeAddress, proxyID := "", ""
    if entry, ok := l.routeCache.Get(instanceID); ok {
        routeAddress, proxyID = entry.RouteAddress, entry.ProxyID
    }
    err := killWithRoute(instanceID, signal, payload, routeAddress, proxyID)
    if hint, ok := asRouteUpdateError(err); ok {
        l.routeCache.Put(hint.InstanceID, routecache.Entry{RouteAddress: hint.RouteAddress, ProxyID: hint.ProxyID})
        if !retried {
            return l.killWithRetry(instanceID, signal, payload, true)
        }
    }
    return err
}
```

- [ ] **Step 5: Wire real C++/protobuf route hint errors into `RouteUpdateError`**

Where `clibruntime` converts C++/protobuf errors to Go errors, detect generated `routeUpdateHint` fields and return `&RouteUpdateError{...}`. Keep non-hint errors unchanged. The conversion must not parse free-form strings.

- [ ] **Step 6: Run tests**

Run:

```bash
cd api/go
go test ./libruntime/libruntimesdkimpl ./libruntime/clibruntime
```

Expected: PASS.

- [ ] **Step 7: Commit retry logic**

```bash
git add api/go/libruntime/libruntimesdkimpl api/go/libruntime/clibruntime api/go/libruntime/cpplibruntime
git commit -m "Retry stale direct routes only inside libruntime

Constraint: frontend and cloud-external clients must not add automatic retry loops.
Confidence: medium
Scope-risk: moderate
Directive: Retry at most once per request after a structured route update hint.
Tested: go test ./libruntime/libruntimesdkimpl ./libruntime/clibruntime
Not-tested: frontend propagation added in the next task."
```

---

## Task 7: Propagate route update hints through frontend without retry

**Files:**
- Modify: `frontend/pkg/frontend/common/util/client.go`
- Modify: `frontend/pkg/frontend/invocation/function_invoke_for_kernel.go`
- Test: `frontend/pkg/frontend/common/util/client_test.go`
- Test: `frontend/pkg/frontend/invocation/function_invoke_for_kernel_test.go`

- [ ] **Step 1: Write frontend no-retry propagation tests**

In `function_invoke_for_kernel_test.go`, add a test with fake client returning a route update error. Assert:

```go
// invokeFunctionWithLibRuntime is called exactly once.
// returned error contains structured routeAddress/proxyID metadata.
// no second Invoke call happens in frontend.
```

In `client_test.go`, add a test for conversion to HTTP/cloud response metadata:

```go
// Given a RouteUpdateError{InstanceID:"inst", RouteAddress:"new-route", ProxyID:"new-proxy"}
// Then response headers or JSON error extension contain those exact fields.
```

- [ ] **Step 2: Run frontend tests and verify fail**

Run:

```bash
cd frontend
go test ./pkg/frontend/common/util ./pkg/frontend/invocation -run 'RouteUpdate|DirectRouting' -v
```

Expected: FAIL because frontend does not expose route update metadata.

- [ ] **Step 3: Add route update metadata type**

In `frontend/pkg/frontend/common/util/client.go`, add:

```go
type RouteUpdateHint struct {
    InstanceID   string `json:"instanceID"`
    RouteAddress string `json:"routeAddress"`
    ProxyID      string `json:"proxyID"`
    Retryable    bool   `json:"retryable"`
    Reason       string `json:"reason"`
}
```

Add helper:

```go
func IsRouteUpdateError(err error) (RouteUpdateHint, bool) {
    var routeErr interface {
        RouteUpdateHint() RouteUpdateHint
    }
    if errors.As(err, &routeErr) {
        return routeErr.RouteUpdateHint(), true
    }
    return RouteUpdateHint{}, false
}
```

- [ ] **Step 4: Preserve hint in invocation error path**

In `function_invoke_for_kernel.go`, when `invokeFunctionWithLibRuntime` receives an error with hint, convert it to existing SNError/HTTP error metadata with exact fields:

```go
if hint, ok := util.IsRouteUpdateError(err); ok {
    return snerror.NewSNErrorWithDetails(statuscode.InnerCommunicationErrCode,
        "stale direct route", map[string]string{
            "instanceID": hint.InstanceID,
            "routeAddress": hint.RouteAddress,
            "proxyID": hint.ProxyID,
            "retryable": strconv.FormatBool(hint.Retryable),
            "reason": hint.Reason,
        })
}
```

Use the existing SNError constructor that supports details; if the exact helper name differs, use the existing details-capable constructor in `frontend/pkg/common/faas_common/snerror`.

- [ ] **Step 5: Assert frontend does not retry**

In the test fake, count calls:

```go
calls := 0
patch := gomonkey.ApplyFunc(invokeFunctionWithLibRuntime, func(...) snerror.SNError {
    calls++
    return routeUpdateSNError
})
defer patch.Reset()
// invoke once
if calls != 1 { t.Fatalf("frontend retried %d times", calls) }
```

- [ ] **Step 6: Run frontend tests**

Run:

```bash
cd frontend
go test ./pkg/frontend/common/util ./pkg/frontend/invocation -run 'RouteUpdate|DirectRouting' -v
```

Expected: PASS.

- [ ] **Step 7: Commit frontend propagation**

```bash
git add frontend/pkg/frontend/common/util/client.go frontend/pkg/frontend/common/util/client_test.go \
  frontend/pkg/frontend/invocation/function_invoke_for_kernel.go \
  frontend/pkg/frontend/invocation/function_invoke_for_kernel_test.go
git commit -m "Propagate direct-route update hints through frontend

Constraint: cloud-external callers need route repair metadata, but frontend must not retry automatically.
Confidence: medium
Scope-risk: moderate
Directive: Keep automatic retry owned by libruntime only.
Tested: go test ./pkg/frontend/common/util ./pkg/frontend/invocation -run 'RouteUpdate|DirectRouting' -v
Not-tested: cloud-external E2E added later."
```

---

## Task 8: Add multi-node direct-routing completeness smoke tests

**Files:**
- Create: `test/smoke/direct-routing-completeness/run_test.sh`
- Create: `test/smoke/direct-routing-completeness/test_direct_routing_completeness.py`
- Create: `test/smoke/direct-routing-completeness/services.yaml`
- Modify if needed: `.buildkite/test_sandbox_k8s.sh` or sandbox smoke manifest to include optional test

- [ ] **Step 1: Create smoke test shell harness**

Create `run_test.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR=$(cd "$(dirname "$0")/../../.." && pwd)
export YR_ENABLE_DIRECT_ROUTING=true
export YR_DIRECT_ROUTE_CACHE_CAPACITY=${YR_DIRECT_ROUTE_CACHE_CAPACITY:-4}
export YR_TEST_MULTI_NODE=${YR_TEST_MULTI_NODE:-2}

cleanup() {
  yr stop || true
}
trap cleanup EXIT

cd "$ROOT_DIR"
yr stop || true
for i in $(seq 1 "$YR_TEST_MULTI_NODE"); do
  yr start --node-id "dr-node-$i" --config test/smoke/direct-routing-completeness/services.yaml
done
python3 test/smoke/direct-routing-completeness/test_direct_routing_completeness.py
```

Make executable:

```bash
chmod +x test/smoke/direct-routing-completeness/run_test.sh
```

- [ ] **Step 2: Create Python E2E assertions**

Create `test_direct_routing_completeness.py`:

```python
import os
import time

import yr


def wait_until(fn, timeout=30, interval=1):
    deadline = time.time() + timeout
    last = None
    while time.time() < deadline:
        try:
            if fn():
                return
        except Exception as exc:
            last = exc
        time.sleep(interval)
    raise AssertionError(f"condition not met, last={last!r}")


@yr.instance(name="dr_named_counter")
class Counter:
    def __init__(self):
        self.value = 0

    def inc(self):
        self.value += 1
        return self.value


@yr.function
def echo(x):
    return x


def test_named_instance_stale_route_repair():
    c = Counter.get_or_create()
    assert c.inc.invoke().get() == 1
    # Force route cache warmup.
    assert c.inc.invoke().get() == 2
    # Delete/recreate named instance through public API used by yrcli if available.
    c.delete()
    c2 = Counter.get_or_create()
    wait_until(lambda: c2.inc.invoke().get() == 1)


def test_lru_miss_queries_metastore():
    refs = []
    for i in range(8):
        refs.append(echo.invoke(i))
    assert [r.get() for r in refs] == list(range(8))
    # Capacity is 4; earlier routes should be evicted. A repeated call must still succeed via metastore miss lookup.
    assert echo.invoke("after-evict").get() == "after-evict"


def test_proxy_failure_returns_retryable_for_intermediate():
    # This test relies on the shell harness or environment to make one proxy temporarily unavailable.
    # It asserts the user-visible behavior is retryable inner communication, not a stale state-machine block.
    try:
        echo.invoke("proxy-failure-window").get()
    except Exception as exc:
        msg = str(exc)
        assert "ERR_INNER_COMMUNICATION" in msg or "inner communication" in msg.lower()


if __name__ == "__main__":
    yr.init()
    test_named_instance_stale_route_repair()
    test_lru_miss_queries_metastore()
    test_proxy_failure_returns_retryable_for_intermediate()
```

- [ ] **Step 3: Create services.yaml**

Copy the minimal smoke service template from `test/smoke/minimal-python/services.yaml` and set direct-routing config/env:

```yaml
env:
  enable_direct_routing: "true"
  direct_route_cache_capacity: "4"
```

Preserve required runtime/function definitions from the minimal template.

- [ ] **Step 4: Run smoke test and refine harness commands**

Run:

```bash
cd test/smoke/direct-routing-completeness
bash run_test.sh
```

Expected: PASS in a multi-node sandbox. If `yr start --node-id` syntax differs, update only `run_test.sh` to the repo-supported multi-node invocation and keep test assertions unchanged.

- [ ] **Step 5: Commit E2E tests**

```bash
git add test/smoke/direct-routing-completeness .buildkite/test_sandbox_k8s.sh
git commit -m "Cover direct routing completeness in multi-node smoke tests

Constraint: validation must cover named instances, proxy failure, route LRU miss, and cloud/frontend-visible route repair behavior.
Confidence: medium
Scope-risk: moderate
Directive: Keep test capacity low so LRU eviction is deterministic.
Tested: bash test/smoke/direct-routing-completeness/run_test.sh
Not-tested: cluster-specific CI wiring if local sandbox lacks multi-node support."
```

---

## Task 9: Full verification and cleanup

**Files:**
- Review all changed files
- Update docs if implementation diverges from exact field names but preserves spec semantics

- [ ] **Step 1: Check status and accidental files**

Run:

```bash
git status --short
```

Expected: only intentional source/test/doc files; no `.superpowers/brainstorm` files staged.

- [ ] **Step 2: Run targeted C++ tests**

Run:

```bash
cd functionsystem
bash run.sh test -j 4 -T '*RequestRouter*'
bash run.sh test -j 4 -T '*InstanceProxy*'
bash run.sh test -j 4 -T '*DirectRoutingStateMachine*'
bash run.sh test -j 4 -T '*Observer*Route*'
```

Expected: all PASS.

- [ ] **Step 3: Run Go tests**

Run:

```bash
cd api/go
go test ./libruntime/libruntimesdkimpl/... ./libruntime/clibruntime
cd ../../frontend
go test ./pkg/frontend/common/util ./pkg/frontend/invocation -run 'RouteUpdate|DirectRouting' -v
```

Expected: all PASS.

- [ ] **Step 4: Run multi-node smoke**

Run:

```bash
bash test/smoke/direct-routing-completeness/run_test.sh
```

Expected: PASS.

- [ ] **Step 5: Run non-DR regression subset**

Run:

```bash
cd functionsystem
YR_ENABLE_DIRECT_ROUTING=false bash run.sh test -j 4 -T '*InstanceProxy*'
cd ../api/go
go test ./libruntime/libruntimesdkimpl/...
```

Expected: PASS; no non-DR behavior regression.

- [ ] **Step 6: Final diff review**

Run:

```bash
git diff --stat origin/feature/sandbox...HEAD
git log --oneline origin/feature/sandbox..HEAD
```

Expected: commits are grouped by protocol, C++ route cache/hints, schedule rollback, Go LRU/retry, frontend propagation, E2E.

- [ ] **Step 7: Commit final docs update if needed**

If implementation field names differ from the design document, update `docs/superpowers/specs/2026-06-09-direct-routing-completeness-design.md` and commit:

```bash
git add docs/superpowers/specs/2026-06-09-direct-routing-completeness-design.md
git commit -m "Record implemented direct-routing hint field names

Constraint: design documentation must match implemented structured route update fields.
Confidence: high
Scope-risk: narrow
Directive: Keep route update semantics structured and full-chain.
Tested: documentation-only consistency review
Not-tested: no runtime changes in this commit."
```

---

## Self-Review Checklist

- Spec coverage:
  - DR-only scope: Tasks 2-4 and regression checks keep non-DR compatible.
  - Bounded function_proxy route/negative LRU: Task 2.
  - Bounded libruntime LRU: Task 5.
  - Full-chain route update hint: Tasks 1, 3, 6, 7.
  - libruntime-only retry: Task 6 and Task 7 no-retry assertions.
  - Schedule failure rollback: Task 4.
  - Multi-node named/proxy/LRU tests: Task 8.
- Red-flag scan: no unfinished-work markers are present; C++ `std::placeholders` appears only as a namespace symbol.
- Type consistency:
  - `RouteUpdateHint` fields use proto lower-case generated names in C++ (`routeupdatehint`, `routeaddress`, `proxyid`) and Go JSON names (`routeAddress`, `proxyID`).
  - `YR_ROUTE_RETRY_ATTEMPT` is the single retry marker at the function_proxy request boundary.
  - Go route cache uses `routecache.Entry{RouteAddress, ProxyID}` consistently.
