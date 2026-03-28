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

"""Sandbox implementation for isolated code execution."""

import argparse
import subprocess
import tempfile
import os
from typing import Optional, Dict, Any, List, TYPE_CHECKING

import yr
from yr.config_manager import ConfigManager

if TYPE_CHECKING:
    from yr.config import PortForwarding


def _sanitize_instance_id(instance_id: str) -> str:
    """Sanitize instance ID to match TraefikRegistry::SanitizeID (C++).

    Rules: @ -> -at-, / . _ -> -, truncate to 200 chars.
    """
    result = instance_id
    pos = 0
    while True:
        pos = result.find("@", pos)
        if pos == -1:
            break
        result = result[:pos] + "-at-" + result[pos + 1:]
        pos += 4
    result = result.replace("/", "-").replace(".", "-").replace("_", "-")
    if len(result) > 200:
        result = result[:200]
    return result


def _get_gateway_host() -> str:
    """Get Gateway host from YR_GATEWAY_ADDRESS or YR_SERVER_ADDRESS."""
    host = os.environ.get("YR_GATEWAY_ADDRESS", "").strip()
    if host:
        return host
    addr = os.environ.get("YR_SERVER_ADDRESS", "").strip()
    if addr:
        return addr
    return ConfigManager().server_address.strip()


def _build_gateway_url(instance_id: str, sandbox_port: int, gateway_host: str, path: str = "") -> str:
    """Build Gateway HTTP path URL: https://{gateway_host}/{safeID}/{sandbox_port}{path}.

    URL format must match TraefikRegistry::RegisterInstance in function_proxy:
    - {safeID} is sanitized instance ID (SanitizeID logic)
    - {sandbox_port} is the original sandbox port
    - Full path format: /{safeID}/{sandbox_port}

    See: functionsystem/src/function_proxy/local_scheduler/traefik_registry/traefik_registry.cpp
    """
    safe_id = _sanitize_instance_id(instance_id)
    base = f"https://{gateway_host}/{safe_id}/{sandbox_port}"
    if path:
        path = path if path.startswith("/") else f"/{path}"
        return f"{base}{path}"
    return base


def _print_gateway_urls(instance_id: str, port_forwardings: List["PortForwarding"]) -> None:
    """Print Gateway URLs for port forwardings after sandbox creation."""
    if not port_forwardings:
        return
    gateway_host = _get_gateway_host()
    if not gateway_host:
        print("Warning: cannot print port forwarding URLs: YR_GATEWAY_ADDRESS or YR_SERVER_ADDRESS not set")
        return
    print("sandbox created, port forwarding URLs:")
    for pf in port_forwardings:
        url = _build_gateway_url(instance_id, pf.port, gateway_host)
        print(f"  port {pf.port}: {url}")


