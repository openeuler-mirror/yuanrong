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

"""Tests for ClusterLauncher shared engine.

Uses importlib to load cluster_launcher.py without triggering yr.__init__.
Heavy use of mocks since we can't instantiate real components in unit tests.
"""

import importlib.util
import os
import subprocess
import sys
import threading
import time
import types
import unittest
from pathlib import Path
from unittest.mock import MagicMock, patch, PropertyMock


# ─── Module loading helpers ─────────────────────────────────────────────

_YR_DIR = os.path.join(os.path.dirname(__file__), "..", "yr")
_CLI_DIR = os.path.join(_YR_DIR, "cli")


def _load_module(name, filepath):
    """Load a Python module by file path, bypassing yr.__init__."""
    spec = importlib.util.spec_from_file_location(name, filepath)
    mod = importlib.util.module_from_spec(spec)
    sys.modules[name] = mod
    spec.loader.exec_module(mod)
    return mod


def _make_namespace_package(name, path):
    """Create a namespace package module without __init__.py."""
    mod = types.ModuleType(name)
    mod.__path__ = [path]
    mod.__package__ = name
    sys.modules[name] = mod
    return mod


def _setup_mock_modules():
    """Pre-populate sys.modules with mocks for yr submodules that need C++ extensions."""
    # Create mock yr package
    yr_mock = types.ModuleType("yr")
    yr_mock.__path__ = [_YR_DIR]
    yr_mock.__file__ = os.path.join(_YR_DIR, "__init__.py")
    yr_mock.__package__ = "yr"
    sys.modules["yr"] = yr_mock

    # Mock modules that would require C++ extensions or unavailable deps
    for mod_name in [
        "yr.libruntime_pb2", "yr.apis", "yr.fnruntime", "yr.config",
        "tomli_w", "tomllib",
    ]:
        sys.modules[mod_name] = MagicMock()

    # Create namespace packages
    _make_namespace_package("yr.cli", _CLI_DIR)

    # Load const module (pure Python, has StartMode enum — no external deps)
    _load_module("yr.cli.const", os.path.join(_CLI_DIR, "const.py"))

    # Mock ConfigResolver before loading component modules
    config_mock = MagicMock()
    config_mock.ConfigResolver = MagicMock
    sys.modules["yr.cli.config"] = config_mock

    # Mock system_launcher helpers (cluster_launcher imports from it)
    sys_launcher_mock = MagicMock()
    sys_launcher_mock._parse_addr = MagicMock(return_value=("127.0.0.1", "8080"))
    sys_launcher_mock._atomic_write_json = MagicMock()
    sys_launcher_mock.write_old_current_master_info = MagicMock()
    sys.modules["yr.cli.system_launcher"] = sys_launcher_mock

    # Load component modules
    comp_dir = os.path.join(_CLI_DIR, "component")
    _load_module("yr.cli.component", os.path.join(comp_dir, "__init__.py"))
    _load_module("yr.cli.component.base", os.path.join(comp_dir, "base.py"))
    _load_module("yr.cli.component.registry", os.path.join(comp_dir, "registry.py"))


_setup_mock_modules()

# Now we can import ClusterLauncher
_load_module("yr.cluster_launcher", os.path.join(_YR_DIR, "cluster_launcher.py"))
from yr.cluster_launcher import ClusterLauncher
from yr.cli.const import StartMode


# ─── Test fixtures ──────────────────────────────────────────────────────

def _make_mock_resolver(components_config=None):
    """Create a mock ConfigResolver."""
    resolver = MagicMock()
    resolver.runtime_context = {
        "deploy_path": Path("/tmp/yr_test_deploy"),
        "time": "20250101_120000",
    }
    if components_config is None:
        components_config = {
            "etcd": True,
            "ds_master": True,
            "ds_worker": True,
            "function_master": True,
            "function_proxy": True,
            "function_agent": True,
        }
    resolver.rendered_config = {
        "mode": {"master": components_config},
        "etcd": {"args": {"listen-client-urls": "http://127.0.0.1:2379", "listen-peer-urls": "http://127.0.0.1:2380"}},
        "values": {"etcd": {"address": "127.0.0.1:2379"}},
        "ds_master": {"args": {"master_address": "127.0.0.1:9000"}},
        "ds_worker": {"args": {"worker_address": "127.0.0.1:9001", "enable_distributed_master": False}},
        "function_master": {"args": {"ip": "127.0.0.1:8000"}},
        "function_proxy": {"args": {"address": "127.0.0.1:8001", "grpc_listen_port": "8002"}},
        "function_agent": {"args": {"ip": "127.0.0.1"}},
        "frontend": {"port": "3000"},
    }
    return resolver


