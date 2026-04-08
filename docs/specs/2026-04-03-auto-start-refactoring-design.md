# openYuanrong `yr.init()` 自动启动后台服务 — 设计文档

> **状态**: ✅ 已实现（基于 master 分支）  
> **作者**: Copilot + lzc  
> **日期**: 2026-04-03（设计）/ 2026-04-08（实现）  
> **分支**: `feature/auto-start-v2`

---

## 实现概要

本设计已在 `feature/auto-start-v2` 分支上实现。核心变更：

| 文件 | 类型 | 说明 |
|------|------|------|
| `yr/process_utils.py` | 新增 | 进程工具：fate-sharing, PID/端口检查 |
| `yr/cluster_launcher.py` | 新增 | 共享启动引擎 |
| `yr/service_manager.py` | 新增 | yr.init() 适配层：检测/启动/清理 |
| `yr/apis.py` | 修改 | init() 中集成 ServiceManager 自动启动 |
| `yr/cli/component/base.py` | 修改 | 添加 preexec_fn 参数，修复 FD 泄露 |

**检测逻辑**（多源检测，防止重复启动）：
1. `session.json` — API 自动启动写入的格式
2. `master.info` / `yr_current_master_info` — CLI (`yr start --master`) 写入的格式
3. 端口探测确认集群存活

**E2E 验证结果**：
- ✅ Layer 1: `yr start --master && yr status && yr stop`（回归通过）
- ✅ Layer 2: `yr.init()` 自动启动并执行 invoke（通过）
- ✅ Layer 2b: 预启动集群时 `yr.init()` 正确复用（通过）
- ✅ 60 个单元/集成测试全部通过

---

## 目录

