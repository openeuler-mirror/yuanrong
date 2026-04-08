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

"""Tests for ServiceManager — cluster detection, auto-start, cleanup."""

import importlib.util
import json
import os
import sys
import tempfile
import threading
import types
import unittest
from pathlib import Path
from unittest.mock import MagicMock, patch

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

    comp_dir = os.path.join(_CLI_DIR, "component")
    _load_module("yr.cli.component", os.path.join(comp_dir, "__init__.py"))
    _load_module("yr.cli.component.base", os.path.join(comp_dir, "base.py"))
    _load_module("yr.cli.component.registry", os.path.join(comp_dir, "registry.py"))

    _load_module("yr.process_utils", os.path.join(_YR_DIR, "process_utils.py"))
    _load_module("yr.cluster_launcher", os.path.join(_YR_DIR, "cluster_launcher.py"))
    _load_module("yr.service_manager", os.path.join(_YR_DIR, "service_manager.py"))


_setup()

from yr.service_manager import ServiceManager, ServiceEndpoints


# ─── Helpers ────────────────────────────────────────────────────────────

def _make_session_json(tmpdir, overrides=None):
    """Create a valid session.json with standard cluster_info."""
    session = {
        "components": {
            "function_master": {"pid": 1001, "status": "running"},
            "function_proxy": {"pid": 1002, "status": "running"},
            "ds_worker": {"pid": 1003, "status": "running"},
        },
        "cluster_info": {
            "for-join": {
                "function_master.ip": "127.0.0.1",
                "function_master.port": "8001",
                "ds_master.ip": "127.0.0.1",
                "ds_master.port": "9001",
                "function_proxy.port": "7001",
            },
        },
    }
    if overrides:
        session.update(overrides)
    path = os.path.join(tmpdir, "session.json")
    with open(path, "w") as f:
        json.dump(session, f)
    return path


# ─── Tests ──────────────────────────────────────────────────────────────

