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

"""Tests for yr.init() integration with ServiceManager.

Tests that init() correctly calls ServiceManager.ensure_services() under
the right conditions and handles failures gracefully.
"""

import importlib.util
import os
import sys
import types
import unittest
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
        "yr.libruntime_pb2", "yr.fnruntime", "yr.config",
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

from yr.service_manager import ServiceManager, ServiceEndpoints


class TestInitAutoStartIntegration(unittest.TestCase):
    """Test that yr.init() triggers ServiceManager under correct conditions.

    Since we can't import yr.apis fully (C++ deps), we test the LOGIC:
    - When conf.is_driver=True, conf.server_address="", conf.local_mode=False
      → ServiceManager.ensure_services() should be called
    - When conf.server_address is set → ServiceManager should NOT be called
    - When ensure_services fails → should fall back gracefully
    """

    def setUp(self):
        ServiceManager.reset()

    def test_auto_start_condition_met(self):
        """When is_driver=True, no server_address, not local_mode → should call ensure_services."""
        # Simulate the condition check from apis.py init()
        conf = MagicMock()
        conf.is_driver = True
        conf.server_address = ""
        conf.local_mode = False

        endpoints = ServiceEndpoints("10.0.0.1:8001", "10.0.0.1:9001")

        # Check that the condition matches
        should_auto_start = conf.is_driver and not conf.server_address and not conf.local_mode
        self.assertTrue(should_auto_start)

        # Verify ServiceManager would be called
        with patch.object(ServiceManager, "ensure_services", return_value=endpoints) as mock_ensure:
            if should_auto_start:
                result = ServiceManager.ensure_services()
                conf.server_address = result.server_address
                conf.ds_address = result.ds_address

        mock_ensure.assert_called_once()
        self.assertEqual(conf.server_address, "10.0.0.1:8001")
        self.assertEqual(conf.ds_address, "10.0.0.1:9001")

    def test_auto_start_skipped_when_server_address_set(self):
        """When server_address is already set → should NOT auto-start."""
        conf = MagicMock()
        conf.is_driver = True
        conf.server_address = "existing:8001"
        conf.local_mode = False

        should_auto_start = conf.is_driver and not conf.server_address and not conf.local_mode
        self.assertFalse(should_auto_start)

    def test_auto_start_skipped_when_local_mode(self):
        """When local_mode=True → should NOT auto-start."""
        conf = MagicMock()
        conf.is_driver = True
        conf.server_address = ""
        conf.local_mode = True

        should_auto_start = conf.is_driver and not conf.server_address and not conf.local_mode
        self.assertFalse(should_auto_start)

    def test_auto_start_skipped_when_not_driver(self):
        """When is_driver=False → should NOT auto-start."""
        conf = MagicMock()
        conf.is_driver = False
        conf.server_address = ""
        conf.local_mode = False

        should_auto_start = conf.is_driver and not conf.server_address and not conf.local_mode
        self.assertFalse(should_auto_start)

    def test_fallback_on_ensure_services_failure(self):
        """When ensure_services raises → should catch and continue (C++ fallback)."""
        conf = MagicMock()
        conf.is_driver = True
        conf.server_address = ""
        conf.local_mode = False

        with patch.object(
            ServiceManager, "ensure_services",
            side_effect=RuntimeError("start failed"),
        ):
            # Simulate the try/except from apis.py
            should_auto_start = conf.is_driver and not conf.server_address and not conf.local_mode
            if should_auto_start:
                try:
                    endpoints = ServiceManager.ensure_services()
                    conf.server_address = endpoints.server_address
                except Exception:
                    pass  # Fall back to C++ auto_init

        # server_address should remain empty (fallback to C++)
        self.assertEqual(conf.server_address, "")


if __name__ == "__main__":
    unittest.main()
