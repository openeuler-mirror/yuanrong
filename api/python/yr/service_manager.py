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

"""ServiceManager: API adapter for yr.init() auto-start.

Handles cluster detection, automatic startup via ClusterLauncher,
and cleanup on process exit. Follows the pattern:

    detect existing cluster → if found, reuse → if not, start → register cleanup
"""

from __future__ import annotations

import atexit
import json
import logging
import signal
import sys
import threading
from dataclasses import dataclass
from pathlib import Path
from typing import ClassVar, Optional

from yr.cli.const import (
    DEFAULT_CONFIG_PATH,
    SESSION_JSON_PATH,
    SESSION_LATEST_PATH,
    SESSIONS_DIR,
    StartMode,
)
from yr.process_utils import is_port_reachable, is_process_alive, set_pdeathsig

# Paths written by the CLI (Go binary) and deploy.sh
_MASTER_INFO_PATH = f"{SESSION_LATEST_PATH}/master.info"
_CURRENT_MASTER_INFO_PATH = f"{SESSIONS_DIR}/yr_current_master_info"

logger = logging.getLogger(__name__)


@dataclass
class ServiceEndpoints:
    """Endpoints returned by ServiceManager for yr.init() to connect to."""

    server_address: str
    ds_address: str


class ServiceManager:
    """Singleton adapter that ensures a local cluster is running for yr.init().

    Thread-safe. Only one instance is created per process.
    """

    _instance: ClassVar[Optional[ServiceManager]] = None
    _lock: ClassVar[threading.Lock] = threading.Lock()

    def __init__(self, engine=None):
        self._engine = engine
        self._started_by_us = False
        self._shutdown_called = False
        self._shutdown_lock = threading.Lock()

    @classmethod
    def ensure_services(cls) -> ServiceEndpoints:
        """Ensure backend services are running. Returns endpoints to connect to.

        This is the main entry point called from yr.init().
        Thread-safe: only one caller will actually start services.
        """
        with cls._lock:
            # Fast path: already started by this process
            if cls._instance is not None and cls._instance._started_by_us:
                endpoints = cls._detect_from_master_info()
                if endpoints:
                    return endpoints

            # 1. Detect existing cluster
            endpoints = cls._detect_running_cluster()
            if endpoints:
                logger.info("Detected existing cluster, reusing.")
                return endpoints

            # 2. Start a new cluster via CLI
            logger.info("No existing cluster detected. Starting local cluster...")
            cls._start_cluster()

            # 3. Register cleanup and save instance
            instance = cls(engine=None)
            instance._started_by_us = True
            instance._register_cleanup()
            cls._instance = instance

            # 4. Read endpoints from the newly started cluster
            endpoints = cls._detect_from_master_info()
            if not endpoints:
                endpoints = cls._read_endpoints_from_session()
            if not endpoints:
                instance.shutdown()
                raise RuntimeError(
                    "Cluster started but failed to read endpoints from session."
                )
            return endpoints

    @staticmethod
    def _detect_running_cluster() -> Optional[ServiceEndpoints]:
        """Multi-source detection for existing clusters.

        Checks in order:
        1. session.json (written by ClusterLauncher / API auto-start)
        2. master.info / yr_current_master_info (written by CLI: yr start --master)
        """
        # 1. Check session.json (our own format)
        endpoints = ServiceManager._detect_from_session_json()
        if endpoints:
            return endpoints

        # 2. Check CLI-created master.info
        endpoints = ServiceManager._detect_from_master_info()
        if endpoints:
            return endpoints

        return None

    @staticmethod
    def _detect_from_session_json() -> Optional[ServiceEndpoints]:
        """Detect cluster from session.json (API auto-start format)."""
        session_path = Path(SESSION_JSON_PATH)
        if not session_path.exists():
            return None

        try:
            session = json.loads(session_path.read_text())
        except (json.JSONDecodeError, OSError):
            logger.debug("Failed to read session.json")
            return None

        components = session.get("components", {})

        # Check key component PIDs are alive
        for name in ("function_master", "function_proxy", "ds_worker"):
            comp_info = components.get(name, {})
            pid = comp_info.get("pid")
            if not pid or not is_process_alive(pid):
                logger.debug(f"Component {name} (pid={pid}) not alive.")
                return None

        # Check function_proxy port is reachable
        cluster_info = session.get("cluster_info", {}).get("for-join", {})
        proxy_port = cluster_info.get("function_proxy.port")
        if proxy_port:
            if not is_port_reachable("127.0.0.1", int(proxy_port)):
                logger.debug(f"function_proxy port {proxy_port} not reachable.")
                return None

        return ServiceManager._extract_endpoints(cluster_info)

    @staticmethod
    def _detect_from_master_info() -> Optional[ServiceEndpoints]:
        """Detect cluster from CLI-created master.info or yr_current_master_info.

        These files use the format:
            key1:val1,key2:val2,...
        """
        for path_str in (_MASTER_INFO_PATH, _CURRENT_MASTER_INFO_PATH):
            path = Path(path_str)
            if not path.exists():
                continue
            try:
                raw = path.read_text().strip()
                info = ServiceManager._parse_master_info(raw)
                if not info:
                    continue

                # Verify the cluster is actually alive by probing a port
                bus_port = info.get("bus")
                master_ip = info.get("master_ip", "127.0.0.1")
                if bus_port and is_port_reachable(master_ip, int(bus_port)):
                    # server_address uses 'bus' port (function_proxy bus),
                    # ds_address uses 'ds-worker' port — matching C++ auto_init
                    ds_worker_port = info.get("ds-worker")
                    if bus_port:
                        server_addr = f"{master_ip}:{bus_port}"
                        ds_addr = f"{master_ip}:{ds_worker_port}" if ds_worker_port else ""
                        logger.info(
                            "Detected CLI-started cluster at %s", server_addr
                        )
                        return ServiceEndpoints(
                            server_address=server_addr,
                            ds_address=ds_addr,
                        )
            except (OSError, ValueError) as e:
                logger.debug("Failed to parse %s: %s", path_str, e)
                continue
        return None

    @staticmethod
    def _parse_master_info(raw: str) -> Optional[dict]:
        """Parse key:val,key:val,... format from master.info."""
        if not raw:
            return None
        result = {}
        for pair in raw.split(","):
            pair = pair.strip()
            if not pair:
                continue
            parts = pair.split(":", 1)
            if len(parts) == 2:
                result[parts[0]] = parts[1]
        return result if result else None

    @staticmethod
    def _read_endpoints_from_session() -> Optional[ServiceEndpoints]:
        """Read endpoints from session.json without health checks."""
        session_path = Path(SESSION_JSON_PATH)
        if not session_path.exists():
            return None
        try:
            session = json.loads(session_path.read_text())
            cluster_info = session.get("cluster_info", {}).get("for-join", {})
            return ServiceManager._extract_endpoints(cluster_info)
        except (json.JSONDecodeError, OSError, KeyError):
            return None

    @staticmethod
    def _extract_endpoints(cluster_info: dict) -> Optional[ServiceEndpoints]:
        """Extract ServiceEndpoints from cluster_info dict."""
        fm_ip = cluster_info.get("function_master.ip")
        fm_port = cluster_info.get("function_master.port")
        ds_ip = cluster_info.get("ds_master.ip")
        ds_port = cluster_info.get("ds_master.port")

        if not fm_ip or not fm_port:
            return None

        server_address = f"{fm_ip}:{fm_port}"
        ds_address = f"{ds_ip}:{ds_port}" if ds_ip and ds_port else ""
        return ServiceEndpoints(
            server_address=server_address,
            ds_address=ds_address,
        )

    @staticmethod
    def _start_cluster():
        """Start a local cluster via 'yr start --master' subprocess.

        Uses the CLI's proven path (Go binary + deploy.sh) which correctly
        resolves all binary paths and component ordering. This avoids
        reimplementing the complex deployment logic in Python.

        The CLI writes master.info which we read for endpoints.
        """
        import shutil
        import subprocess
        import time

        yr_bin = shutil.which("yr")
        if not yr_bin:
            raise RuntimeError(
                "Cannot find 'yr' CLI binary. Ensure the yr package is installed."
            )

        logger.info("Starting cluster via: yr start --master")
        start_time = time.monotonic()

        try:
            result = subprocess.run(
                [yr_bin, "start", "--master"],
                capture_output=True,
                text=True,
                timeout=120,
            )
        except subprocess.TimeoutExpired:
            raise RuntimeError(
                "Cluster start timed out after 120 seconds."
            )

        elapsed = time.monotonic() - start_time

        if result.returncode != 0:
            logger.error("yr start --master failed (exit=%d):\nstdout: %s\nstderr: %s",
                         result.returncode, result.stdout, result.stderr)
            raise RuntimeError(
                f"Failed to start local cluster (exit code {result.returncode})."
            )

        logger.info("Cluster started in %.1f seconds via CLI.", elapsed)
        # Return None for engine — cleanup uses 'yr stop' via subprocess
        return None

    def _register_cleanup(self) -> None:
        """Register atexit and signal handlers for cleanup."""
        atexit.register(self.shutdown)

        # Chain SIGTERM handler (don't replace user's handler)
        prev_handler = signal.getsignal(signal.SIGTERM)

        def _sigterm_handler(signum, frame):
            self.shutdown()
            if callable(prev_handler) and prev_handler not in (
                signal.SIG_DFL,
                signal.SIG_IGN,
            ):
                prev_handler(signum, frame)

        try:
            signal.signal(signal.SIGTERM, _sigterm_handler)
        except (OSError, ValueError):
            # Can't set signal handler (not main thread, etc.)
            logger.debug("Could not register SIGTERM handler for cleanup.")

    def shutdown(self) -> None:
        """Stop all managed services. Idempotent and thread-safe."""
        with self._shutdown_lock:
            if self._shutdown_called:
                return
            self._shutdown_called = True

        if not self._started_by_us:
            return

        if self._engine is not None:
            logger.info("Shutting down auto-started cluster via engine...")
            try:
                self._engine.stop_all(force=False)
            except Exception:
                logger.exception("Error during cluster shutdown")
            self._engine = None
        else:
            # CLI-started cluster: use 'yr stop' subprocess
            import shutil
            import subprocess

            yr_bin = shutil.which("yr")
            if yr_bin:
                logger.info("Shutting down auto-started cluster via: yr stop")
                try:
                    subprocess.run(
                        [yr_bin, "stop"],
                        capture_output=True,
                        text=True,
                        timeout=30,
                    )
                except Exception:
                    logger.exception("Error during cluster shutdown via CLI")

    @classmethod
    def reset(cls) -> None:
        """Reset singleton state. For testing only."""
        with cls._lock:
            if cls._instance is not None:
                cls._instance.shutdown()
            cls._instance = None
