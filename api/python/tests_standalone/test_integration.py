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

"""Integration tests: full lifecycle with mocked components.

Tests the complete flow:
  ServiceManager.ensure_services() → ClusterLauncher → mock components → shutdown

All components are mocked (no real binaries), but the full call chain is exercised.
"""

import importlib.util
import json
import os
import subprocess
import sys
import tempfile
import threading
import time
import types
import unittest
from pathlib import Path
from unittest.mock import MagicMock, patch, PropertyMock

# ─── Module loading (bypass yr.__init__) ────────────────────────────────

_YR_DIR = os.path.join(os.path.dirname(__file__), "..", "yr")
_CLI_DIR = os.path.join(_YR_DIR, "cli")


def _load_module(name, filepath):
    spec = importlib.util.spec_from_file_location(name, filepath)
    mod = importlib.util.module_from_spec(spec)
    sys.modules[name] = mod
    spec.loader.exec_module(mod)
    return mod


def _make_ns_pkg(name, path):
    mod = types.ModuleType(name)
    mod.__path__ = [path]
    mod.__package__ = name
    sys.modules[name] = mod
    return mod


def _setup():
    yr_mock = types.ModuleType("yr")
    yr_mock.__path__ = [_YR_DIR]
    yr_mock.__package__ = "yr"
    sys.modules["yr"] = yr_mock

    for mod_name in [
        "yr.libruntime_pb2", "yr.apis", "yr.fnruntime", "yr.config",
        "tomli_w", "tomllib",
    ]:
        sys.modules[mod_name] = MagicMock()

    _make_ns_pkg("yr.cli", _CLI_DIR)
    _load_module("yr.cli.const", os.path.join(_CLI_DIR, "const.py"))

    config_mock = MagicMock()
    config_mock.ConfigResolver = MagicMock
    sys.modules["yr.cli.config"] = config_mock

    sys_launcher_mock = MagicMock()
    sys_launcher_mock._parse_addr = MagicMock(return_value=("127.0.0.1", "8080"))
    sys_launcher_mock._atomic_write_json = MagicMock()
    sys_launcher_mock.write_old_current_master_info = MagicMock()
    sys.modules["yr.cli.system_launcher"] = sys_launcher_mock

    comp_dir = os.path.join(_CLI_DIR, "component")
    _load_module("yr.cli.component", os.path.join(comp_dir, "__init__.py"))
    _load_module("yr.cli.component.base", os.path.join(comp_dir, "base.py"))
    _load_module("yr.cli.component.registry", os.path.join(comp_dir, "registry.py"))

    _load_module("yr.process_utils", os.path.join(_YR_DIR, "process_utils.py"))
    _load_module("yr.cluster_launcher", os.path.join(_YR_DIR, "cluster_launcher.py"))
    _load_module("yr.service_manager", os.path.join(_YR_DIR, "service_manager.py"))


_setup()

from yr.cluster_launcher import ClusterLauncher
from yr.service_manager import ServiceManager, ServiceEndpoints
from yr.cli.const import StartMode


# ─── Helpers ────────────────────────────────────────────────────────────

def _write_session_json(tmpdir, data):
    """Write session.json to tmpdir."""
    path = os.path.join(tmpdir, "session.json")
    with open(path, "w") as f:
        json.dump(data, f)
    return path


def _make_valid_session():
    """Standard session data with all required fields."""
    return {
        "components": {
            "etcd": {"pid": 2001, "status": "running"},
            "ds_master": {"pid": 2002, "status": "running"},
            "ds_worker": {"pid": 2003, "status": "running"},
            "function_master": {"pid": 2004, "status": "running"},
            "function_proxy": {"pid": 2005, "status": "running"},
            "function_agent": {"pid": 2006, "status": "running"},
        },
        "cluster_info": {
            "for-join": {
                "function_master.ip": "127.0.0.1",
                "function_master.port": "8001",
                "ds_master.ip": "127.0.0.1",
                "ds_master.port": "9001",
                "etcd.ip": "127.0.0.1",
                "etcd.port": "2379",
                "function_proxy.port": "7001",
                "function_proxy.grpc_port": "7002",
                "ds_worker.port": "9002",
            },
            "daemon": {"pid": 1000},
        },
        "mode": "master",
        "session_id": "test-session-001",
        "start_time": "Thu Jan  1 00:00:00 2025",
    }


# ─── Integration Tests ─────────────────────────────────────────────────

class TestDetectAndReuseExistingCluster(unittest.TestCase):
    """Test: when a healthy cluster exists, ensure_services() reuses it."""

    def setUp(self):
        ServiceManager.reset()

    def test_reuses_healthy_existing_cluster(self):
        endpoints = ServiceEndpoints("127.0.0.1:8001", "127.0.0.1:9001")
        with patch.object(
            ServiceManager, "_detect_running_cluster", return_value=endpoints,
        ):
            result = ServiceManager.ensure_services()

        self.assertEqual(result.server_address, "127.0.0.1:8001")
        self.assertEqual(result.ds_address, "127.0.0.1:9001")
        # No engine should have been created (reuse path)
        self.assertIsNone(ServiceManager._instance)

    def test_starts_new_when_dead_components(self):
        """If detection finds dead PIDs, should start a new cluster."""
        mock_engine = MagicMock()
        mock_engine.start_all.return_value = True

        with tempfile.TemporaryDirectory() as tmpdir:
            session_path = _write_session_json(tmpdir, _make_valid_session())
            endpoints = ServiceEndpoints("127.0.0.1:8001", "127.0.0.1:9001")

            with (
                patch("yr.service_manager.SESSION_JSON_PATH", session_path),
                patch("yr.service_manager.is_process_alive", return_value=False),
                patch.object(ServiceManager, "_start_cluster", return_value=mock_engine),
                patch.object(ServiceManager, "_read_endpoints_from_session", return_value=endpoints),
            ):
                result = ServiceManager.ensure_services()

            self.assertEqual(result.server_address, "127.0.0.1:8001")
            self.assertIsNotNone(ServiceManager._instance)


