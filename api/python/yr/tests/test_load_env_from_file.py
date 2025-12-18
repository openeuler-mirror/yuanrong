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

"""Unit tests for load_env_from_file function."""

import json
import os
import tempfile
import unittest
from unittest.mock import patch

from yr.common import utils


class TestLoadEnvFromFile(unittest.TestCase):
    """Test cases for load_env_from_file function."""

    def setUp(self):
        """Set up test fixtures."""
        # Save original environment variables that might be modified
        self.original_env = os.environ.copy()
        # Clear test environment variables
        test_keys = ["TEST_KEY1", "TEST_KEY2", "TEST_KEY3", "TEST_NUM", "TEST_BOOL"]
        for key in test_keys:
            if key in os.environ:
                del os.environ[key]

    def tearDown(self):
        """Clean up after tests."""
        # Restore original environment
        os.environ.clear()
        os.environ.update(self.original_env)

    def test_load_env_from_file(self):
        """Test loading basic environment variables from JSON file."""
        env_data = {
            "TEST_KEY1": "value1",
            "TEST_KEY2": "value2",
            "TEST_KEY3": "value with spaces"
        }

        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
            json.dump(env_data, f)
            env_file_path = f.name

        try:
            utils.load_env_from_file(env_file_path)

            self.assertEqual(os.environ.get("TEST_KEY1"), "value1")
            self.assertEqual(os.environ.get("TEST_KEY2"), "value2")
            self.assertEqual(os.environ.get("TEST_KEY3"), "value with spaces")
        finally:
            os.unlink(env_file_path)


if __name__ == "__main__":
    unittest.main()