@yr.instance
class SandboxInstance:
    """
    SandboxInstance class provides isolated environment for code execution.

    This class creates a sandboxed environment where code can be executed
    with limited permissions and resource constraints.

    This is the underlying instance class decorated with @yr.instance.
    Users should typically use the SandBox wrapper class instead.
    """

    def __init__(
        self, working_dir: Optional[str] = None, env: Optional[Dict[str, str]] = None
    ):
        """
        Initialize the SandBox instance.

        Args:
            working_dir (Optional[str]): The working directory for sandbox execution.
                If None, a temporary directory will be created.
            env (Optional[Dict[str, str]]): Environment variables for the sandbox.
                If None, inherits from parent process.
        """
        if working_dir is None:
            self.working_dir = tempfile.mkdtemp(prefix="yr_sandbox_")
            self._temp_dir_created = True
        else:
            self.working_dir = working_dir
            self._temp_dir_created = False

        self.env = env if env is not None else os.environ.copy()
        self._initialized = True

    def execute(self, command: str, timeout: Optional[int] = None) -> Dict[str, Any]:
        """
        Execute a command in the sandbox environment.

        Args:
            command (str): The command to execute.
            timeout (Optional[int]): Timeout in seconds for command execution.
                If None, no timeout is set.

        Returns:
            Dict[str, Any]: A dictionary containing:
                - returncode (int): The return code of the command.
                - stdout (str): Standard output of the command.
                - stderr (str): Standard error of the command.

        Raises:
            RuntimeError: If the sandbox is not initialized.
            subprocess.TimeoutExpired: If the command execution times out.

        Examples:
            >>> sandbox = yr.sandbox.SandBox.invoke()
            >>> result = yr.get(sandbox.execute.invoke("ls -la"))
            >>> print(result['stdout'])
        """
        if not self._initialized:
            raise RuntimeError("SandBox is not initialized")

        try:
            result = subprocess.run(
                command,
                shell=True,
                cwd=self.working_dir,
                env=self.env,
                capture_output=True,
                text=True,
                timeout=timeout,
            )
            import sys
            return {
                "returncode": result.returncode,
                "stdout": sys.version + '\n' + result.stdout,
                "stderr": result.stderr,
            }
        except subprocess.TimeoutExpired as e:
            return {
                "returncode": -1,
                "stdout": e.stdout.decode() if e.stdout else "",
                "stderr": f"Command timed out after {timeout} seconds",
            }
        except Exception as e:
            return {"returncode": -1, "stdout": "", "stderr": str(e)}

    def get_working_dir(self) -> str:
        """
        Get the working directory of the sandbox.

        Returns:
            str: The path to the working directory.
        """
        return self.working_dir

    def cleanup(self) -> None:
        """
        Cleanup the sandbox environment.

        This method removes temporary files and directories created by the sandbox.
        """
        # Use getattr to safely handle proxy objects that may not have these attributes
        temp_dir_created = getattr(self, "_temp_dir_created", False)
        working_dir = getattr(self, "working_dir", None)

        if temp_dir_created and working_dir and os.path.exists(working_dir):
            import shutil

            try:
                shutil.rmtree(working_dir)
            except Exception as e:
                # Log the error but don't raise
                print(
                    f"Warning: Failed to cleanup sandbox directory {working_dir}: {e}"
                )

    def get_name(self):
        """
        Get the name of the sandbox instance.

        Returns:
            str: The name of the sandbox instance.
        """
        return os.environ.get("INSTANCE_ID", "")

    def start_tunnel_server(self, ws_port: int = 8765, http_port: int = 8766) -> None:
        """Start TunnelServer in a background thread within this sandbox instance."""
        import asyncio
        import threading
        import time

        from yr.sandbox.tunnel_server import TunnelServer

        def _run():
            loop = asyncio.new_event_loop()
            asyncio.set_event_loop(loop)
            server = TunnelServer(ws_port=ws_port, http_port=http_port)
            loop.run_until_complete(server.start())
            loop.run_forever()

        t = threading.Thread(target=_run, name="tunnel-server", daemon=True)
        t.start()
        # Wait until both ports are actually bound (up to 5s)
        import socket as _socket
        deadline = time.time() + 5.0
        for port in (ws_port, http_port):
            while time.time() < deadline:
                try:
                    _socket.create_connection(("127.0.0.1", port), timeout=0.1).close()
                    break
                except OSError:
                    time.sleep(0.1)

    def __del__(self):
        """Destructor to ensure cleanup on object deletion."""
        self.cleanup()


def create(
    working_dir: Optional[str] = None,
    env: Optional[Dict[str, str]] = None,
    port_forwardings: Optional[List["PortForwarding"]] = None,
    upstream: Optional[str] = None,
    proxy_port: int = 8766,
):
    """
    Create a new SandBox instance.

    Args:
        working_dir: Working directory for sandbox execution.
        env: Environment variables for the sandbox.
        port_forwardings: Additional port forwarding rules.
        upstream: Local service address to tunnel to, e.g. "192.168.3.45:8000".
            When set, starts a reverse tunnel so sandbox code can reach the
            local service via http://127.0.0.1:{proxy_port}.
        proxy_port: Port B — the HTTP proxy port inside the sandbox
            (default 8766). Port A (WS tunnel) = proxy_port - 1.

    Returns:
        SandBox wrapper instance.
    """
    return SandBox(working_dir, env, port_forwardings, upstream=upstream, proxy_port=proxy_port)