class TestFullStartupShutdownCycle(unittest.TestCase):
    """Test: full startup → monitor → shutdown lifecycle with mocked engine."""

    def setUp(self):
        ServiceManager.reset()

    def test_startup_and_shutdown(self):
        mock_engine = MagicMock()
        mock_engine.start_all.return_value = True
        mock_engine._stopped = False

        endpoints = ServiceEndpoints("127.0.0.1:8001", "127.0.0.1:9001")

        with (
            patch.object(ServiceManager, "_detect_running_cluster", return_value=None),
            patch.object(ServiceManager, "_start_cluster", return_value=mock_engine),
            patch.object(ServiceManager, "_read_endpoints_from_session", return_value=endpoints),
        ):
            result = ServiceManager.ensure_services()

        self.assertEqual(result.server_address, "127.0.0.1:8001")
        self.assertIsNotNone(ServiceManager._instance)

        # Shutdown
        ServiceManager._instance.shutdown()
        mock_engine.stop_all.assert_called_once_with(force=False)


class TestClusterLauncherDirectIntegration(unittest.TestCase):
    """Test ClusterLauncher with mocked components — no ServiceManager."""

    def test_start_stop_cycle(self):
        """Load mock components, start them, stop them."""
        resolver = MagicMock()
        resolver.runtime_context = {
            "deploy_path": Path("/tmp/yr_test_integration"),
            "time": "20250101_120000",
        }
        resolver.rendered_config = {
            "mode": {"master": {"comp_a": True, "comp_b": True}},
        }

        engine = ClusterLauncher(
            resolver=resolver,
            mode=StartMode.MASTER,
            max_restart_count=3,
        )

        # Mock launcher classes
        for name in ["comp_a", "comp_b"]:
            mock_class = MagicMock()
            mock_instance = MagicMock()
            mock_instance.component_config.prepend_char = "--"
            mock_instance.component_config.depends_on = []
            mock_instance.component_config.process = None
            mock_instance.component_config.pid = None
            mock_instance.component_config.restart_count = 0
            mock_instance.wait_until_healthy.return_value = True
            mock_process = MagicMock()
            mock_process.pid = 10000
            mock_process.poll.return_value = None
            mock_instance.launch.return_value = mock_process
            mock_class.return_value = mock_instance
            engine.launcher_classes[name] = mock_class

        engine.load_components()
        self.assertEqual(len(engine.components), 2)

        order = engine.get_start_order()
        result = engine.start_components_in_order(order)
        self.assertTrue(result)

        # Verify all components had launch() called
        for name in engine.components:
            engine.components[name].launch.assert_called_once()

        # Stop
        engine.stop_all()
        self.assertTrue(engine.is_stopped)
        for name in engine.components:
            engine.components[name].terminate.assert_called_once()


class TestPreexecFnPropagation(unittest.TestCase):
    """Test that preexec_fn flows from ClusterLauncher to component launch."""

    def test_preexec_fn_passed_through(self):
        resolver = MagicMock()
        resolver.runtime_context = {
            "deploy_path": Path("/tmp/yr_test_preexec"),
            "time": "20250101_120000",
        }
        resolver.rendered_config = {
            "mode": {"master": {"comp_x": True}},
        }

        mock_fn = MagicMock()
        engine = ClusterLauncher(
            resolver=resolver,
            mode=StartMode.MASTER,
            preexec_fn=mock_fn,
        )

        mock_class = MagicMock()
        mock_instance = MagicMock()
        mock_instance.component_config.prepend_char = "--"
        mock_instance.component_config.depends_on = []
        mock_instance.component_config.process = None
        mock_instance.wait_until_healthy.return_value = True
        mock_process = MagicMock()
        mock_process.pid = 99999
        mock_instance.launch.return_value = mock_process
        mock_class.return_value = mock_instance
        engine.launcher_classes["comp_x"] = mock_class

        engine.load_components()
        engine.start_component("comp_x")

        engine.components["comp_x"].launch.assert_called_once_with(preexec_fn=mock_fn)


class TestConcurrentEnsureServices(unittest.TestCase):
    """Test thread safety: multiple threads calling ensure_services()."""

    def setUp(self):
        ServiceManager.reset()

    def test_only_one_cluster_started(self):
        call_count = [0]

        def mock_start():
            call_count[0] += 1
            time.sleep(0.05)  # simulate startup delay
            return MagicMock()

        endpoints = ServiceEndpoints("127.0.0.1:8001", "127.0.0.1:9001")

        with (
            patch.object(ServiceManager, "_detect_running_cluster", return_value=None),
            patch.object(ServiceManager, "_start_cluster", side_effect=mock_start),
            patch.object(ServiceManager, "_read_endpoints_from_session", return_value=endpoints),
        ):
            results = [None] * 5
            errors = [None] * 5

            def run(idx):
                try:
                    results[idx] = ServiceManager.ensure_services()
                except Exception as e:
                    errors[idx] = e

            threads = [threading.Thread(target=run, args=(i,)) for i in range(5)]
            for t in threads:
                t.start()
            for t in threads:
                t.join()

        # Only one cluster should have been started
        self.assertEqual(call_count[0], 1)
        # All threads should get valid endpoints
        for i, result in enumerate(results):
            self.assertIsNone(errors[i], f"Thread {i} got error: {errors[i]}")
            self.assertIsNotNone(result)


if __name__ == "__main__":
    unittest.main()
