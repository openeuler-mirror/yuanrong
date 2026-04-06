# api/python/yr/sandbox/tunnel_client.py
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
"""Tunnel client: runs locally in a background thread.

Connects to Port A (WS) of TunnelServer, receives request frames,
forwards them to the local upstream service, and sends response frames back.
"""
import asyncio
import base64
import logging
import os
import random
import ssl
import threading
import time
from typing import Optional

import httpx
import websockets

from yr.sandbox.tunnel_protocol import (
    parse_frame,
    HttpReqFrame, HttpRespFrame,
    WsConnectFrame, WsConnectedFrame, WsMessageFrame, WsCloseFrame, ErrorFrame,
    PingFrame, PongFrame, make_id,
)

logger = logging.getLogger(__name__)

_WS_CHANNEL_QUEUE_MAX = 100


def _ssl_verify_enabled() -> bool:
    """Check whether SSL verification is enabled (default: True).

    Set TUNNEL_SSL_VERIFY=0 to disable.
    """
    return os.environ.get("TUNNEL_SSL_VERIFY", "1") not in ("0", "false", "")


class TunnelClient:
    def __init__(
        self,
        upstream: str,
        ping_interval: float = 30.0,
        ping_timeout: float = 10.0,
        reconnect_base_delay: float = 1.0,
        reconnect_max_delay: float = 60.0,
    ):
        """
        Args:
            upstream: upstream service address, e.g. "192.168.3.45:8000" or
                      "http://192.168.3.45:8000" or "https://...".
            ping_interval: seconds between heartbeat PingFrames.
            ping_timeout: seconds to wait for PongFrame before closing.
            reconnect_base_delay: base delay for exponential backoff (Task 4).
            reconnect_max_delay: max delay for exponential backoff (Task 4).
        """
        if "://" not in upstream:
            upstream = f"http://{upstream}"
        self._upstream = upstream.rstrip("/")
        self._ping_interval = ping_interval
        self._ping_timeout = ping_timeout
        self._reconnect_base_delay = reconnect_base_delay
        self._reconnect_max_delay = reconnect_max_delay
        self._current_ping_id: Optional[str] = None
        self._pong_event: Optional[asyncio.Event] = None
        self._tunnel_url: Optional[str] = None
        self._thread: Optional[threading.Thread] = None
        self._loop: Optional[asyncio.AbstractEventLoop] = None
        self._main_task: Optional[asyncio.Task] = None
        self._stop_event = threading.Event()
        self._ws_channels: dict = {}  # channel_id -> asyncio.Queue
        self._ssl_verify = _ssl_verify_enabled()

    def start(self, tunnel_url: str) -> None:
        """Start the client in a background daemon thread."""
        self._tunnel_url = tunnel_url
        self._thread = threading.Thread(target=self._run, name="tunnel-client", daemon=True)
        self._thread.start()

    def stop(self) -> None:
        """Stop the client and wait for the thread to finish."""
        self._stop_event.set()
        if self._loop and not self._loop.is_closed():
            self._loop.call_soon_threadsafe(self._cancel_main_task)
        if self._thread:
            self._thread.join(timeout=5)

    def _cancel_main_task(self) -> None:
        if self._main_task and not self._main_task.done():
            self._main_task.cancel()

    def _run(self) -> None:
        self._loop = asyncio.new_event_loop()
        asyncio.set_event_loop(self._loop)
        try:
            self._main_task = self._loop.create_task(self._connect_loop())
            self._loop.run_until_complete(self._main_task)
        except asyncio.CancelledError:
            pass
        finally:
            pending = [task for task in asyncio.all_tasks(self._loop) if not task.done()]
            for task in pending:
                task.cancel()
            if pending:
                self._loop.run_until_complete(asyncio.gather(*pending, return_exceptions=True))
            self._loop.run_until_complete(self._loop.shutdown_asyncgens())
            self._loop.close()

    async def _connect_loop(self) -> None:
        attempt = 0
        while not self._stop_event.is_set():
            try:
                ssl_ctx = self._make_ssl_context()
                async with websockets.connect(
                    self._tunnel_url,
                    ssl=ssl_ctx,
                    ping_interval=30,
                    ping_timeout=10,
                ) as ws:
                    logger.info("Connected to tunnel: %s", self._tunnel_url)
                    attempt = 0  # reset on successful connect
                    async with httpx.AsyncClient(
                        base_url=self._upstream,
                        verify=self._ssl_verify,
                        timeout=httpx.Timeout(10.0),
                    ) as http:
                        await self._recv_loop(ws, http)
            except Exception as e:
                if self._stop_event.is_set():
                    break
                logger.warning("Tunnel disconnected (%s), reconnecting...", e)

            # Increment attempt after disconnect (before backoff calculation)
            attempt += 1

            # Cleanup stale WS channels on disconnect
            self._cleanup_ws_channels()

            if not self._stop_event.is_set():
                delay = min(
                    self._reconnect_base_delay * (2 ** attempt),
                    self._reconnect_max_delay,
                ) + random.random() * min(self._reconnect_base_delay, 1.0)
                logger.info("Reconnecting in %.1fs (attempt %d)", delay, attempt + 1)
                await asyncio.sleep(delay)

    def _cleanup_ws_channels(self) -> None:
        """Cancel in-flight WS proxy tasks and clear channels."""
        self._ws_channels.clear()

    def _make_ssl_context(self) -> Optional[ssl.SSLContext]:
        if self._tunnel_url and self._tunnel_url.startswith("wss://"):
            if self._ssl_verify:
                return None  # use websockets default SSL verification
            ctx = ssl.create_default_context()
            ctx.check_hostname = False
            ctx.verify_mode = ssl.CERT_NONE
            logger.warning(
                "TunnelClient: SSL certificate verification disabled "
                "via TUNNEL_SSL_VERIFY=0."
            )
            return ctx
        return None

    async def _recv_loop(self, ws, http: httpx.AsyncClient) -> None:
        """Orchestrate recv and heartbeat tasks. First to exit triggers cancel of the other."""
        self._pong_event = asyncio.Event()
        recv_task = asyncio.create_task(self._recv_frames(ws, http))
        hb_task = asyncio.create_task(self._heartbeat_loop(ws))
        done, pending = await asyncio.wait(
            [recv_task, hb_task], return_when=asyncio.FIRST_COMPLETED
        )
        for t in pending:
            t.cancel()
            try:
                await t
            except asyncio.CancelledError:
                pass

    async def _recv_frames(self, ws, http: httpx.AsyncClient) -> None:
        """Receive frames and dispatch them."""
        async for message in ws:
            try:
                frame = parse_frame(message)
            except Exception as e:
                logger.warning("Dropping malformed tunnel frame: %s", e)
                continue
            if isinstance(frame, PongFrame) and frame.id == self._current_ping_id:
                self._pong_event.set()
            elif isinstance(frame, HttpReqFrame):
                asyncio.create_task(self._handle_http(ws, http, frame))
            elif isinstance(frame, WsConnectFrame):
                asyncio.create_task(self._handle_ws_connect(ws, frame))
            elif isinstance(frame, (WsMessageFrame, WsCloseFrame)):
                q = self._ws_channels.get(frame.id)
                if q is not None:
                    await q.put(frame)

    async def _heartbeat_loop(self, ws) -> None:
        """Send PingFrame periodically. Close ws on pong timeout."""
        while True:
            await asyncio.sleep(self._ping_interval)
            if self._stop_event.is_set():
                return
            self._current_ping_id = make_id()
            self._pong_event.clear()
            ping = PingFrame(id=self._current_ping_id, timestamp=time.time())
            try:
                await ws.send(ping.to_json())
                await asyncio.wait_for(self._pong_event.wait(), timeout=self._ping_timeout)
            except (asyncio.TimeoutError, websockets.ConnectionClosed):
                logger.warning("Heartbeat timeout, closing connection")
                await ws.close()
                return

    async def _handle_http(self, ws, http: httpx.AsyncClient, frame: HttpReqFrame) -> None:
        try:
            resp = await http.request(
                method=frame.method,
                url=frame.path,
                headers=frame.headers,
                content=frame.body,
            )
            resp_frame = HttpRespFrame(
                id=frame.id, status=resp.status_code,
                headers=dict(resp.headers), body=resp.content,
            )
        except Exception as e:
            resp_frame = ErrorFrame(id=frame.id, message=str(e))
        await ws.send(resp_frame.to_json())

    async def _handle_ws_connect(self, ws, frame: WsConnectFrame) -> None:
        upstream_ws_url = (
            self._upstream
            .replace("http://", "ws://")
            .replace("https://", "wss://")
        ) + frame.path
        queue: asyncio.Queue = asyncio.Queue(maxsize=_WS_CHANNEL_QUEUE_MAX)
        self._ws_channels[frame.id] = queue
        try:
            async with websockets.connect(upstream_ws_url) as upstream_ws:
                await ws.send(WsConnectedFrame(id=frame.id).to_json())

                async def from_upstream():
                    async for msg in upstream_ws:
                        if isinstance(msg, str):
                            await ws.send(
                                WsMessageFrame(id=frame.id, data=msg, binary=False).to_json()
                            )
                        else:
                            await ws.send(
                                WsMessageFrame(
                                    id=frame.id,
                                    data=base64.b64encode(msg).decode(),
                                    binary=True,
                                ).to_json()
                            )
                    await ws.send(WsCloseFrame(id=frame.id).to_json())

                async def from_sandbox():
                    while True:
                        f = await queue.get()
                        if isinstance(f, WsMessageFrame):
                            if f.binary:
                                await upstream_ws.send(base64.b64decode(f.data))
                            else:
                                await upstream_ws.send(f.data)
                        elif isinstance(f, WsCloseFrame):
                            break

                t1 = asyncio.create_task(from_upstream())
                t2 = asyncio.create_task(from_sandbox())
                done, pending = await asyncio.wait([t1, t2], return_when=asyncio.FIRST_COMPLETED)
                for task in pending:
                    task.cancel()
                    try:
                        await task
                    except asyncio.CancelledError:
                        pass
        except Exception as e:
            logger.warning("WS channel %s error: %s", frame.id, e)
            await ws.send(ErrorFrame(id=frame.id, message=str(e)).to_json())
        finally:
            self._ws_channels.pop(frame.id, None)