1. [问题陈述](#1-问题陈述)
2. [现有实现深度分析](#2-现有实现深度分析)
3. [方案评估与选择](#3-方案评估与选择)
4. [详细设计：共享 ClusterLauncher 方案](#4-详细设计共享-clusterlauncher-方案)
5. [代码复用分析](#5-代码复用分析)
6. [稳定性修复清单](#6-稳定性修复清单)
7. [兼容性保证矩阵](#7-兼容性保证矩阵)
8. [实施计划](#8-实施计划)

---

## 1. 问题陈述

### 1.1 现状

用户使用 openYuanrong 需要两步：

```bash
# 步骤 1：终端启动后台服务集群（运维操作）
pip install yuanrong
yr start --master

# 步骤 2：Python 脚本调用（开发操作）
python my_script.py
```

```python
# my_script.py
import yr
yr.init()

@yr.invoke
def hello(msg):
    return f"Hello, {msg}!"

result = hello.invoke("world")
```

**痛点**：步骤 1 是额外的运维负担。新用户容易遗漏，且 CLI 启动与 Python API 使用体验割裂。

### 1.2 目标

用户 `pip install yuanrong` 后，直接在 Python 脚本中调用即可：

```python
import yr
yr.init()  # ← 自动检测并启动后台服务

@yr.invoke
def hello(msg):
    return f"Hello, {msg}!"

result = hello.invoke("world")
# 脚本退出 → 后台服务自动清理
```

后台服务（etcd、ds_master、ds_worker、function_master、function_proxy、function_agent）**自动启动**，脚本退出时**自动清理**。

### 1.3 用户约束（已确认）

| 约束 | 决策 |
|------|------|
| **范围** | 单机本地自动启动（不涉及远程集群发现） |
| **触发** | `yr.init()` 无参数调用即触发 |
| **生命周期** | 进程退出时自动清理后台服务 |
| **多语言** | Python 层实现，C++/Java 通过 subprocess 调 CLI |
| **CLI 关系** | 共享引擎，零冗余 |

---

## 2. 现有实现深度分析

### 2.1 C++ 层自动启动链路（auto_init.cpp）

**关键发现**：`yr.init()` 已有一条完整的自动启动链路，实现在 C++ 的 `auto_init.cpp` 中，通过 Python Cython 绑定 `fnruntime.pyx` 的 `auto_get_cluster_access_info()` 调用。

完整调用链：

```
[Python]  yr.init(conf)
              │
              ▼
[Python]  _auto_get_cluster_access_info(conf)        ← apis.py:143
              │
              │  传入 {serverAddr, dsAddr, inCluster}
              ▼
[Cython]  auto_get_cluster_access_info(info, args)   ← fnruntime.pyx:2957
              │
              ▼
[C++]     AutoGetClusterAccessInfo(info, args)        ← auto_init.cpp:348
              │
              ├─ NeedToBeParsed? (serverAddr 为空或带协议前缀)
              │     No → 直接返回，连接用户指定地址
              │     Yes ↓
              │
              ├─ info.AutoParse()                     ← auto_init.cpp:39
              │     ├─ Step 1: ParseFromEnv()
              │     │    读 YR_SERVER_ADDRESS / YR_DS_ADDRESS 环境变量
              │     ├─ Step 2: ParseServerAddrProtocol()
              │     │    http:// → 外部集群, grpc:// → 集群内部
              │     ├─ Step 3: ParseDsAddr()
              │     │    推断 ds_address
              │     └─ 如果 serverAddr && dsAddr 都有 → 返回（连接已有集群）
              │
              ├─ ParseFromMasterInfo()                ← auto_init.cpp:54
              │     读取 /tmp/yr_sessions/yr_current_master_info 文件
              │     （yr start --master 成功后写入此文件）
              │     解析出 server_addr, ds_addr 等
              │     如果成功 → 返回（连接已有集群）
              │
              └─ AutoCreateYuanRongCluster(args)      ← auto_init.cpp:290
                    ├─ 检查 PATH 中是否有 `yr` 命令
                    ├─ fork() 子进程
                    │     子进程: prctl(PR_SET_PDEATHSIG, SIGTERM)
                    │     子进程: execvp("yr", ["yr", "start", "--master", "--block", "true"])
                    ├─ 父进程轮询等待 yr_current_master_info 文件出现
                    │     每 100ms 检查一次, 最多 100 次 (= 10s 超时)
                    └─ 文件出现后, ParseFromMasterInfo() 获取地址
```

### 2.2 "连接"还是"启动"的判断逻辑

C++ 层采用**瀑布式逐级检测**，每一级都是"尝试获取服务地址，获取不到就尝试下一种方式"：

```
    用户传了 server_address？ ──Yes──→ 直接连接（用户明确指定）
              │ No
              ▼
    环境变量 YR_SERVER_ADDRESS？ ──Yes──→ 直接连接（运维预设）
              │ No
              ▼
    文件 yr_current_master_info 存在？ ──Yes──→ 解析地址，连接（已有集群）
              │ No
              ▼
    fork + execvp("yr start --master") ──→ 等文件出现 ──→ 连接（自动启动新集群）
```

这个设计思路本身是正确的，但**实现层面有严重的稳定性问题**（见 2.4 节）。

### 2.3 SystemLauncher 现有架构（1160 行）

`SystemLauncher` 是 CLI `yr start` 命令的核心实现，承担了以下职责：

```
SystemLauncher (api/python/yr/cli/system_launcher.py, 1160 行)
├── 配置层：ConfigResolver (Jinja2 模板 + TOML 解析 + config.toml)
├── 编排层：load_components() + _get_start_order() (拓扑排序)
├── 进程层：_start_component() → ComponentLauncher.launch() → Popen
├── 健康层：ComponentLauncher.wait_until_healthy() (基类 + 各组件覆写)
├── 守护层：_daemonize() (double-fork) + _monitor_loop() (重启)
├── 会话层：SessionManager (session.json 读写)
├── CLI 交互层：_wait_for_daemon_ready() + _print_join_info()
└── TLS/状态层：TLS 上下文构建 + 集群状态查询
```

**组件依赖图**（定义在 `registry.py:DEPENDS_ON_OVERRIDES_BY_MODE`）：

```
Master 模式:
  etcd → ds_master, ds_worker → function_master, function_proxy → function_agent

Agent 模式:
  ds_worker → function_proxy → function_agent
```

**组件启动流程**：

```
ConfigResolver.render() → 渲染 config.toml.jinja + values.toml + services.yaml
    → 得到各组件的配置（命令行参数、端口、日志路径）
    → _get_start_order() 拓扑排序
    → 按序执行 _start_component():
        for name in order:
            launcher = LAUNCHER_CLASSES[name](config)
            process = launcher.launch()        # subprocess.Popen
            launcher.wait_until_healthy()       # 组件特有健康检查
            session_manager.update(name, pid)   # 记录到 session.json
```

### 2.4 已发现的稳定性问题（7 个）

| # | 问题 | 位置 | 严重度 | 详细分析 |
|---|------|------|--------|----------|
| 1 | **Fate-sharing 失效** | auto_init.cpp:243-256 | 🔴 致命 | fork 的子进程执行 `yr start --master`，而 yr start 内部 `_daemonize()` 做 double-fork。实际服务运行在孙子进程（daemon）中，`PR_SET_PDEATHSIG` 只对直接子进程有效，daemon 脱离进程树不受影响。**Python 退出后服务变成孤儿进程** |
| 2 | **等待机制粗糙** | auto_init.cpp:265-273 | 🟡 高 | 只等 `yr_current_master_info` 文件出现（100×100ms=10s），不验证服务是否真正可用。集群启动慢时直接超时失败 |
| 3 | **waitpid WNOHANG** | auto_init.cpp:274-278 | 🟡 高 | 非阻塞 waitpid，子进程（中间进程）退出状态可能还未就绪就检查了 |
| 4 | **无运行时健康检查** | auto_init.cpp:303-306 | 🟡 高 | 只检查 serverAddr 非空，不验证端口可达或服务就绪 |
| 5 | **无限重启无退避** | system_launcher.py:894-935 | 🟡 高 | `_monitor_loop` 无限重启崩溃组件，无退避策略，可能导致快速失败循环 |
| 6 | **FD 泄漏** | component/base.py:162-163, 233-234 | 🟡 高 | `launch()` 和 `restart()` 打开日志文件但不关闭，长期运行导致 FD 耗尽 |
| 7 | **双重 stop_all 竞争** | system_launcher.py signal+monitor | 🟢 中 | 信号处理器和 monitor 线程都可能调用 `stop_all()`，无锁保护，可能并发冲突 |

#### 问题 1 详解：Fate-sharing 失效的根因

```
Python 脚本 (PID 100)
    │
    ├─ fork()  → 中间子进程 (PID 101)
    │              prctl(PR_SET_PDEATHSIG, SIGTERM)  ← 只对 PID 101 有效
    │              execvp("yr start --master --block true")
    │              │
    │              ├─ _daemonize() 第一次 fork → PID 102
    │              │                                第二次 fork → PID 103 (daemon)
    │              │                                PID 102 立即退出
    │              │
    │              └─ 中间进程退出（daemon 化完成）
    │
    └─ 父进程等待 master_info 文件...

结果：
  - PID 100 (Python) 退出
  - PID 101 (中间进程) 收到 SIGTERM → 但它可能已经退出了
  - PID 103 (daemon/实际服务) → 不在进程树中，完全不受影响 → 成为孤儿进程
```

### 2.5 关键文件路径

| 文件 | 路径 | 用途 |
|------|------|------|
| Session 目录 | `/tmp/yr_sessions/` | 所有会话数据 |
| Latest 符号链接 | `/tmp/yr_sessions/latest/` | 指向最新会话 |
| Session JSON | `/tmp/yr_sessions/latest/session.json` | 组件 PID、端口、状态 |
| Master 信息 | `/tmp/yr_sessions/yr_current_master_info` | C++ auto_init 读取的文件 |
| 默认配置 | `/etc/yuanrong/config.toml` | CLI 全局配置 |
| CLI 模板 | `yr/cli/config.toml.jinja`, `values.toml`, `services.yaml` | 配置渲染模板 |

---

## 3. 方案评估与选择

### 5.1 备选方案

| 方案 | 描述 | 优点 | 缺点 |
|------|------|------|------|
| **A. 修补 SystemLauncher** | 在 SystemLauncher 加参数控制 daemon/非 daemon | 改动最小 | SystemLauncher 已 1160 行，继续加重；API 特有逻辑混入 CLI |
| **B. 独立 ServiceManager** | 新建独立模块重新实现启动逻辑 | 模块清晰 | 与 SystemLauncher **大量代码重复**（拓扑排序、组件启动、健康检查） |
| **C. 抽取 ClusterLauncher** | 抽取共享引擎，CLI 和 API 复用同一套启动逻辑 | 零重复，架构清晰，可维护 | 需要重构 SystemLauncher（风险可控） |

### 5.2 选择方案 C

**方案 C：抽取共享 ClusterLauncher** 是最优选择。理由：

1. **共享引擎 + 调用者适配**：业界成熟模式，已被大规模验证
2. **零冗余开发**：拓扑排序、组件启动、健康检查等核心逻辑只有一份
3. **C++/Java 无需改动**：它们通过 subprocess 调 CLI，CLI 底层用的就是共享引擎
4. **SystemLauncher 瘦身**：从 1160 行降到 ~400 行，只保留 CLI 专有逻辑
5. **风险可控**：共享引擎的代码是从 SystemLauncher 搬出来的，不是重写

---

## 6. 详细设计：共享 ClusterLauncher 方案

### 6.1 目标架构

```
[yr.init() Python API]          [yr start CLI]              [C++/Java API]
        │                           │                           │
        ▼                           │                     fork+exec yr start
   ServiceManager                   │                      (保持现有方式)
   (API 适配器)                     │                           │
   · 服务检测                       │                           │
   · fate-sharing                   │                           │
   · atexit 清理                    │                           │
        │                           │                           │
        ▼                           ▼                           ▼
  ┌─────────────────────────────────────────────────────────────────┐
  │                    ClusterLauncher (共享引擎)                    │
  │  从 SystemLauncher 抽取，CLI 和 API 共用                        │
  │                                                                 │
  │  · 组件注册 & 依赖拓扑排序  (原 _get_start_order)              │
  │  · 组件启动 Popen           (原 _start_component)              │
  │  · 健康检查等待             (原 wait_until_healthy)            │
  │  · 组件停止                 (原 stop_all)                      │
  │  · 监控循环                 (原 _monitor_loop)                 │
  │  · session.json 管理        (原 SessionManager)                │
  └─────────────────────────────────────────────────────────────────┘
                  ▲                          ▲
                  │                          │
     ┌────────────┴────────────┐  ┌──────────┴──────────────┐
     │  yr.init() 模式参数      │  │  yr start CLI 模式参数   │
     │  daemonize=False         │  │  daemonize=True          │
     │  fate_sharing=True       │  │  fate_sharing=False      │
     │  config: 内置默认        │  │  config: config.toml     │
     │  shutdown_at_exit=True   │  │  shutdown_at_exit=block  │
     └─────────────────────────┘  └─────────────────────────┘

  复用现有模块（不改动）：
  ┌─────────────────────────────────────────────────────────────────┐
  │  cli/component/base.py     ComponentLauncher  (启动/健康检查)    │
  │  cli/component/etcd.py     ETCDLauncher       (etcd 特有逻辑)   │
  │  cli/component/registry.py LAUNCHER_CLASSES   (组件注册表)       │
  │  cli/component/...         各组件 Launcher                      │
  └─────────────────────────────────────────────────────────────────┘
```

### 6.2 模块拆分

#### 现有 SystemLauncher → 拆分为三部分

```
SystemLauncher (1160 行)
│
├─── 抽取到 ClusterLauncher (共享引擎, ~520 行)
│    ├── load_components()              从 rendered config 加载组件
│    ├── start_all_components()         按拓扑序启动 + 健康检查
│    ├── stop_all()                     按反序停止
│    ├── _start_component()             Popen 启动单个组件
│    ├── _get_start_order()             拓扑排序
│    ├── _start_monitor_daemon()        监控线程
│    ├── _monitor_loop()                健康监控 + 重启
│    └── session 管理 (save/load/clear)
│
├─── 保留在 SystemLauncher (CLI 适配器, ~400 行)
│    ├── _daemonize()                   double-fork daemon 化
│    ├── _run_parent_wait_ready()       父进程等待 session.json
│    ├── _run_daemon_start()            daemon 进程主循环
│    ├── _start_startup_cancel_watcher() Ctrl-C 取消
│    ├── _wait_for_daemon_ready()       超时等待
│    ├── _print_join_info()             打印集群加入命令
│    ├── _print_cluster_summary()       状态打印
│    └── TLS 相关方法
│
└─── 新建 ServiceManager (API 适配器, ~200 行)
     ├── ensure_services()              检测 + 启动 + 返回端点
     ├── _detect_running_cluster()      三层检测
     ├── _read_endpoints_from_session() 从 session.json 读端点
     ├── _make_default_config()         生成内置默认配置
     ├── _register_cleanup()            atexit + signal
     └── shutdown()                     停止所有组件
```

#### 新增/修改文件清单

```
yr/
├── cluster_launcher.py      # 新增：从 SystemLauncher 抽取的共享启动引擎
├── service_manager.py       # 新增：yr.init() 的 API 适配器
├── process_utils.py         # 新增：fate-sharing 等进程工具函数
├── apis.py                  # 修改：集成 ServiceManager (~10 行改动)
├── cli/
│   ├── system_launcher.py   # 修改：瘦身，改为调用 ClusterLauncher
│   └── component/
│       └── base.py          # 修改：支持 preexec_fn + 修复 FD 泄漏
```

### 6.3 ClusterLauncher 共享引擎

```python
# yr/cluster_launcher.py

class ClusterLauncher:
    """CLI 和 Python API 共享的集群启动引擎。
    
    同一个类，不同的参数控制行为差异。
    """
    
    def __init__(
        self,
        resolver: ConfigResolver,
        mode: StartMode,
        *,
        daemonize: bool = False,           # CLI: True, API: False
        fate_sharing: bool = False,        # CLI: False, API: True (Linux)
        shutdown_at_exit: bool = False,    # API: True
        monitor_interval: int = 3,         # 健康监控间隔(秒)
        max_restart_count: int = 3,        # API: 有限重启, CLI: 可以更多
        restart_backoff_base: float = 1.0, # 指数退避基数(秒)
    ):
        self.resolver = resolver
        self.mode = mode
        self.daemonize = daemonize
        self.fate_sharing = fate_sharing
        self.shutdown_at_exit = shutdown_at_exit
        self.monitor_interval = monitor_interval
        self.max_restart_count = max_restart_count
        self.restart_backoff_base = restart_backoff_base
        
        self.components: dict[str, ComponentLauncher] = {}
        self.processes: dict[str, subprocess.Popen] = {}
        self._stopped = False
        self._stop_lock = threading.Lock()  # 修复双重 stop 竞争
        self._monitor_thread = None
        self.session_manager = None

    def load_components(self) -> None:
        """从配置加载并注册组件。复用现有 LAUNCHER_CLASSES 和 registry。"""
        # 直接复用 registry.py 的 LAUNCHER_CLASSES
        # 直接复用 registry.py 的 DEPENDS_ON_OVERRIDES_BY_MODE
        ...

    def start_all(self) -> bool:
        """按拓扑序启动所有组件并等待健康。
        
        Returns:
            True 表示所有组件启动成功，False 表示有组件失败。
        """
        order = self._get_start_order()
        for comp_name in order:
            process = self._start_component(comp_name)
            if not process:
                return False
            if not self.components[comp_name].wait_until_healthy():
                return False
        return True

    def _start_component(self, name: str) -> subprocess.Popen:
        """启动单个组件。fate_sharing=True 时设置 PR_SET_PDEATHSIG。"""
        launcher = self.components[name]
        preexec_fn = set_pdeathsig if self.fate_sharing else None
        process = launcher.launch(preexec_fn=preexec_fn)
        return process

    def stop_all(self, force: bool = False) -> None:
        """线程安全的停止所有组件。用锁防止信号处理与 monitor 并发调用。"""
        with self._stop_lock:
            if self._stopped:
                return
            self._stopped = True
        # 按反向拓扑序停止
        for launcher in reversed(list(self.components.values())):
            launcher.terminate(force=force)

    def _get_start_order(self) -> list[str]:
        """依赖拓扑排序。直接从现有 SystemLauncher 搬来。"""
        ...

    def start_monitor(self) -> None:
        """启动后台健康监控线程。"""
        self._monitor_thread = threading.Thread(
            target=self._monitor_loop, daemon=True
        )
        self._monitor_thread.start()

    def _monitor_loop(self) -> None:
        """健康监控 + 指数退避重启。改进现有的无限重启问题。"""
        restart_counts: dict[str, int] = {}
        while not self._stopped:
            time.sleep(self.monitor_interval)
            for name, launcher in list(self.components.items()):
                process = launcher.component_config.process
                if process is None or process.poll() is None:
                    continue  # 正在运行
                
                count = restart_counts.get(name, 0)
                if count >= self.max_restart_count:
                    logger.error(
                        f"{name}: exceeded max restart count ({self.max_restart_count}), "
                        f"giving up"
                    )
                    continue
                
                # 指数退避: 1s, 2s, 4s, ..., max 30s
                backoff = min(self.restart_backoff_base * (2 ** count), 30.0)
                time.sleep(backoff)
                
                launcher.restart()
                restart_counts[name] = count + 1
```

### 6.4 ServiceManager API 适配器

```python
# yr/service_manager.py

@dataclass
class ServiceEndpoints:
    """服务端点信息。"""
    server_address: str
    ds_address: str
    
class ServiceManager:
    """yr.init() 自动启动后台服务的适配器。
    
    职责：
    1. 检测是否有已运行的集群（三层检测）
    2. 如果没有，通过 ClusterLauncher 启动本地集群
    3. 注册 atexit/signal 清理钩子确保退出时清理
    4. 返回服务端点信息供 yr.init() 使用
    """
    
    _instance: ClassVar[Optional['ServiceManager']] = None
    _lock: ClassVar[threading.Lock] = threading.Lock()
    
    def __init__(self, engine: ClusterLauncher):
        self.engine = engine
    
    @classmethod
    def ensure_services(cls, conf: Config) -> ServiceEndpoints:
        """确保后台服务运行中。线程安全，单例模式。
        
        完整流程：
        1. 已有实例？ → 返回已有端点
        2. 检测已运行集群 → 返回端点
        3. 创建默认配置 → 启动集群 → 返回端点
        """
        with cls._lock:
            if cls._instance:
                return cls._instance._read_endpoints()
            
            # 1. 检测已有集群（三层检测）
            endpoints = cls._detect_running_cluster()
            if endpoints:
                return endpoints
            
            # 2. 准备配置（不依赖 config.toml，使用内置默认值）
            resolver = cls._make_default_resolver(conf)
            
            # 3. 通过共享引擎启动集群
            engine = ClusterLauncher(
                resolver=resolver,
                mode=StartMode.MASTER,
                daemonize=False,           # 关键：不 daemon 化！
                fate_sharing=True,         # 关键：子进程随父进程退出
                shutdown_at_exit=True,
                max_restart_count=3,       # 有限重启
            )
            engine.load_components()
            success = engine.start_all()
            if not success:
                engine.stop_all(force=True)
                raise RuntimeError("Failed to start local YuanRong cluster")
            
            # 4. 启动后台监控
            engine.start_monitor()
            
            # 5. 注册清理 + 缓存实例
            cls._instance = cls(engine)
            cls._instance._register_cleanup()
            
            return cls._instance._read_endpoints()
    
    @staticmethod
    def _detect_running_cluster() -> Optional[ServiceEndpoints]:
        """三层检测已运行的集群。
        
        Layer 1: session.json 文件是否存在
        Layer 2: 关键组件进程是否存活 (os.kill(pid, 0))
        Layer 3: 核心端口是否可达 (TCP connect)
        """
        session_path = Path(SESSION_JSON_PATH)
        if not session_path.exists():
            return None
        
        try:
            session = json.loads(session_path.read_text())
        except (json.JSONDecodeError, IOError):
            return None
        
        components = session.get("components", {})
        
        # Layer 2: 关键组件进程存活检查
        for name in ["function_master", "function_proxy", "ds_worker"]:
            pid = components.get(name, {}).get("pid")
            if not pid or not is_process_alive(pid):
                return None
        
        # Layer 3: function_proxy 端口可达性检查
        cluster_info = session.get("cluster_info", {}).get("for-join", {})
        proxy_port = cluster_info.get("function_proxy.port")
        if proxy_port and not is_port_reachable("127.0.0.1", int(proxy_port)):
            return None
        
        # 所有检查通过，返回端点
        server_addr = cluster_info.get("function_master.server_address", "")
        ds_addr = cluster_info.get("ds_master.ds_address", "")
        return ServiceEndpoints(server_address=server_addr, ds_address=ds_addr)
    
    @staticmethod
    def _make_default_resolver(conf: Config) -> ConfigResolver:
        """生成不依赖外部 config.toml 的默认配置。
        
        使用 yr 包安装目录下的模板 + 默认值 + 自动分配端口。
        """
        cli_dir = Path(__file__).resolve().parent / "cli"
        return ConfigResolver(
            config_path=Path("/dev/null"),  # 无外部配置
            cli_dir=cli_dir,
            mode=StartMode.MASTER,
            render=True,
        )
    
    def _register_cleanup(self):
        """多重保险清理机制：atexit + signal handler。"""
        import atexit, signal
        
        atexit.register(self.shutdown)
        
        # SIGTERM 处理（不覆盖用户已有的 handler）
        prev = signal.getsignal(signal.SIGTERM)
        def _handler(signum, frame):
            self.shutdown()
            if callable(prev) and prev not in (signal.SIG_DFL, signal.SIG_IGN):
                prev(signum, frame)
        signal.signal(signal.SIGTERM, _handler)
    
    def shutdown(self):
        """优雅关闭所有组件。幂等。"""
        if self.engine:
            self.engine.stop_all()
            self.engine = None
    
    def _read_endpoints(self) -> ServiceEndpoints:
        """从 session.json 读取服务端点。"""
        ...
```

### 6.5 yr.init() 集成改动

```python
# apis.py 改动（约 10 行）

def init(conf: Config = None) -> ClientInfo:
    if is_initialized() and ConfigManager().is_driver:
        raise RuntimeError("yr.init cannot be called twice")
    
    conf = Config() if conf is None else conf
    
    # ===== 新增：自动启动后台服务 =====
    # 条件：driver 模式 且 未指定 server_address 且 非 local_mode
    if conf.is_driver and not conf.server_address and not conf.local_mode:
        from yr.service_manager import ServiceManager
        try:
            endpoints = ServiceManager.ensure_services(conf)
            conf.server_address = endpoints.server_address
            conf.ds_address = endpoints.ds_address
        except Exception as e:
            _logger.warning(f"Auto-start failed, falling back to C++ auto_init: {e}")
            # 回退到现有 C++ 逻辑，保持向后兼容
    # ===== 新增结束 =====
    
    conf = _auto_get_cluster_access_info(conf)  # 现有逻辑保留
    ConfigManager().init(conf, is_initialized())
    runtime_holder.init()
    ...
```

**回退策略**：如果 Python ServiceManager 失败，捕获异常后仍然走现有 C++ `auto_init` 路径。保证向后兼容。

### 6.6 ComponentLauncher 小改动

```python
# cli/component/base.py 修改

def launch(self, preexec_fn=None) -> subprocess.Popen:    # 新增 preexec_fn 参数
    ...
    # 修复 FD 泄漏：用 fd 而非 file object
    stdout_fd = os.open(str(log_file), os.O_WRONLY | os.O_CREAT | os.O_APPEND)
    process = subprocess.Popen(
        cmd,
        env=env,
        cwd=cwd,
        stdout=stdout_fd,
        stderr=stdout_fd,
        preexec_fn=preexec_fn,    # 新增：API 模式传入 set_pdeathsig
    )
    os.close(stdout_fd)  # Popen 已 dup fd，安全关闭原始 fd
    ...
```

### 6.7 process_utils.py 工具模块

```python
# yr/process_utils.py

import ctypes
import os
import signal
import socket

def set_pdeathsig():
    """Linux: 父进程退出时子进程收到 SIGTERM。
    
    用作 subprocess.Popen 的 preexec_fn 参数。
    使用 SIGTERM 而非 SIGKILL，更优雅地清理子进程。
    """
    if hasattr(ctypes, 'CDLL'):
        libc = ctypes.CDLL("libc.so.6", use_errno=True)
        PR_SET_PDEATHSIG = 1
        libc.prctl(PR_SET_PDEATHSIG, signal.SIGTERM)

def is_process_alive(pid: int) -> bool:
    """检查进程是否存活。"""
    try:
        os.kill(pid, 0)
        return True
    except (ProcessLookupError, PermissionError):
        return False

def is_port_reachable(host: str, port: int, timeout: float = 1.0) -> bool:
    """检查端口是否可达。"""
    try:
        with socket.create_connection((host, port), timeout=timeout):
            return True
    except (ConnectionRefusedError, TimeoutError, OSError):
        return False
```

### 6.8 C++/Java API 策略

**不需要改动 C++/Java 的自动启动逻辑。**

C++ `auto_init.cpp` 通过 `fork+execvp("yr start --master")` 调用 CLI，这是多语言框架中常见的模式：非 Python 语言通过 subprocess 复用 Python CLI 的启动能力。

**Python ServiceManager 与 C++ 层不冲突**：

```
场景 1：Python ServiceManager 先启动了服务
  → C++ ParseFromMasterInfo() 检测到 yr_current_master_info → 直接连接

场景 2：C++ 先通过 yr start 启动了服务
  → Python _detect_running_cluster() 检测到 session.json → 直接复用

协调点：同一个 session.json / yr_current_master_info 文件
```

**后续可选优化**（不阻塞本次重构）：
- C++ `auto_init.cpp` 等待机制单独改进（增加超时、加健康检查）
- 长期可考虑删除 C++ 层自动启动逻辑，统一由 Python 层处理

---

## 7. 代码复用分析

### 7.1 共享 vs 专有代码量

| 代码 | 行数(估) | 来源 | 使用者 |
|------|----------|------|--------|
| **ClusterLauncher 共享引擎** | **~520** | **从 SystemLauncher 抽取** | **CLI + API** |
| 　组件启动 Popen | ~60 | 原 _start_component | 共享 |
| 　健康检查等待 | ~30 | 原 wait_until_healthy | 共享 |
| 　拓扑排序 | ~40 | 原 _get_start_order | 共享 |
| 　监控循环（含退避改进） | ~60 | 原 _monitor_loop | 共享 |
| 　组件加载 | ~40 | 原 load_components | 共享 |
| 　session.json 管理 | ~80 | 原 SessionManager | 共享 |
| 　停止逻辑 | ~50 | 原 stop_all | 共享 |
| **CLI 专有** | **~400** | **保留在 SystemLauncher** | **仅 CLI** |
| 　double-fork daemon | ~50 | 原 _daemonize | 仅 CLI |
| 　父进程等待 | ~60 | 原 _wait_for_daemon_ready | 仅 CLI |
| 　取消监听 | ~30 | 原 cancel watcher | 仅 CLI |
| 　集群信息打印 | ~100 | 原 _print_join_info 等 | 仅 CLI |
| 　TLS 处理 | ~60 | 原 TLS 方法 | 仅 CLI |
| **API 专有** | **~200** | **新增** | **仅 API** |
| 　ServiceManager | ~120 | 新增 | 仅 API |
| 　process_utils | ~40 | 新增 | 仅 API |
| 　apis.py 改动 | ~15 | 修改 | 仅 API |
| 　ComponentLauncher 扩展 | ~10 | 修改 | 共享 |

**总结**：~520 行共享，~400 行 CLI 专有，~200 行 API 专有。**零冗余开发。**

### 7.2 与 C++/Java 的关系

| 语言 | 自动启动方式 | 实现位置 | 本次改动 |
|------|-------------|---------|---------|
| **Python API** | ServiceManager → ClusterLauncher (直接调用) | Python 层 | 本次重构新增 |
| **Python CLI** | SystemLauncher → ClusterLauncher (直接调用) | Python 层 | 瘦身重构 |
| **C++ API** | fork+exec `yr start --master` (间接调用 CLI) | auto_init.cpp | **无需改动** |
| **Java API** | subprocess `yr start --master` (间接调用 CLI) | RunManager.java | **无需改动** |

---

## 8. 稳定性修复清单

| # | 问题 | 修复方案 | 涉及文件 | 优先级 |
|---|------|---------|---------|--------|
| 1 | **Fate-sharing 失效** | Python 层 ServiceManager 不 daemon 化，子进程用 `PR_SET_PDEATHSIG` | service_manager.py, process_utils.py | P0 |
| 2 | **等待机制粗糙** | Python 层分层健康检查（进程存活 → 端口可达 → 服务 ready）取代 C++ 文件轮询 | service_manager.py | P0 |
| 3 | **无限重启无退避** | ClusterLauncher 的 `_monitor_loop` 加指数退避 + `max_restart_count` | cluster_launcher.py | P1 |
| 4 | **FD 泄漏** | `launch()/restart()` 使用 fd 而非 file object，Popen 后立即 close | component/base.py | P1 |
| 5 | **双重 stop_all 竞争** | ClusterLauncher 用 `threading.Lock` + `_stopped` flag 保护 | cluster_launcher.py | P1 |
| 6 | **session.json 频繁写入** | 仅在组件状态变更时写入（而非定时 3s 一次） | cluster_launcher.py | P2 |
| 7 | **仅启动时健康检查** | `_monitor_loop` 定期做端口/HTTP 健康检查，不仅检查 `process.poll()` | cluster_launcher.py | P2 |

---

## 9. 兼容性保证矩阵

| 场景 | 预期行为 |
|------|---------|
| `yr.init()` 无参数 | ServiceManager 检测 → 无已有集群 → ClusterLauncher 启动 → 连接 |
| `yr.init(server_address="x:y")` | 跳过 ServiceManager，直接连接用户指定地址 |
| 先 `yr start --master`，再 `yr.init()` | ServiceManager 检测到 session.json → 复用已有集群 |
| C++ `auto_init` 先启动了集群 | ServiceManager 检测到 session.json/master_info → 复用 |
| `yr.init()` 启动后执行 `yr health` | session.json 格式兼容，CLI 可正常查看状态 |
| `yr stop` 手动停止 | 可停止任何方式启动的服务（session.json 格式一致） |
| `yr.finalize()` 或脚本正常退出 | atexit 触发 `shutdown()` 优雅停止 |
| 脚本崩溃/被 kill | `PR_SET_PDEATHSIG(SIGTERM)` 兜底杀子进程 |
| Python ServiceManager 失败 | 捕获异常，回退到 C++ `auto_init` 逻辑（向后兼容） |
| 多个 Python 脚本并行 `yr.init()` | 第二个检测到第一个启动的集群 → 复用 |

---

## 10. 实施计划

### Phase 1: 抽取共享引擎

- 新建 `yr/process_utils.py`（fate-sharing, 进程/端口检测工具函数）
- 新建 `yr/cluster_launcher.py`（从 SystemLauncher 抽取共享逻辑）
- 修改 `yr/cli/system_launcher.py`（瘦身，改为调用 ClusterLauncher）
- **验证**：`yr start --master` / `yr stop` / `yr health` 行为不变

### Phase 2: API 适配器

- 新建 `yr/service_manager.py`（服务检测 + 启动 + 清理）
- 修改 `yr/cli/component/base.py`（支持 `preexec_fn` 参数 + 修复 FD 泄漏）
- 修改 `yr/apis.py`（集成 ServiceManager，约 10 行改动）
- **验证**：`yr.init()` 无参数可自动启动服务

### Phase 3: 稳定性修复

- ClusterLauncher: 指数退避重启 + `max_restart_count`
- ClusterLauncher: 线程安全 `stop_all`（Lock + `_stopped` flag）
- ClusterLauncher: 仅状态变更时写 session.json
- `_monitor_loop`: 定期端口/HTTP 健康检查（不仅 `process.poll()`）

### Phase 4: 端到端验证

- `yr.init()` 无参数自动启动
- 脚本退出后服务自动清理（atexit + fate-sharing）
- 已有集群时 `yr.init()` 正确复用
- CLI `yr start/stop/health` 回归测试
- C++ `auto_init` 与 Python ServiceManager 共存测试

---

## 附录 A: 开放问题

| 问题 | 状态 | 备注 |
|------|------|------|
| `ConfigResolver` 能否无 `config.toml` 工作？ | 待验证 | ServiceManager 需要纯内置默认配置 |
| C++ `--block true` 标志是否被 CLI 支持？ | 待验证 | auto_init.cpp 中使用，但 CLI 定义中未见 |
| C++ auto_init 与 Python ServiceManager 过渡期共存策略 | 已设计 | 通过 try/except 回退保持兼容 |
| `PR_SET_PDEATHSIG` 用 SIGTERM 还是 SIGKILL？ | 已决策 | 先用 SIGTERM（更优雅），业界也有用 SIGKILL 的实践 |
