#!/usr/bin/env python3
# coding=UTF-8
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

"""Shared cluster launch engine used by both CLI (yr start) and Python API (yr.init()).

One engine, different parameters for CLI vs API.
"""

from __future__ import annotations

import json
import logging
import os
import subprocess
import sys
import threading
import time
from collections import deque
from pathlib import Path
from typing import Callable, Optional

from yr.cli.component.base import ComponentConfig, ComponentLauncher
from yr.cli.component.registry import (
    LAUNCHER_CLASSES,
    PREPEND_CHAR_OVERRIDES,
    get_depends_on_overrides,
)
from yr.cli.config import ConfigResolver
from yr.cli.const import (
    DEFAULT_DEPLOY_DIR,
    DEFAULT_LOG_DIR,
    SESSION_JSON_PATH,
    SESSION_LATEST_PATH,
    StartMode,
)
from yr.cli.system_launcher import _atomic_write_json, _parse_addr, write_old_current_master_info

logger = logging.getLogger(__name__)


class ClusterLauncher:
    """Shared cluster launch engine for CLI and Python API.

    Args:
        resolver: ConfigResolver that provides rendered configuration.
        mode: StartMode (MASTER, WORKER, etc.).
        preexec_fn: Optional callable passed to each component's Popen (e.g. set_pdeathsig).
        shutdown_at_exit: If True, stop_all() is called on engine disposal.
        monitor_interval: Seconds between health check iterations.
        max_restart_count: Max restarts per component before giving up (0 = unlimited).
        restart_backoff_base: Base delay in seconds for exponential restart backoff.
    """

    def __init__(
        self,
        resolver: ConfigResolver,
        mode: StartMode,
        *,
        preexec_fn: Optional[Callable] = None,
        shutdown_at_exit: bool = False,
        monitor_interval: int = 3,
        max_restart_count: int = 0,
        restart_backoff_base: float = 1.0,
    ):
        self.resolver = resolver
        self.mode = mode
        self.preexec_fn = preexec_fn
        self.shutdown_at_exit = shutdown_at_exit
        self.monitor_interval = monitor_interval
        self.max_restart_count = max_restart_count
        self.restart_backoff_base = restart_backoff_base

        self.components: dict[str, ComponentLauncher] = {}
        self.processes: dict[str, subprocess.Popen] = {}
        self._stopped: bool = False
        self._stop_lock: threading.Lock = threading.Lock()
        self._monitor_thread: Optional[threading.Thread] = None

        # Component registry
        self.launcher_classes = LAUNCHER_CLASSES
        self.prepend_char_overrides = PREPEND_CHAR_OVERRIDES
        self.depends_on_overrides = get_depends_on_overrides(self.mode)

    # ─── Environment Preparation ────────────────────────────────────────

    def prepare_environment(self) -> None:
        """Create deploy dirs, session symlink, and chdir to deploy path."""
        deploy_path = self.resolver.runtime_context["deploy_path"]
        deploy_path.mkdir(parents=True, exist_ok=True)

        symlink_path = Path(SESSION_LATEST_PATH)
        if symlink_path.is_symlink() or symlink_path.exists():
            symlink_path.unlink()
        symlink_path.symlink_to(deploy_path)
        logger.info(f"Created symlink {symlink_path} -> {deploy_path}")

        Path(DEFAULT_LOG_DIR).mkdir(parents=True, exist_ok=True)
        Path(DEFAULT_DEPLOY_DIR).mkdir(parents=True, exist_ok=True)
        os.chdir(DEFAULT_DEPLOY_DIR)
        self.resolver.runtime_context["deploy_path"] = deploy_path

    # ─── Component Loading ──────────────────────────────────────────────

    def load_components(self) -> None:
        """Load and register components from rendered config."""
        components_config = self.resolver.rendered_config["mode"].get(self.mode.value, {})

        # Check if distributed master is enabled (skip ds_master if so)
        enable_distributed_master = False
        if "ds_worker" in self.resolver.rendered_config:
            enable_distributed_master = self.resolver.rendered_config["ds_worker"]["args"].get(
                "enable_distributed_master", False
            )

        for comp_name, enable in components_config.items():
            if comp_name == "ds_master" and enable_distributed_master:
                logger.info(
                    f"Skipping ds_master since enable_distributed_master is {enable_distributed_master}"
                )
                continue

            if enable:
                launcher_class: Optional[type[ComponentLauncher]] = self.launcher_classes.get(comp_name)
                if launcher_class is None:
                    logger.error(f"Unknown component: {comp_name}")
                    sys.exit(1)

                launcher = launcher_class(comp_name, self.resolver)
                self._apply_component_overrides(comp_name, launcher)
                self.components[comp_name] = launcher

    def _apply_component_overrides(self, comp_name: str, launcher: ComponentLauncher) -> None:
        """Apply per-component configuration tweaks after a launcher is constructed."""
        override_char = self.prepend_char_overrides.get(comp_name)
        if override_char and launcher.component_config.prepend_char == "--":
            launcher.component_config.prepend_char = override_char
        depends_on_override = self.depends_on_overrides.get(comp_name)
        if depends_on_override is not None:
            launcher.component_config.depends_on = depends_on_override

    # ─── Start / Stop ───────────────────────────────────────────────────

    def get_start_order(self) -> list[str]:
        """Topological sort of components based on dependency graph."""
        graph: dict[str, set[str]] = {name: set() for name in self.components}
        in_degree: dict[str, int] = dict.fromkeys(self.components.keys(), 0)

        for name, launcher in self.components.items():
            depends_on = launcher.component_config.depends_on or []
            for dep in depends_on:
                if dep not in self.components:
                    logger.error(f"Component '{name}' depends on unknown component '{dep}'")
                    sys.exit(1)
                if name not in graph[dep]:
                    graph[dep].add(name)
                    in_degree[name] += 1

        queue: deque[str] = deque([name for name, deg in in_degree.items() if deg == 0])
        order: list[str] = []

        while queue:
            cur = queue.popleft()
            order.append(cur)
            for nxt in graph[cur]:
                in_degree[nxt] -= 1
                if in_degree[nxt] == 0:
                    queue.append(nxt)

        if len(order) != len(self.components):
            cycle_nodes = [name for name, deg in in_degree.items() if deg > 0]
            logger.error(f"Detected cyclic or unresolved dependencies among components: {cycle_nodes}")
            sys.exit(1)

        logger.debug(f"Component start order (topological): {order}")
        return order

    def start_component(self, component_name: str) -> Optional[subprocess.Popen]:
        """Start a single component, passing preexec_fn if configured."""
        launcher = self.components[component_name]
        process = launcher.launch(preexec_fn=self.preexec_fn)
        self.processes[component_name] = process
        return process

    def start_components_in_order(self, components_order: list[str]) -> bool:
        """Start components sequentially, waiting for each to become healthy."""
        for comp_name in components_order:
            logger.debug(f"Starting {comp_name}...")
            process = self.start_component(comp_name)
            if not process:
                logger.error(f"Failed to start {comp_name}")
                return False
            if not self.components[comp_name].wait_until_healthy():
                logger.debug(f"{comp_name} failed health check after start.")
                return False
        return True

    def start_all(self) -> bool:
        """Prepare environment, load components, start in order. Returns True on success."""
        logger.info("Starting system components...")
        self.prepare_environment()
        self.load_components()
        order = self.get_start_order()
        success = self.start_components_in_order(order)
        if success:
            self.save_session()
            logger.info("✅ All components started and session saved.")
        return success

    def save_session(self) -> None:
        """Save session.json compatible with SystemLauncher/SessionManager format."""
        config = self.resolver.rendered_config
        session_data = {
            "start_time": time.ctime(),
            "mode": self.mode.value,
            "components": {},
            "cluster_info": {},
        }

        for name, launcher in self.components.items():
            comp = launcher.component_config
            if comp.pid:
                session_data["components"][name] = {
                    "pid": comp.pid,
                    "status": comp.status.value,
                    "start_time": comp.start_time,
                    "cmd": comp.args,
                    "env_vars": comp.env_vars,
                    "restart_count": comp.restart_count,
                }

        mode_config = config["mode"].get(self.mode.value, {})

        etcd_ip = etcd_port = etcd_peer_port = etcd_addresses = None
        if mode_config.get("etcd", False):
            etcd_ip, etcd_port = _parse_addr(config["etcd"]["args"]["listen-client-urls"])
            _, etcd_peer_port = _parse_addr(config["etcd"]["args"]["listen-peer-urls"])
            etcd_addresses = config["values"]["etcd"]["address"]
        ds_master_ip = ds_master_port = None
        if mode_config.get("ds_master", False):
            ds_master_ip, ds_master_port = _parse_addr(config["ds_master"]["args"]["master_address"])
        fm_ip = fm_port = None
        if mode_config.get("function_master", False):
            fm_ip, fm_port = _parse_addr(config["function_master"]["args"]["ip"])
        fp_port = fp_grpc_port = None
        if mode_config.get("function_proxy", False):
            _, fp_port = _parse_addr(config["function_proxy"]["args"]["address"])
            fp_grpc_port = config["function_proxy"]["args"]["grpc_listen_port"]
        ds_worker_port = None
        if mode_config.get("ds_worker", False):
            _, ds_worker_port = _parse_addr(config["ds_worker"]["args"]["worker_address"])
        frontend_port = config["frontend"].get("port") if mode_config.get("frontend") else None
        agent_ip = None
        if mode_config.get("function_agent", False):
            agent_ip = config["function_agent"]["args"]["ip"]

        session_data["cluster_info"] = {
            "for-join": {
                "function_master.ip": fm_ip,
                "function_master.port": fm_port,
                "etcd.ip": etcd_ip,
                "etcd.port": etcd_port,
                "etcd.peer_port": etcd_peer_port,
                "etcd.addresses": etcd_addresses,
                "ds_master.ip": ds_master_ip,
                "ds_master.port": ds_master_port,
                "function_proxy.port": fp_port,
                "function_proxy.grpc_port": fp_grpc_port,
                "ds_worker.port": ds_worker_port,
                "agent.ip": agent_ip,
                "frontend.port": frontend_port,
            },
            "daemon": {
                "pid": os.getpid(),
            },
        }

        session_file = Path(SESSION_JSON_PATH)
        session_file.parent.mkdir(parents=True, exist_ok=True)
        _atomic_write_json(session_file, session_data)
        logger.info(f"Session saved to: {session_file}")

        if self.mode == StartMode.MASTER:
            write_old_current_master_info(session_data)

    def stop_all(self, force: bool = False) -> None:
        """Thread-safe stop of all components."""
        with self._stop_lock:
            if self._stopped:
                return
            self._stopped = True

        logger.info("Stopping system components...")
        for launcher in self.components.values():
            launcher.terminate(force=force)
        logger.info("✅ All components stopped.")

    # ─── Monitoring ─────────────────────────────────────────────────────

    def start_monitor(self) -> None:
        """Start a background daemon thread that monitors component health."""
        if self._monitor_thread is not None and self._monitor_thread.is_alive():
            logger.info("Monitor daemon already running")
            return

        self._monitor_thread = threading.Thread(
            target=self._monitor_loop, daemon=True, name="component-monitor",
        )
        self._monitor_thread.start()

    def _monitor_loop(self) -> None:
        """Health monitor with exponential backoff restarts."""
        logger.info("🔍 Monitor daemon started")
        restart_counts: dict[str, int] = {}

        while not self._stopped:
            time.sleep(self.monitor_interval)
            if self._stopped:
                break

            for comp_name, launcher in list(self.components.items()):
                process = launcher.component_config.process
                if process is None:
                    continue
                exit_code = process.poll()
                if exit_code is None:
                    continue

                count = restart_counts.get(comp_name, 0)
                if self.max_restart_count > 0 and count >= self.max_restart_count:
                    logger.error(
                        f"Component {comp_name} exceeded max restart count "
                        f"({self.max_restart_count}). Not restarting.",
                    )
                    continue

                logger.warning(
                    f"Component {comp_name} (PID: {launcher.component_config.pid}) "
                    f"exited with code {exit_code}. Restarting...",
                )

                # Exponential backoff
                if self.restart_backoff_base > 0 and count > 0:
                    backoff = min(self.restart_backoff_base * (2 ** (count - 1)), 30.0)
                    logger.info(f"Waiting {backoff:.1f}s before restarting {comp_name}...")
                    time.sleep(backoff)

                try:
                    new_process = launcher.restart(preexec_fn=self.preexec_fn)
                    if new_process:
                        self.processes[comp_name] = new_process
                        launcher.component_config.restart_count += 1
                        restart_counts[comp_name] = count + 1
                        logger.info(
                            f"✅ Successfully restarted {comp_name} "
                            f"(new PID: {launcher.component_config.pid})",
                        )
                    else:
                        logger.error(f"❌ Failed to restart {comp_name}")
                except Exception:
                    logger.exception(f"❌ Error restarting {comp_name}")

        logger.info("🔍 Monitor daemon stopped")
        if self.shutdown_at_exit:
            self.stop_all(force=False)

    # ─── Utility ────────────────────────────────────────────────────────

    def get_component_configs(self) -> dict[str, ComponentConfig]:
        """Return map of component name → ComponentConfig for session saving."""
        return {name: launcher.component_config for name, launcher in self.components.items()}

    @property
    def is_stopped(self) -> bool:
        return self._stopped
