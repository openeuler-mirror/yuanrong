# Direct Routing 完备性设计

日期：2026-06-09
分支基线：`origin/feature/sandbox` (`58e1a635`)
工作区：`.worktrees/direct-routing-completeness`
范围：仅覆盖 `enable_direct_routing=true`；非 Direct Routing 旧路径保持兼容。

## 1. 背景与目标

Direct Routing 当前已具备 route 透传、部分 LRU/查询和调度失败清理逻辑，但仍存在三类完备性缺口：

1. **远端实例状态残留**：DR 模式下，本地不应长期持有非本节点实例的完整 state machine。远端可路由信息应收敛为有界 route/negative cache。
2. **调度失败残留**：只要调度发生在某节点，就可能短暂创建本地 state machine；调度失败后应立即回滚/删除，避免影响后续同 instanceID 或具名实例调度。
3. **过期路由反向更新**：实例删除、迁移或 owner 变化后，请求可能仍按旧 route 到达前 owner。旧 owner 需要把最新路由信息反向传回调用链，而不是直接失败或继续链式代理。

本设计目标：

- DR 模式下只有真实 owner 节点保留本地实例 state machine。
- route cache 全部有界，`function_proxy` 与 `libruntime` 都使用 LRU。
- route miss 后支持按需查询 metastore；只有查询不到或实例不可用时才返回失败。
- stale route 能通过 route update hint 覆盖 `云外客户端 -> frontend -> libruntime -> function_proxy` 全链路。
- 自动重试只由 `libruntime` 执行，且同一请求最多一次。
- 端到端测试必须覆盖多节点、具名实例、proxy 故障和上述功能点。

## 2. 当前代码上下文

本设计基于最新 `feature/sandbox` worktree 探测结果：

- `functionsystem` 当前代码路径为 `functionsystem/functionsystem/src/...`，不是旧文档中的 `functionsystem/src/...`。
- C++ Direct Routing 相关位置：
  - `functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/instance_proxy.*`
  - `functionsystem/functionsystem/src/function_proxy/busproxy/instance_proxy/request_router.*`
  - `functionsystem/functionsystem/src/function_proxy/common/observer/observer_actor.*`
  - `functionsystem/functionsystem/src/function_proxy/common/state_machine/instance_control_view.*`
  - `functionsystem/functionsystem/src/function_proxy/common/state_machine/instance_state_machine.*`
  - `functionsystem/functionsystem/src/function_proxy/local_scheduler/instance_control/instance_ctrl_actor.*`
- Go route cache 当前主要在 `api/go/libruntime/libruntimesdkimpl/libruntimesdkimpl.go`，已有 `sync.Map` 风格 route cache，需要改为 LRU。
- Go/frontend route 透传相关位置：
  - `api/go/yr/cluster_mode_runtime.go`
  - `api/go/libruntime/...`
  - `frontend/pkg/frontend/common/util/client.go`
  - `frontend/pkg/frontend/invocation/function_invoke_for_kernel.go`
  - `go/pkg/functionscaler/faasscheduler.go`

## 3. 设计原则

1. **DR-only 改造**：只改变 `enable_direct_routing=true` 的行为。非 DR watch/dispatcher 语义不主动重构。
2. **owner 才有真实 state machine**：远端 route 查询不得创建长期本地 remote state machine。
3. **中间态不缓存**：metastore 查询得到 `SCHEDULING`、`CREATING`、`EXITING`、`EVICTING`、`SUSPEND` 等中间态时，不写 LRU，不订阅，不创建 state machine，返回可重试 inner communication 错误。
4. **RUNNING 才缓存 route**：远端 `RUNNING` 表示可路由，写 route LRU。
5. **终态/不存在可缓存 negative**：终态或不存在结果可进入 negative LRU，避免重复查询 metastore。
6. **旧 owner 不链式代理**：旧 owner 只查询最新 route 并返回 update hint，不继续代理到新 owner。
7. **重试只在 libruntime 层**：frontend 与云外客户端只传播/消费 route update 信息，不增加自己的自动重试循环。

## 4. 总体架构

### 4.1 function_proxy

- 本地 owner 实例：保留真实 `InstanceStateMachine`。
- 远端 RUNNING 实例：只缓存 route 信息，不创建长期 remote state machine。
- 远端中间态：返回 retryable `ERR_INNER_COMMUNICATION`，不缓存。
- 远端终态/不存在：返回明确失败；可写 negative LRU。
- stale route：旧 owner 检测本地 instance actor 不存在或自己不再是 owner 后，查询 metastore 并返回 route update hint。
- 调度失败：立即删除 request future、回滚/删除本地 state machine、释放 owner 标记。