def _make_launcher_with_mocked_components(
    components_config=None, **kwargs
):
    """Create a ClusterLauncher with mocked component launchers."""
    resolver = _make_mock_resolver(components_config)
    engine = ClusterLauncher(
        resolver=resolver,
        mode=StartMode.MASTER,
        **kwargs,
    )

    # Replace launcher_classes with mocks that produce mock ComponentLaunchers
    mock_launcher_classes = {}
    for comp_name in (components_config or {
        "etcd": True, "ds_master": True, "ds_worker": True,
        "function_master": True, "function_proxy": True, "function_agent": True,
    }):
        mock_class = MagicMock()
        mock_instance = MagicMock()
        mock_instance.component_config.prepend_char = "--"
        mock_instance.component_config.depends_on = []
        mock_instance.component_config.process = None
        mock_instance.component_config.pid = None
        mock_instance.component_config.restart_count = 0
        mock_instance.wait_until_healthy.return_value = True
        mock_process = MagicMock()
        mock_process.pid = 10000 + hash(comp_name) % 1000
        mock_process.poll.return_value = None  # running
        mock_instance.launch.return_value = mock_process
        mock_instance.restart.return_value = mock_process
        mock_class.return_value = mock_instance
        mock_launcher_classes[comp_name] = mock_class

    engine.launcher_classes = mock_launcher_classes
    return engine


# ─── Tests ──────────────────────────────────────────────────────────────

class TestClusterLauncherInit(unittest.TestCase):
    """Test ClusterLauncher initialization."""

    def test_default_values(self):
        resolver = _make_mock_resolver()
        engine = ClusterLauncher(resolver=resolver, mode=StartMode.MASTER)
        self.assertFalse(engine.shutdown_at_exit)
        self.assertIsNone(engine.preexec_fn)
        self.assertEqual(engine.monitor_interval, 3)
        self.assertEqual(engine.max_restart_count, 0)
        self.assertFalse(engine._stopped)

    def test_custom_values(self):
        resolver = _make_mock_resolver()
        fn = MagicMock()
        engine = ClusterLauncher(
            resolver=resolver,
            mode=StartMode.MASTER,
            preexec_fn=fn,
            shutdown_at_exit=True,
            monitor_interval=5,
            max_restart_count=3,
            restart_backoff_base=2.0,
        )
        self.assertEqual(engine.preexec_fn, fn)
        self.assertTrue(engine.shutdown_at_exit)
        self.assertEqual(engine.monitor_interval, 5)
        self.assertEqual(engine.max_restart_count, 3)
        self.assertEqual(engine.restart_backoff_base, 2.0)


class TestGetStartOrder(unittest.TestCase):
    """Test topological sort of components."""

    def test_no_dependencies_returns_all(self):
        config = {"comp_a": True, "comp_b": True}
        engine = _make_launcher_with_mocked_components(config)
        engine.load_components()
        order = engine.get_start_order()
        self.assertEqual(set(order), {"comp_a", "comp_b"})

    def test_linear_dependency_chain(self):
        config = {"comp_a": True, "comp_b": True, "comp_c": True}
        engine = _make_launcher_with_mocked_components(config)
        engine.load_components()
        # Set up chain: a -> b -> c
        engine.components["comp_b"].component_config.depends_on = ["comp_a"]
        engine.components["comp_c"].component_config.depends_on = ["comp_b"]
        order = engine.get_start_order()
        self.assertEqual(order, ["comp_a", "comp_b", "comp_c"])

    def test_disabled_component_not_loaded(self):
        config = {"comp_a": True, "comp_b": False}
        engine = _make_launcher_with_mocked_components(config)
        engine.load_components()
        self.assertIn("comp_a", engine.components)
        self.assertNotIn("comp_b", engine.components)


