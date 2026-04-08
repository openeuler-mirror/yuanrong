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

"""Process utilities for fate-sharing, liveness checks, and port probing."""

import ctypes
import logging
import os
import signal
import socket

logger = logging.getLogger(__name__)


def set_pdeathsig() -> None:
    """Set PR_SET_PDEATHSIG so this process receives SIGTERM when its parent exits.

    Intended for use as ``subprocess.Popen(preexec_fn=set_pdeathsig)``.
    Linux-only; silently does nothing on other platforms.
    """
    try:
        if not hasattr(ctypes, "CDLL"):
            return
        libc = ctypes.CDLL("libc.so.6", use_errno=True)
        PR_SET_PDEATHSIG = 1
        result = libc.prctl(PR_SET_PDEATHSIG, signal.SIGTERM)
        if result != 0:
            logger.warning("prctl(PR_SET_PDEATHSIG) returned %d", result)
    except OSError:
        logger.debug("PR_SET_PDEATHSIG not available on this platform")


def is_process_alive(pid: int) -> bool:
    """Check whether *pid* refers to a running process."""
    try:
        os.kill(pid, 0)
        return True
    except ProcessLookupError:
        return False
    except PermissionError:
        # Process exists but we don't own it — still alive
        return True


def is_port_reachable(host: str, port: int, timeout: float = 1.0) -> bool:
    """Return True if a TCP connection to *host*:*port* succeeds."""
    try:
        with socket.create_connection((host, port), timeout=timeout):
            return True
    except (ConnectionRefusedError, TimeoutError, OSError):
        return False