### 4.2 libruntime

- 将本地 route cache 从无界 `sync.Map` 替换为线程安全 LRU。
- route cache value 至少包含 `routeAddress` 和 `proxyID`。
- create/lease/notifyresult 成功返回 route 时写入 LRU。
- invoke/kill 读取 LRU 并携带 route。
- 收到 route update hint 后：更新 LRU，并且同一请求最多自动重试一次。
- 第二次失败或第二次收到 hint 时停止重试并返回上游。

### 4.3 frontend 与云外客户端

- frontend 透传调用方 route 或从 lease/allocation 获取 route。
- frontend 识别下游 route update hint，并把它转换/透传给云外客户端。
- frontend 不执行自动重试。
- 云外客户端可更新自身缓存；不要求自动重试。

## 5. 数据结构与协议语义

### 5.1 Route LRU entry

`function_proxy` route/negative LRU：

```text
key: instanceID
value:
  RouteEntry {
    kind: RUNNING
    routeAddress: string
    proxyID: string
    modRevision/version: optional
  }
  NegativeEntry {
    kind: TERMINAL_OR_NOT_FOUND
    statusCode: error code
    reason: string
    modRevision/version: optional
  }
```

`libruntime` route LRU：

```text
key: instanceID
value: {
  routeAddress: string
  proxyID: string
}
```

容量必须可配置，默认 `1024`；实现不得保持无界 map。

### 5.2 Route update hint

统一 route update hint 至少包含：

- `instanceID`
- `routeAddress`
- `proxyID`
- `retryable=true`
- `reason=stale_route`
- 可选 `modRevision` 或 `version`

传递形式必须是显式结构化字段或结构化错误扩展，不能依赖字符串解析。实现计划阶段需基于现有 `runtime_rpc::StreamingMessage`、call response、frontend HTTP 响应和 SDK 错误模型选定具体字段位置，但语义必须在每个边界保持一致。

### 5.3 错误语义

| 场景 | 行为 |
| --- | --- |
| route 过期且查到新 RUNNING owner | 返回 route update hint |
| metastore 查到远端中间态 | 返回 retryable `ERR_INNER_COMMUNICATION`；不缓存 |
| metastore 查到终态 | 返回明确失败；可写 negative LRU |
| metastore 查不到 | 返回 not found；可写 negative LRU |
| 调度失败 | 返回调度失败响应；删除本地 state machine |

## 6. 关键流程

### 6.1 DR invoke/call 路由

1. 调用方携带已有 `YR_ROUTE`，或由 libruntime/frontend 从 LRU/lease 填充 route。
2. function_proxy 按 route 转发。
3. 转发成功则正常返回。
4. 转发失败则删除本地 route LRU entry，并查询 metastore。
5. 查询结果：
   - RUNNING：返回 route update hint，或在 libruntime 本地路径更新 LRU 后重试一次。
   - 中间态：返回 retryable `ERR_INNER_COMMUNICATION`。
   - 终态/不存在：返回失败，写 negative LRU。

### 6.2 旧 owner 收到 stale route 请求

1. 请求按旧 route 到达旧 owner。
2. 旧 owner 找不到本地 instance actor，或发现本地 owner 与请求目标不匹配。
3. 旧 owner 查询 metastore。
4. 查询结果：
   - 新 owner RUNNING：返回 route update hint。
   - 中间态：返回 retryable `ERR_INNER_COMMUNICATION`。
   - 终态/不存在：返回明确失败。
5. 旧 owner 不链式代理到新 owner。

### 6.3 libruntime 自动重试

1. libruntime 发起 invoke/kill，读取 route LRU。
2. 收到 route update hint：
   - 更新 route LRU；
   - 如果当前请求未重试过，则带新 route 重试一次；
   - 如果当前请求已重试过，则返回错误。
3. 收到 retryable inner communication：不更新 LRU，返回上游。
4. frontend 和云外客户端不自动重试。

### 6.4 调度失败回滚

1. DR 模式下，本节点调度开始后仍可能短暂创建 state machine。
2. 如果调度失败：
   - 删除 `createRequestFuture_` / runtime promise 关联；
   - 删除或回滚 `InstanceControlView` 中对应 state machine；
   - 释放 owner 标记；
   - 记录日志与 metrics；
   - 通过调度响应返回失败原因。
