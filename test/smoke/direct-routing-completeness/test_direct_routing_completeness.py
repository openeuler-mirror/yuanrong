#!/usr/bin/env python3
# coding=UTF-8
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

"""Direct Routing completeness smoke tests.

The shell harness starts YuanRong with enable_direct_routing=true and a tiny
route cache. These assertions exercise the user-visible contracts:

* named instances continue to work after route warmup and recreation;
* LRU route-cache misses still succeed by querying cluster metadata;
* stale/proxy-failure behavior is driven by the harness-controlled proxy
  failure injector and fails loudly when injection is unavailable.
"""

from __future__ import annotations

import os
import subprocess
import re
import sys
import time
import traceback
from typing import Callable

import yr
from yr.config import Config

YR_SERVER_ADDRESS = os.environ.get("YR_SERVER_ADDRESS", "127.0.0.1:8888")
ENABLE_PROXY_FAILURE_CHECK = os.environ.get("YR_TEST_PROXY_FAILURE", "false").lower() == "true"
ROUTE_CACHE_CAPACITY = int(os.environ.get("YR_DIRECT_ROUTE_CACHE_CAPACITY", "4"))
PROXY_FAILURE_COMMAND = os.environ.get("YR_PROXY_FAILURE_COMMAND", "")
DEPLOY_PATH = os.environ.get("DEPLOY_PATH", "")

results: list[tuple[str, bool, str]] = []


def record(name: str, passed: bool, detail: str = "") -> None:
    results.append((name, passed, detail))
    tag = "PASS" if passed else "FAIL"
    suffix = f" - {detail}" if detail else ""
    print(f"{name}: {tag}{suffix}", flush=True)


def wait_until(fn: Callable[[], bool], timeout: int = 30, interval: float = 1.0) -> None:
    deadline = time.time() + timeout
    last: BaseException | None = None
    while time.time() < deadline:
        try:
            if fn():
                return
        except BaseException as exc:  # keep last user-visible failure for diagnostics
            last = exc
        time.sleep(interval)
    raise AssertionError(f"condition not met before timeout, last={last!r}")


def init_runtime() -> None:
    conf = Config(server_address=YR_SERVER_ADDRESS, is_driver=True, auto=False)
    conf.in_cluster = False
    yr.init(conf)


@yr.invoke
def echo(value):
    return value


@yr.invoke
def runtime_node_id():
    from yr.apis import get_node_id

    return get_node_id()


@yr.instance
class Counter:
    def __init__(self):
        self.value = 0

    def inc(self):
        self.value += 1
        return self.value

    def get_node_id(self):
        from yr.apis import get_node_id

        return get_node_id()


def test_named_instance_recreate_sync() -> None:
    opt = yr.InvokeOptions(name="dr_named_counter", namespace="dr_smoke")
    counter = Counter.options(opt).invoke()
    try:
        first = yr.get(counter.inc.invoke())
        second = yr.get(counter.inc.invoke())
        if (first, second) != (1, 2):
            raise AssertionError(f"expected warmup increments 1/2, got {first}/{second}")
        first_node = yr.get(counter.get_node_id.invoke())
        counter.terminate(is_sync=True)

        recreated = Counter.options(opt).invoke()
        try:
            wait_until(lambda: yr.get(recreated.inc.invoke()) == 1, timeout=45)
            second_node = yr.get(recreated.get_node_id.invoke())
            record("named_instance_recreate_sync", True, f"nodes={first_node}->{second_node}")
        finally:
            recreated.terminate(is_sync=True)
    finally:
        try:
            counter.terminate(is_sync=True)
        except Exception:
            pass


def test_lru_miss_queries_metastore() -> None:
    count = max(ROUTE_CACHE_CAPACITY + 3, 7)
    counters = []
    try:
        for idx in range(count):
            opt = yr.InvokeOptions(name=f"dr_lru_counter_{idx}", namespace="dr_smoke")
            counter = Counter.options(opt).invoke()
            counters.append(counter)
            got = yr.get(counter.inc.invoke())
            if got != 1:
                raise AssertionError(f"expected first increment for counter {idx}, got {got}")

        # Capacity is intentionally tiny. The first counter's direct route should
        # have been evicted; this second call must recover through route-cache
        # miss + metadata lookup rather than relying on an in-memory hit.
        got = yr.get(counters[0].inc.invoke())
        if got != 2:
            raise AssertionError(f"expected evicted counter to recover and increment to 2, got {got}")
        record("lru_miss_queries_metastore", True, f"capacity={ROUTE_CACHE_CAPACITY}, instances={count}")
    finally:
        for counter in counters:
            try:
                counter.terminate(is_sync=True)
            except Exception:
                pass