class SandBox:
    """
    SandBox wrapper class for convenient sandbox operations.

    When upstream is provided, starts a reverse tunnel:
    - Port B (proxy_port, loopback): sandbox code calls http://127.0.0.1:{proxy_port}
    - Port A (proxy_port-1, 0.0.0.0): WebSocket tunnel endpoint registered with Traefik

    Examples:
        >>> import yr
        >>> yr.init()
        >>>
        >>> # Basic sandbox
        >>> sb = yr.sandbox.SandBox()
        >>> result = yr.get(sb.exec("echo hello"))
        >>> print(result['stdout'])
        >>>
        >>> # Sandbox with reverse tunnel to local service
        >>> sb = yr.sandbox.create(upstream="192.168.3.45:8000")
        >>> url = sb.get_tunnel_url()   # "http://127.0.0.1:8766"
        >>> result = yr.get(sb.exec(f"curl {url}/api/data"))
        >>>
        >>> sb.terminate()
        >>> yr.finalize()
    """

    def __init__(
        self,
        working_dir: Optional[str] = None,
        env: Optional[Dict[str, str]] = None,
        port_forwardings: Optional[List["PortForwarding"]] = None,
        upstream: Optional[str] = None,
        proxy_port: int = 8766,
    ):
        self._proxy_port = proxy_port
        self._tunnel_client = None

        opt = yr.InvokeOptions()
        opt.skip_serialize = True

        if upstream is not None:
            tunnel_port = proxy_port - 1
            tunnel_pf = yr.PortForwarding(port=tunnel_port)
            all_pf = (list(port_forwardings) if port_forwardings else []) + [tunnel_pf]
            opt.port_forwardings = all_pf
            self._instance = SandboxInstance.options(opt).invoke(working_dir, env)
            if port_forwardings:
                instance_id = yr.get(self._instance.get_name.invoke())
                _print_gateway_urls(instance_id, port_forwardings)
            # Start tunnel server inside sandbox as a background thread
            yr.get(self._instance.start_tunnel_server.invoke(tunnel_port, proxy_port))
            # Build WSS URL for tunnel Port A via Traefik
            instance_id = yr.get(self._instance.get_name.invoke())
            gateway_host = _get_gateway_host()
            tunnel_url = _build_gateway_url(instance_id, tunnel_port, gateway_host)
            tunnel_ws_url = tunnel_url.replace("https://", "wss://").replace("http://", "ws://")
            # Start local TunnelClient in background thread
            from yr.sandbox.tunnel_client import TunnelClient
            self._tunnel_client = TunnelClient(upstream)
            self._tunnel_client.start(tunnel_ws_url)
        else:
            if port_forwardings:
                opt.port_forwardings = port_forwardings
            self._instance = SandboxInstance.options(opt).invoke(working_dir, env)
            if port_forwardings:
                instance_id = yr.get(self._instance.get_name.invoke())
                _print_gateway_urls(instance_id, port_forwardings)

    def exec(self, command: str, timeout: Optional[int] = None):
        """Execute a command in the sandbox. Returns ObjectRef; unwrap with yr.get()."""
        return self._instance.execute.invoke(command, timeout)

    def get_working_dir(self):
        """Get the working directory of the sandbox."""
        return self._instance.get_working_dir.invoke()

    def cleanup(self):
        """Cleanup temp files in the sandbox."""
        return self._instance.cleanup.invoke()

    def terminate(self):
        """Stop tunnel client (if any) and terminate the sandbox instance."""
        if self._tunnel_client is not None:
            self._tunnel_client.stop()
            self._tunnel_client = None
        self._instance.terminate()

    def __del__(self):
        try:
            if hasattr(self, "_instance") and self._instance is not None:
                yr.get(self.cleanup())
                self.terminate()
        except Exception:
            pass

    def get_tunnel_url(self) -> str:
        """Return the internal HTTP proxy URL for sandbox code to call.

        Returns:
            str: e.g. "http://127.0.0.1:8766"
        Raises:
            RuntimeError: if no upstream was configured.
        """
        if self._tunnel_client is None:
            raise RuntimeError("No upstream configured. Pass upstream= to create().")
        return f"http://127.0.0.1:{self._proxy_port}"


def main():
    parser = argparse.ArgumentParser(description="Create a detached sandbox instance")
    parser.add_argument(
        "--name", type=str, default=None, help="Name of the sandbox instance"
    )
    parser.add_argument(
        "--namespace",
        type=str,
        default="detached.sandbox",
        help="Namespace for the sandbox instance",
    )
    args = parser.parse_args()
    os.environ.pop("YR_WORKING_DIR", None)

    cfg = yr.Config()
    cfg.in_cluster = True
    yr.init(cfg)
    try:
        opt = yr.InvokeOptions()
        opt.custom_extensions["lifecycle"] = "detached"
        opt.idle_timeout = 60 * 60 * 24 * 7
        opt.cpu = 1000
        opt.memory = 2048
        opt.name = args.name
        opt.namespace = args.namespace
        opt.skip_serialize = True  # Skip serialization for pre-deployed SDK class
        if not opt.name:
            import uuid
            opt.name = str(uuid.uuid4())

        sandbox = SandboxInstance.options(opt).invoke()
        try:
            name = yr.get(sandbox.get_name.invoke())
            print(f"sandbox created, instance_name={name}")
        except Exception as e:
            print(f"sandbox create failed, name={opt.name}, error={e}")
    finally:
        yr.finalize()


if __name__ == "__main__":
    main()