class TestDetectRunningCluster(unittest.TestCase):
    """Test multi-source detection logic."""

    def setUp(self):
        ServiceManager.reset()

    def test_no_session_file_returns_none(self):
        with (
            patch("yr.service_manager.SESSION_JSON_PATH", "/nonexistent/session.json"),
            patch("yr.service_manager._MASTER_INFO_PATH", "/nonexistent/master.info"),
            patch("yr.service_manager._CURRENT_MASTER_INFO_PATH", "/nonexistent/yr_info"),
        ):
            result = ServiceManager._detect_running_cluster()
        self.assertIsNone(result)

    def test_all_healthy_returns_endpoints(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            session_path = _make_session_json(tmpdir)
            with (
                patch("yr.service_manager.SESSION_JSON_PATH", session_path),
                patch("yr.service_manager.is_process_alive", return_value=True),
                patch("yr.service_manager.is_port_reachable", return_value=True),
            ):
                result = ServiceManager._detect_running_cluster()
            self.assertIsNotNone(result)
            self.assertEqual(result.server_address, "127.0.0.1:8001")
            self.assertEqual(result.ds_address, "127.0.0.1:9001")

    def test_dead_pid_returns_none(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            session_path = _make_session_json(tmpdir)
            with (
                patch("yr.service_manager.SESSION_JSON_PATH", session_path),
                patch("yr.service_manager.is_process_alive", return_value=False),
                patch("yr.service_manager._MASTER_INFO_PATH", "/nonexistent/master.info"),
                patch("yr.service_manager._CURRENT_MASTER_INFO_PATH", "/nonexistent/yr_info"),
            ):
                result = ServiceManager._detect_running_cluster()
            self.assertIsNone(result)

    def test_unreachable_port_returns_none(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            session_path = _make_session_json(tmpdir)
            with (
                patch("yr.service_manager.SESSION_JSON_PATH", session_path),
                patch("yr.service_manager.is_process_alive", return_value=True),
                patch("yr.service_manager.is_port_reachable", return_value=False),
                patch("yr.service_manager._MASTER_INFO_PATH", "/nonexistent/master.info"),
                patch("yr.service_manager._CURRENT_MASTER_INFO_PATH", "/nonexistent/yr_info"),
            ):
                result = ServiceManager._detect_running_cluster()
            self.assertIsNone(result)

    def test_corrupt_session_json_returns_none(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            path = os.path.join(tmpdir, "session.json")
            with open(path, "w") as f:
                f.write("NOT JSON")
            with (
                patch("yr.service_manager.SESSION_JSON_PATH", path),
                patch("yr.service_manager._MASTER_INFO_PATH", "/nonexistent/master.info"),
                patch("yr.service_manager._CURRENT_MASTER_INFO_PATH", "/nonexistent/yr_info"),
            ):
                result = ServiceManager._detect_running_cluster()
            self.assertIsNone(result)


class TestDetectFromMasterInfo(unittest.TestCase):
    """Test CLI-created master.info detection."""

    def setUp(self):
        ServiceManager.reset()

    def test_detects_cli_started_cluster(self):
        master_info = (
            "local_ip:172.17.0.2,master_ip:172.17.0.2,etcd_ip:172.17.0.2,"
            "etcd_port:15344,meta_service_port:13470,global_scheduler_port:15650,"
            "ds_master_port:16675,etcd_peer_port:12726,bus-proxy:38442,bus:27955,"
            "ds-worker:39079,"
        )
        with tempfile.TemporaryDirectory() as tmpdir:
            info_path = os.path.join(tmpdir, "master.info")
            with open(info_path, "w") as f:
                f.write(master_info)
            with (
                patch("yr.service_manager.SESSION_JSON_PATH", "/nonexistent/session.json"),
                patch("yr.service_manager._MASTER_INFO_PATH", info_path),
                patch("yr.service_manager._CURRENT_MASTER_INFO_PATH", "/nonexistent/yr_info"),
                patch("yr.service_manager.is_port_reachable", return_value=True),
            ):
                result = ServiceManager._detect_running_cluster()
            self.assertIsNotNone(result)
            self.assertEqual(result.server_address, "172.17.0.2:15650")
            self.assertEqual(result.ds_address, "172.17.0.2:16675")

    def test_dead_cli_cluster_returns_none(self):
        master_info = (
            "master_ip:127.0.0.1,global_scheduler_port:8001,"
            "ds_master_port:9001,bus:27955,"
        )
        with tempfile.TemporaryDirectory() as tmpdir:
            info_path = os.path.join(tmpdir, "master.info")
            with open(info_path, "w") as f:
                f.write(master_info)
            with (
                patch("yr.service_manager.SESSION_JSON_PATH", "/nonexistent/session.json"),
                patch("yr.service_manager._MASTER_INFO_PATH", info_path),
                patch("yr.service_manager._CURRENT_MASTER_INFO_PATH", "/nonexistent/yr_info"),
                patch("yr.service_manager.is_port_reachable", return_value=False),
            ):
                result = ServiceManager._detect_running_cluster()
            self.assertIsNone(result)

    def test_parse_master_info(self):
        raw = "key1:val1,key2:val2,key3:val3,"
        result = ServiceManager._parse_master_info(raw)
        self.assertEqual(result, {"key1": "val1", "key2": "val2", "key3": "val3"})

    def test_parse_empty_returns_none(self):
        self.assertIsNone(ServiceManager._parse_master_info(""))
        self.assertIsNone(ServiceManager._parse_master_info("  "))


class TestExtractEndpoints(unittest.TestCase):
    """Test endpoint extraction from cluster_info."""

    def test_valid_cluster_info(self):
        info = {
            "function_master.ip": "10.0.0.1",
            "function_master.port": "8080",
            "ds_master.ip": "10.0.0.1",
            "ds_master.port": "9090",
        }
        result = ServiceManager._extract_endpoints(info)
        self.assertEqual(result.server_address, "10.0.0.1:8080")
        self.assertEqual(result.ds_address, "10.0.0.1:9090")

    def test_missing_fm_returns_none(self):
        result = ServiceManager._extract_endpoints({})
        self.assertIsNone(result)

    def test_missing_ds_returns_empty_ds_address(self):
        info = {
            "function_master.ip": "10.0.0.1",
            "function_master.port": "8080",
        }
        result = ServiceManager._extract_endpoints(info)
        self.assertIsNotNone(result)
        self.assertEqual(result.ds_address, "")


class TestEnsureServices(unittest.TestCase):
    """Test the main ensure_services flow."""

    def setUp(self):
        ServiceManager.reset()

    def test_existing_cluster_reused(self):
        endpoints = ServiceEndpoints("1.2.3.4:8001", "1.2.3.4:9001")
        with patch.object(
            ServiceManager, "_detect_running_cluster", return_value=endpoints,
        ):
            result = ServiceManager.ensure_services()
        self.assertEqual(result.server_address, "1.2.3.4:8001")

    def test_starts_cluster_when_none_detected(self):
        mock_engine = MagicMock()
        mock_engine.start_all.return_value = True

        endpoints = ServiceEndpoints("127.0.0.1:8001", "127.0.0.1:9001")

        with (
            patch.object(ServiceManager, "_detect_running_cluster", return_value=None),
            patch.object(ServiceManager, "_start_cluster", return_value=mock_engine),
            patch.object(
                ServiceManager, "_read_endpoints_from_session", return_value=endpoints,
            ),
        ):
            result = ServiceManager.ensure_services()

        self.assertEqual(result.server_address, "127.0.0.1:8001")
        self.assertIsNotNone(ServiceManager._instance)

    def test_raises_on_start_failure(self):
        with (
            patch.object(ServiceManager, "_detect_running_cluster", return_value=None),
            patch.object(
                ServiceManager,
                "_start_cluster",
                side_effect=RuntimeError("start failed"),
            ),
        ):
            with self.assertRaises(RuntimeError):
                ServiceManager.ensure_services()


class TestShutdown(unittest.TestCase):
    """Test cleanup behavior."""

    def setUp(self):
        ServiceManager.reset()

    def test_shutdown_is_idempotent(self):
        mock_engine = MagicMock()
        mgr = ServiceManager(engine=mock_engine)
        mgr.shutdown()
        mgr.shutdown()  # second call should be no-op
        mock_engine.stop_all.assert_called_once()

    def test_shutdown_thread_safety(self):
        mock_engine = MagicMock()
        call_count = [0]

        def counting_stop_all(**kwargs):
            call_count[0] += 1

        mock_engine.stop_all = counting_stop_all
        mgr = ServiceManager(engine=mock_engine)

        threads = [threading.Thread(target=mgr.shutdown) for _ in range(5)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()

        self.assertEqual(call_count[0], 1)


class TestReset(unittest.TestCase):
    """Test singleton reset for testing."""

    def test_reset_clears_instance(self):
        mock_engine = MagicMock()
        ServiceManager._instance = ServiceManager(engine=mock_engine)
        ServiceManager.reset()
        self.assertIsNone(ServiceManager._instance)
        mock_engine.stop_all.assert_called()


if __name__ == "__main__":
    unittest.main()