class TestStartComponentsInOrder(unittest.TestCase):
    """Test sequential component startup."""

    def test_all_healthy_returns_true(self):
        config = {"comp_a": True, "comp_b": True}
        engine = _make_launcher_with_mocked_components(config)
        engine.load_components()
        order = engine.get_start_order()
        result = engine.start_components_in_order(order)
        self.assertTrue(result)
        for name in engine.components:
            engine.components[name].launch.assert_called_once()

    def test_unhealthy_component_returns_false(self):
        config = {"comp_a": True, "comp_b": True}
        engine = _make_launcher_with_mocked_components(config)
        engine.load_components()
        # Make comp_a fail health check
        engine.components["comp_a"].wait_until_healthy.return_value = False
        order = engine.get_start_order()
        result = engine.start_components_in_order(order)
        self.assertFalse(result)

    def test_preexec_fn_passed_to_launch(self):
        fn = MagicMock()
        config = {"comp_a": True}
        engine = _make_launcher_with_mocked_components(config, preexec_fn=fn)
        engine.load_components()
        engine.start_component("comp_a")
        engine.components["comp_a"].launch.assert_called_once_with(preexec_fn=fn)


class TestStopAll(unittest.TestCase):
    """Test thread-safe stop."""

    def test_stop_terminates_all_components(self):
        config = {"comp_a": True, "comp_b": True}
        engine = _make_launcher_with_mocked_components(config)
        engine.load_components()
        engine.stop_all()
        for name in engine.components:
            engine.components[name].terminate.assert_called_once_with(force=False)
        self.assertTrue(engine._stopped)

    def test_stop_is_idempotent(self):
        config = {"comp_a": True}
        engine = _make_launcher_with_mocked_components(config)
        engine.load_components()
        engine.stop_all()
        engine.stop_all()  # Second call should be no-op
        engine.components["comp_a"].terminate.assert_called_once()

    def test_stop_is_thread_safe(self):
        config = {"comp_a": True}
        engine = _make_launcher_with_mocked_components(config)
        engine.load_components()

        call_count = 0

        original_terminate = engine.components["comp_a"].terminate

        def counting_terminate(**kwargs):
            nonlocal call_count
            call_count += 1
            time.sleep(0.05)

        engine.components["comp_a"].terminate = counting_terminate

        threads = [threading.Thread(target=engine.stop_all) for _ in range(5)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()

        self.assertEqual(call_count, 1)


class TestMonitorLoop(unittest.TestCase):
    """Test health monitoring with restarts."""

    def test_monitor_restarts_crashed_component(self):
        config = {"comp_a": True}
        engine = _make_launcher_with_mocked_components(config, monitor_interval=0)
        engine.load_components()
        engine.start_components_in_order(["comp_a"])

        # Simulate: set component_config.process to a mock with poll behavior
        process_mock = MagicMock()
        call_count = [0]

        def poll_side_effect():
            call_count[0] += 1
            if call_count[0] == 1:
                return 1  # crashed
            return None  # running after restart

        process_mock.poll.side_effect = poll_side_effect
        engine.components["comp_a"].component_config.process = process_mock
        engine.components["comp_a"].component_config.pid = 12345

        # Run monitor for a short time
        engine.start_monitor()
        time.sleep(0.3)
        engine._stopped = True
        engine._monitor_thread.join(timeout=2)

        engine.components["comp_a"].restart.assert_called()

    def test_monitor_respects_max_restart_count(self):
        config = {"comp_a": True}
        engine = _make_launcher_with_mocked_components(
            config, monitor_interval=0, max_restart_count=2,
            restart_backoff_base=0,
        )
        engine.load_components()
        engine.start_components_in_order(["comp_a"])

        # Always report as crashed
        process_mock = MagicMock()
        process_mock.poll.return_value = 1
        engine.components["comp_a"].component_config.process = process_mock
        engine.components["comp_a"].component_config.pid = 12345

        engine.start_monitor()
        time.sleep(0.5)
        engine._stopped = True
        engine._monitor_thread.join(timeout=2)

        # Should have been called at most max_restart_count times
        self.assertLessEqual(
            engine.components["comp_a"].restart.call_count,
            2,
        )


class TestGetComponentConfigs(unittest.TestCase):
    """Test utility methods."""

    def test_returns_component_configs(self):
        config = {"comp_a": True, "comp_b": True}
        engine = _make_launcher_with_mocked_components(config)
        engine.load_components()
        configs = engine.get_component_configs()
        self.assertIn("comp_a", configs)
        self.assertIn("comp_b", configs)

    def test_is_stopped_property(self):
        resolver = _make_mock_resolver()
        engine = ClusterLauncher(resolver=resolver, mode=StartMode.MASTER)
        self.assertFalse(engine.is_stopped)
        engine._stopped = True
        self.assertTrue(engine.is_stopped)


class TestDistributedMasterSkip(unittest.TestCase):
    """Test that ds_master is skipped when enable_distributed_master is True."""

    def test_ds_master_skipped_when_distributed_master_enabled(self):
        config = {"ds_master": True, "ds_worker": True, "function_master": True}
        engine = _make_launcher_with_mocked_components(config)
        # Enable distributed master in the rendered config
        engine.resolver.rendered_config["ds_worker"]["args"]["enable_distributed_master"] = True
        engine.load_components()
        self.assertNotIn("ds_master", engine.components)
        self.assertIn("ds_worker", engine.components)
        self.assertIn("function_master", engine.components)

    def test_ds_master_loaded_when_distributed_master_disabled(self):
        config = {"ds_master": True, "ds_worker": True}
        engine = _make_launcher_with_mocked_components(config)
        engine.resolver.rendered_config["ds_worker"]["args"]["enable_distributed_master"] = False
        engine.load_components()
        self.assertIn("ds_master", engine.components)


class TestSaveSession(unittest.TestCase):
    """Test session.json writing."""

    def test_save_session_calls_atomic_write(self):
        import inspect
        config = {"etcd": True, "function_master": True}
        engine = _make_launcher_with_mocked_components(config)
        engine.load_components()
        engine.start_components_in_order(["etcd", "function_master"])

        # Set up component PIDs
        for name, launcher in engine.components.items():
            launcher.component_config.pid = 12345
            launcher.component_config.status = MagicMock()
            launcher.component_config.status.value = "running"
            launcher.component_config.start_time = 1000.0
            launcher.component_config.args = ["fake_cmd"]
            launcher.component_config.env_vars = {}
            launcher.component_config.restart_count = 0

        # Patch in the actual module where save_session's globals live,
        # not sys.modules (which may have been replaced by other test files).
        globs = engine.save_session.__func__.__globals__
        mock_write = MagicMock()
        mock_parse = MagicMock(return_value=("127.0.0.1", "8080"))
        mock_master_info = MagicMock()
        saved = {k: globs[k] for k in ("_atomic_write_json", "_parse_addr", "write_old_current_master_info")}
        globs["_atomic_write_json"] = mock_write
        globs["_parse_addr"] = mock_parse
        globs["write_old_current_master_info"] = mock_master_info
        try:
            with patch.object(Path, 'mkdir'):
                engine.save_session()
        finally:
            globs.update(saved)

        mock_write.assert_called()
        call_args = mock_write.call_args
        session_data = call_args[0][1]
        self.assertIn("components", session_data)
        self.assertIn("cluster_info", session_data)
        self.assertIn("for-join", session_data["cluster_info"])
        self.assertEqual(session_data["mode"], "master")

    def test_start_all_calls_save_session_on_success(self):
        config = {"comp_a": True}
        engine = _make_launcher_with_mocked_components(config)

        with patch.object(engine, 'prepare_environment'), \
             patch.object(engine, 'load_components'), \
             patch.object(engine, 'get_start_order', return_value=["comp_a"]), \
             patch.object(engine, 'start_components_in_order', return_value=True), \
             patch.object(engine, 'save_session') as mock_save:
            result = engine.start_all()
            self.assertTrue(result)
            mock_save.assert_called_once()

    def test_start_all_skips_save_session_on_failure(self):
        config = {"comp_a": True}
        engine = _make_launcher_with_mocked_components(config)

        with patch.object(engine, 'prepare_environment'), \
             patch.object(engine, 'load_components'), \
             patch.object(engine, 'get_start_order', return_value=["comp_a"]), \
             patch.object(engine, 'start_components_in_order', return_value=False), \
             patch.object(engine, 'save_session') as mock_save:
            result = engine.start_all()
            self.assertFalse(result)
            mock_save.assert_not_called()


if __name__ == "__main__":
    unittest.main()
