# api/python/yr/sandbox/tunnel_server.py
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
"""Tunnel server: runs inside the sandbox.

Port A (ws_port, 0.0.0.0): WebSocket endpoint for TunnelClient to connect.
Port B (http_port, 127.0.0.1): HTTP/WS proxy for sandbox code.
"""
import asyncio
import base64
import logging
from typing import Optional

import aiohttp
from aiohttp import web
import websockets

from yr.sandbox.tunnel_protocol import (
    parse_frame, make_id,
    HttpReqFrame, HttpRespFrame,
    WsConnectFrame, WsConnectedFrame, WsMessageFrame, WsCloseFrame, ErrorFrame,
)

logger = logging.getLogger(__name__)


class TunnelServer:
    def __init__(self, ws_port: int, http_port: int):
        self._ws_port = ws_port
        self._http_port = http_port
        self._sdk_ws = None          # active WebSocket from TunnelClient
        self._pending: dict = {}     # id -> Future (HTTP) or Queue (WS)
        self._loop: Optional[asyncio.AbstractEventLoop] = None
        self._ws_server = None
        self._http_runner = None

    async def start(self):
        self._loop = asyncio.get_event_loop()
        self._ws_server = await websockets.serve(
            self._handle_tunnel_conn, "0.0.0.0", self._ws_port,
            reuse_address=True,
        )
        app = web.Application()
        app.router.add_route("*", "/{path_info:.*}", self._handle_request)
        self._http_runner = web.AppRunner(app)
        await self._http_runner.setup()
        site = web.TCPSite(self._http_runner, "127.0.0.1", self._http_port)
        await site.start()
        logger.info("TunnelServer started ws=0.0.0.0:%d http=127.0.0.1:%d",
                    self._ws_port, self._http_port)

    async def stop(self):
        if self._ws_server:
            self._ws_server.close()
            try:
                await asyncio.wait_for(self._ws_server.wait_closed(), timeout=2.0)
            except asyncio.TimeoutError:
                pass
        if self._http_runner:
            await self._http_runner.cleanup()

    async def _handle_tunnel_conn(self, websocket):
        if self._sdk_ws is not None:
            logger.info("TunnelClient reconnected, replacing previous connection")
        self._sdk_ws = websocket
        logger.info("TunnelClient connected")
        try:
            async for message in websocket:
                frame = parse_frame(message)
                await self._dispatch_frame(frame)
        except websockets.ConnectionClosed:
            logger.info("TunnelClient disconnected")
        finally:
            # Only clean up state if this connection is still the active one.
            # A reconnect may have already replaced self._sdk_ws; in that case
            # the stale finally block must not tear down the new healthy session.
            if websocket is self._sdk_ws:
                for item in self._pending.values():
                    if isinstance(item, asyncio.Future) and not item.done():
                        item.set_exception(RuntimeError("TunnelClient disconnected"))
                    elif isinstance(item, asyncio.Queue):
                        await item.put(WsCloseFrame(id="", code=1001, reason="SDK disconnected"))
                self._pending.clear()
                self._sdk_ws = None

    async def _dispatch_frame(self, frame):
        fid = frame.id
        if fid not in self._pending:
            return
        target = self._pending[fid]
        if isinstance(frame, HttpRespFrame):
            if isinstance(target, asyncio.Future) and not target.done():
                target.set_result(frame)
        elif isinstance(frame, ErrorFrame):
            # Deliver errors to both HTTP futures and WS queues.
            if isinstance(target, asyncio.Future) and not target.done():
                target.set_result(frame)
            elif isinstance(target, asyncio.Queue):
                await target.put(frame)
        elif isinstance(frame, (WsConnectedFrame, WsMessageFrame, WsCloseFrame)):
            if isinstance(target, asyncio.Queue):
                await target.put(frame)

    async def _send_frame(self, frame):
        if self._sdk_ws is None:
            raise RuntimeError("No TunnelClient connected")
        await self._sdk_ws.send(frame.to_json())

    async def _handle_request(self, request: web.Request):
        if request.headers.get("Upgrade", "").lower() == "websocket":
            return await self._handle_ws(request)
        return await self._handle_http(request)

    async def _handle_http(self, request: web.Request) -> web.Response:
        body = await request.read()
        headers = {k: v for k, v in request.headers.items() if k.lower() != "host"}
        fid = make_id()
        frame = HttpReqFrame(
            id=fid, method=request.method,
            path=str(request.rel_url),
            headers=headers, body=body,
        )
        fut = self._loop.create_future()
        self._pending[fid] = fut
        try:
            await self._send_frame(frame)
            resp_frame = await asyncio.wait_for(fut, timeout=30)
        except asyncio.TimeoutError:
            return web.Response(status=504, text="Tunnel timeout")
        except RuntimeError as e:
            return web.Response(status=503, text=str(e))
        finally:
            self._pending.pop(fid, None)
        if isinstance(resp_frame, ErrorFrame):
            return web.Response(status=502, text=resp_frame.message)
        return web.Response(
            status=resp_frame.status,
            headers=resp_frame.headers,
            body=resp_frame.body,
        )

    async def _handle_ws(self, request: web.Request) -> web.WebSocketResponse:
        ws_resp = web.WebSocketResponse()
        await ws_resp.prepare(request)
        fid = make_id()
        headers = {k: v for k, v in request.headers.items() if k.lower() != "host"}
        queue: asyncio.Queue = asyncio.Queue()
        self._pending[fid] = queue
        try:
            await self._send_frame(WsConnectFrame(id=fid, path=str(request.rel_url), headers=headers))
            ack = await asyncio.wait_for(queue.get(), timeout=10)
            if not isinstance(ack, WsConnectedFrame):
                msg = ack.message if isinstance(ack, ErrorFrame) else "tunnel error"
                await ws_resp.close(code=1011, message=msg.encode())
                return ws_resp

            async def from_portb_to_sdk():
                async for msg in ws_resp:
                    if msg.type == aiohttp.WSMsgType.TEXT:
                        await self._send_frame(WsMessageFrame(id=fid, data=msg.data, binary=False))
                    elif msg.type == aiohttp.WSMsgType.BINARY:
                        await self._send_frame(
                            WsMessageFrame(id=fid, data=base64.b64encode(msg.data).decode(), binary=True)
                        )
                    elif msg.type in (aiohttp.WSMsgType.CLOSE, aiohttp.WSMsgType.ERROR):
                        break
                await self._send_frame(WsCloseFrame(id=fid))

            async def from_sdk_to_portb():
                while True:
                    f = await queue.get()
                    if isinstance(f, WsMessageFrame):
                        if f.binary:
                            await ws_resp.send_bytes(base64.b64decode(f.data))
                        else:
                            await ws_resp.send_str(f.data)
                    elif isinstance(f, WsCloseFrame):
                        await ws_resp.close(code=f.code)
                        break
                    elif isinstance(f, ErrorFrame):
                        await ws_resp.close(code=1011, message=f.message.encode())
                        break

            t1 = asyncio.create_task(from_portb_to_sdk())
            t2 = asyncio.create_task(from_sdk_to_portb())
            done, pending = await asyncio.wait([t1, t2], return_when=asyncio.FIRST_COMPLETED)
            for task in pending:
                task.cancel()
                try:
                    await task
                except asyncio.CancelledError:
                    pass
        finally:
            self._pending.pop(fid, None)
        return ws_resp


async def _main(ws_port: int, http_port: int):
    server = TunnelServer(ws_port, http_port)
    await server.start()
    await asyncio.Future()  # run forever


if __name__ == "__main__":
    import argparse
    p = argparse.ArgumentParser()
    p.add_argument("--ws-port", type=int, default=8765)
    p.add_argument("--http-port", type=int, default=8766)
    args = p.parse_args()
    logging.basicConfig(level=logging.INFO)
    asyncio.run(_main(args.ws_port, args.http_port))
