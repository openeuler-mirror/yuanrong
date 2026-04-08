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

import importlib
import os
import socket
import subprocess
import sys
import time
import unittest


def _import_process_utils():
    """Import process_utils without triggering yr.__init__ (which needs C++ extensions).

    This allows running tests in environments where the full yr package
    is not built/installed (e.g., pure-Python unit testing).
    """
    module_path = os.path.join(os.path.dirname(__file__), "..", "yr", "process_utils.py")
    spec = importlib.util.spec_from_file_location("yr.process_utils", module_path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


_pu = _import_process_utils()
set_pdeathsig = _pu.set_pdeathsig
is_process_alive = _pu.is_process_alive
is_port_reachable = _pu.is_port_reachable


class TestIsProcessAlive(unittest.TestCase):
    def test_current_process_is_alive(self):
        self.assertTrue(is_process_alive(os.getpid()))

    def test_nonexistent_pid_is_not_alive(self):
        # PID 2^22 is very unlikely to exist
        self.assertFalse(is_process_alive(4194304))

    def test_dead_child_process_is_not_alive(self):
        p = subprocess.Popen([sys.executable, "-c", "pass"])
        p.wait()
        time.sleep(0.1)
        self.assertFalse(is_process_alive(p.pid))


class TestIsPortReachable(unittest.TestCase):
    def test_reachable_port(self):
        server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        server.bind(("127.0.0.1", 0))
        server.listen(1)
        port = server.getsockname()[1]
        try:
            self.assertTrue(is_port_reachable("127.0.0.1", port, timeout=1.0))
        finally:
            server.close()

    def test_unreachable_port(self):
        self.assertFalse(is_port_reachable("127.0.0.1", 1, timeout=0.1))


class TestSetPdeathsig(unittest.TestCase):
    def test_set_pdeathsig_does_not_raise(self):
        # Use direct file import to avoid yr.__init__ in subprocess too
        script = (
            "import importlib.util, os; "
            f"spec = importlib.util.spec_from_file_location('pu', {repr(os.path.join(os.path.dirname(__file__), '..', 'yr', 'process_utils.py'))}); "
            "mod = importlib.util.module_from_spec(spec); "
            "spec.loader.exec_module(mod); "
            "mod.set_pdeathsig()"
        )
        p = subprocess.Popen(
            [sys.executable, "-c", script],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        )
        _, stderr = p.communicate(timeout=5)
        self.assertEqual(p.returncode, 0, f"set_pdeathsig failed: {stderr.decode()}")

    @unittest.skipUnless(sys.platform == "linux", "PR_SET_PDEATHSIG is Linux-only")
    def test_fate_sharing_kills_child_on_parent_exit(self):
        """Verify child gets SIGTERM when parent exits."""
        pu_path = os.path.join(os.path.dirname(__file__), "..", "yr", "process_utils.py")
        parent_script = f'''
import subprocess, sys, importlib.util

def _load_set_pdeathsig():
    spec = importlib.util.spec_from_file_location("pu", {repr(pu_path)})
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod.set_pdeathsig

child = subprocess.Popen(
    [sys.executable, "-c", "import time; time.sleep(60)"],
    preexec_fn=_load_set_pdeathsig(),
)
print(child.pid, flush=True)
# Parent exits immediately — child should get SIGTERM
'''
        parent = subprocess.Popen(
            [sys.executable, "-c", parent_script],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        )
        stdout, _ = parent.communicate(timeout=5)
        child_pid = int(stdout.strip())
        time.sleep(0.5)
        try:
            os.kill(child_pid, 0)
            alive = True
        except ProcessLookupError:
            alive = False
        self.assertFalse(alive, f"Child PID {child_pid} should be dead after parent exit")


if __name__ == "__main__":
    unittest.main()