3. 后续同 instanceID 或具名实例调度不得被旧失败 state machine 拦截。

### 6.5 metastore miss 查询

1. route LRU miss 后查询 metastore route key。
2. RUNNING：写 route LRU，返回 route。
3. 中间态：不写 LRU，返回 retryable `ERR_INNER_COMMUNICATION`。
4. 终态/不存在：写 negative LRU，返回失败。

## 7. 测试设计

### 7.1 function_proxy 单元测试

- `QueryInstanceRoute`：
  - RUNNING 返回 route。
  - 中间态返回 retryable `ERR_INNER_COMMUNICATION`。
  - 终态/不存在返回失败或 negative entry。
  - DR 模式下不创建远端 state machine。
- `RequestRouter` / `InstanceProxy` stale route：
  - instance actor 不存在时查询 metastore，而不是直接永久失败。
  - 查询到新 RUNNING owner 返回 route update hint。
  - 查询到中间态返回 retryable inner communication。
  - 查询不到返回 not found。
- 调度失败回滚：
  - 调度失败后 `InstanceControlView::GetInstance(instanceID)` 为空。
  - request future 被清理。
  - 后续同 named instance 调度不被旧状态拦截。

### 7.2 Go / libruntime 单元测试

- route cache LRU：
  - 超容量淘汰最久未使用 entry。
  - Get 提升热度。
  - 并发读写安全。
- route update hint：
  - 更新 LRU。
  - 同一请求自动重试一次。
  - 第二次失败不继续重试。
- invoke 与 kill 共用相同 route LRU 语义。

### 7.3 frontend 单元测试

- 下游 route update hint 可转换/透传给云外客户端。
- frontend 不自动重试。
- lease/allocation route 仍写入 `YR_ROUTE`。

### 7.4 多节点端到端测试

端到端必须模拟多节点，可通过多次 `yr start` 或等价 sandbox 多节点方式。

覆盖：

1. **具名实例**
   - 创建具名实例并触发 route 缓存。
   - 删除、重建或迁移具名实例。
   - 旧 route 调用触发 route update hint。
   - libruntime 更新 LRU 并重试一次后成功，或明确返回失败。
   - 调度失败不残留 state machine，不影响后续同名调度。

2. **proxy 故障**
   - 多节点启动。
   - 让旧 owner/proxy 不可用或 route 过期。
   - 验证不依赖旧 owner 链式代理。
   - 验证 LRU 淘汰/更新后能查询最新 route。
   - metastore 中间态返回 retryable inner communication。

3. **LRU 行为**
   - 构造超过容量的不同 instance route。
   - 验证旧 entry 被淘汰。
   - 被淘汰后 miss 会查询 metastore。
   - 查询不到才返回失败。

4. **云外客户端 + frontend 全链路**
   - 云外客户端通过 frontend 调用。
   - 下游产生 route update hint。
   - frontend 返回/转换 hint 给云外客户端。
   - frontend 不自动重试。
   - libruntime 场景确认重试只发生在 libruntime 层。

### 7.5 回归验证

- function_proxy 相关单测。
- Go libruntime 单测。
- frontend invocation / lease 单测。
- 多节点 smoke/e2e。
- 非 DR 模式关键回归，确认旧语义未被主动改变。

## 8. 非目标

- 不重构 `enable_direct_routing=false` 的旧全量 watch/dispatcher 行为。
- 不让旧 owner 链式代理到新 owner。
- 不缓存远端中间态。
- 不要求 frontend 或云外客户端实现自动重试循环。
- 不用失败 state machine 作为调度失败诊断的主要载体。

## 9. 实施阶段需细化的决策

以下问题不改变需求语义，但需在 implementation plan 中基于代码细节定稿：

1. route update hint 具体落在哪个 proto/响应扩展字段，需兼容 call response、frontend HTTP 和 SDK 错误模型。
2. Go LRU 是新增轻量实现，还是复用已有项目内工具；不得引入新外部依赖，除非另行批准。
3. function_proxy negative LRU 必须落地；终态/不存在结果需要有界缓存，避免反复查询 metastore。
4. 多节点 `yr start` E2E 的具体脚本入口与环境变量，需要在实施前通过当前 sandbox 工具链确认。