def discover_started_nodes() -> set[str]:
    completed = subprocess.run(
        "pgrep -af '[f]unction_proxy'", shell=True, text=True, capture_output=True, check=False
    )
    nodes: set[str] = set()
    for line in completed.stdout.splitlines():
        match = re.search(r"--node_id=([^ ]+)", line)
        if match:
            nodes.add(match.group(1))
    return nodes


def test_multinode_scheduler_surface() -> None:
    require_multi = os.environ.get("YR_REQUIRE_MULTI_NODE", "true").lower() == "true"
    runtime_nodes = {yr.get(runtime_node_id.invoke()) for _ in range(8)}
    started_nodes = discover_started_nodes()
    if require_multi and len(started_nodes) < 2:
        raise AssertionError(
            f"expected at least two started function_proxy nodes, got {sorted(started_nodes)}; "
            f"runtime_nodes={sorted(runtime_nodes)}"
        )
    record(
        "multinode_scheduler_surface",
        True,
        f"started_nodes={sorted(started_nodes)}, runtime_nodes={sorted(runtime_nodes)}",
    )


def run_proxy_failure_injection(node_id: str) -> None:
    if not PROXY_FAILURE_COMMAND:
        raise AssertionError("proxy failure injection unavailable: set YR_PROXY_FAILURE_COMMAND")
    command = PROXY_FAILURE_COMMAND.format(node_id=node_id, deploy_path=DEPLOY_PATH)
    completed = subprocess.run(command, shell=True, text=True, capture_output=True, check=False)
    if completed.returncode != 0:
        raise AssertionError(
            f"proxy failure injection failed rc={completed.returncode}, "
            f"stdout={completed.stdout!r}, stderr={completed.stderr!r}")


def test_named_instance_proxy_failure_retryable_window() -> None:
    if not ENABLE_PROXY_FAILURE_CHECK:
        raise AssertionError("proxy failure coverage disabled: set YR_TEST_PROXY_FAILURE=true")

    opt = yr.InvokeOptions(name="dr_proxy_failure_counter", namespace="dr_smoke")
    counter = Counter.options(opt).invoke()
    first_node = ""
    try:
        if yr.get(counter.inc.invoke()) != 1:
            raise AssertionError("named counter warmup failed")
        first_node = yr.get(counter.get_node_id.invoke())
        run_proxy_failure_injection(first_node)
        time.sleep(1)

        try:
            got = yr.get(counter.inc.invoke())
            if got < 2:
                raise AssertionError(f"expected named stale route recovery increment >=2, got {got}")
            record("named_proxy_failure_retryable_window", True, f"recovered_after_proxy_failure node={first_node}")
        except Exception as exc:
            msg = str(exc)
            if (
                "ERR_INNER_COMMUNICATION" in msg
                or "inner communication" in msg.lower()
                or "3002" in msg
                or "instance route is not available for direct routing" in msg
            ):
                record("named_proxy_failure_retryable_window", True, f"retryable_or_stale_route node={first_node}: {msg}")
                return
            raise
    finally:
        try:
            counter.terminate(is_sync=True)
        except Exception:
            pass


def run() -> int:
    try:
        init_runtime()
        record("runtime_init", True)
    except Exception as exc:
        record("runtime_init", False, str(exc))
        traceback.print_exc()
        return 1

    tests = [
        test_multinode_scheduler_surface,
        test_named_instance_recreate_sync,
        test_lru_miss_queries_metastore,
        test_named_instance_proxy_failure_retryable_window,
    ]
    for test in tests:
        try:
            test()
        except Exception as exc:
            record(test.__name__, False, str(exc))
            traceback.print_exc()

    try:
        yr.finalize()
    except Exception:
        pass

    print("")
    if all(passed for _, passed, _ in results):
        print("--- Direct Routing Completeness Smoke: ALL PASS ---")
        return 0
    print("--- Direct Routing Completeness Smoke: SOME FAILURES ---")
    for name, passed, detail in results:
        if not passed:
            print(f"  {name}: {detail}")
    return 1


if __name__ == "__main__":
    sys.exit(run())
