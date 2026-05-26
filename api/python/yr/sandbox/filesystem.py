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

"""Sandbox filesystem copy operations.

All actual data transfer is delegated to :mod:`yr.cli.exec` so the transport
layer is not duplicated:

* :func:`yr.cli.exec.copy_to_remote` / :func:`~yr.cli.exec.copy_to_remote_streaming`
* :func:`yr.cli.exec.copy_from_remote` / :func:`~yr.cli.exec.copy_from_remote_streaming`
* :func:`yr.cli.exec.choose_cp_mode` – auto mode selection
"""

import asyncio
import os
from enum import Enum
from typing import Optional, TYPE_CHECKING

if TYPE_CHECKING:
    from yr.sandbox.sandbox import Sandbox


class CpDirection(Enum):
    """Transfer direction for sandbox copy operations.

    ``UPLOAD``   – copy from local filesystem **into** the sandbox (default).
    ``DOWNLOAD`` – copy from the sandbox **out** to the local filesystem.
    """

    UPLOAD = "upload"
    DOWNLOAD = "download"


def _get_gateway_host() -> str:
    """Return the faasfrontend host:port for exec WebSocket connections.

    Resolution order:
    1. ``YR_GATEWAY_ADDRESS`` environment variable
    2. ``YR_SERVER_ADDRESS`` environment variable
    3. :class:`~yr.config_manager.ConfigManager` ``server_address``
    """
    from yr.config_manager import ConfigManager

    host = os.environ.get("YR_GATEWAY_ADDRESS", "").strip()
    if host:
        return host
    addr = os.environ.get("YR_SERVER_ADDRESS", "").strip()
    if addr:
        return addr
    return ConfigManager().server_address.strip()


class SandboxFilesystem:
    """Namespace for sandbox filesystem copy operations.

    Access via :attr:`~yr.sandbox.sandbox.Sandbox.filesystem`.

    Transport is fully delegated to :mod:`yr.cli.exec` — no duplicate
    implementation of tar/gzip streaming logic.

    Example::

        sb = yr.sandbox.create()

        # Upload a local file into the sandbox
        sb.filesystem.copy_from_local("/local/data.csv", "/sandbox/data.csv")

        # Download a file from the sandbox to a local path
        sb.filesystem.copy_to_local("/sandbox/output.txt", "/local/output.txt")
    """

    def __init__(self, sandbox: "Sandbox") -> None:
        self._sandbox = sandbox

    def _get_connection(self):
        """Resolve ``(host, port, instance_id)`` for the exec WebSocket.

        The exec WebSocket is served by faasfrontend, so the address is
        resolved via :func:`_get_gateway_host`, not the bus address used by
        :func:`yr.init`.

        Raises:
            RuntimeError: If no server address is configured.
        """
        import yr

        gateway_addr = _get_gateway_host()
        if not gateway_addr:
            raise RuntimeError(
                "Server address is not configured. "
                "Set YR_SERVER_ADDRESS (faasfrontend address) or call yr.init()."
            )
        host, port = gateway_addr.rsplit(":", 1)
        instance_id = yr.get(self._sandbox._instance.get_name.invoke())
        return host, port, instance_id

    def _cp(
        self,
        src: str,
        dst: str,
        direction: CpDirection = CpDirection.UPLOAD,
        streaming: Optional[bool] = None,
    ) -> None:
        """Core copy implementation; delegates to :mod:`yr.cli.exec` transport.

        Args:
            src:       Source path.
            dst:       Destination path.
            direction: ``UPLOAD`` → src is local, dst is sandbox path.
                       ``DOWNLOAD`` → src is sandbox path, dst is local.
            streaming: ``None`` = auto-select, ``True`` = gzip streaming,
                       ``False`` = non-streaming tar.

        Raises:
            FileNotFoundError: Local source not found (upload only).
            RuntimeError: Server address not configured.
        """
        from yr.cli.exec import (
            copy_to_remote,
            copy_from_remote,
            copy_to_remote_streaming,
            copy_from_remote_streaming,
            choose_cp_mode,
        )

        upload = direction == CpDirection.UPLOAD
        local_path = src if upload else dst
        remote_path = dst if upload else src

        if upload and not os.path.exists(local_path):
            raise FileNotFoundError(f"Local source path not found: {local_path}")

        host, port, instance_id = self._get_connection()

        if streaming is None:
            streaming = choose_cp_mode(local_path, remote_path, upload=upload)

        if upload:
            fn = copy_to_remote_streaming if streaming else copy_to_remote
            asyncio.run(
                fn(
                    host=host,
                    port=port,
                    instance=instance_id,
                    local_path=local_path,
                    remote_path=remote_path,
                )
            )
        else:
            fn = copy_from_remote_streaming if streaming else copy_from_remote
            asyncio.run(
                fn(
                    host=host,
                    port=port,
                    instance=instance_id,
                    remote_path=remote_path,
                    local_path=local_path,
                )
            )

    def copy_from_local(
        self,
        local_path: str,
        remote_path: str,
        streaming: Optional[bool] = None,
    ) -> None:
        """Copy a local file or directory **into** the sandbox.

        Args:
            local_path:  Absolute or relative path on the **local** machine.
            remote_path: Absolute path inside the **sandbox**.
            streaming:   ``None`` = auto, ``True`` = gzip streaming,
                         ``False`` = non-streaming tar.

        Raises:
            FileNotFoundError: *local_path* does not exist.
            RuntimeError: Server address is not configured.
        """
        self._cp(local_path, remote_path, CpDirection.UPLOAD, streaming)

    def copy_to_local(
        self,
        remote_path: str,
        local_path: str,
        streaming: Optional[bool] = None,
    ) -> None:
        """Copy a file or directory **from** the sandbox to the local machine.

        Args:
            remote_path: Absolute path inside the **sandbox**.
            local_path:  Absolute or relative path on the **local** machine.
            streaming:   ``None`` = auto, ``True`` = gzip streaming,
                         ``False`` = non-streaming tar.

        Raises:
            RuntimeError: Server address is not configured.
        """
        self._cp(remote_path, local_path, CpDirection.DOWNLOAD, streaming)
