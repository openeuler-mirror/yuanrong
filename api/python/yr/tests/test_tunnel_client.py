# api/python/yr/tests/test_tunnel_client.py
import asyncio
import base64
import json
import threading
import time
import unittest
import websockets
from aiohttp import web
from yr.sandbox.tunnel_client import TunnelClient
from yr.sandbox.tunnel_protocol import (
    HttpReqFrame, HttpRespFrame, ErrorFrame,
    WsConnectFrame, WsConnectedFrame, WsMessageFrame, WsCloseFrame,
    parse_frame,
)

MOCK_WS_PORT = 28765      # mock Port A (pretend to be TunnelServer)
MOCK_UPSTREAM_PORT = 28800  # mock local service C


class TestTunnelClientHttp(unittest.TestCase):
    def test_http_req_frame_forwarded_to_upstream(self):
        """TunnelClient receives http_req frame, forwards to upstream, sends http_resp frame back."""
        results = {}

        async def _run():
            # Start mock upstream (service C)
            async def upstream_handler(request):
                results["upstream_path"] = request.path
                return web.Response(status=200, body=b"from-upstream",
                                    headers={"Content-Type": "text/plain"})

            upstream_app = web.Application()
            upstream_app.router.add_route("*", "/{path_info:.*}", upstream_handler)
            upstream_runner = web.AppRunner(upstream_app)
            await upstream_runner.setup()
            await web.TCPSite(upstream_runner, "127.0.0.1", MOCK_UPSTREAM_PORT).start()

            # Start mock Port A (WS server)
            received_frames = []

            async def mock_port_a(websocket):
                # Send an http_req frame to TunnelClient
                req = HttpReqFrame(id="r1", method="GET", path="/test",
                                   headers={}, body=b"")
                await websocket.send(req.to_json())
                # Receive the http_resp frame back
                raw = await asyncio.wait_for(websocket.recv(), timeout=5)
                received_frames.append(parse_frame(raw))

            ws_server = await websockets.serve(mock_port_a, "127.0.0.1", MOCK_WS_PORT)

            # Start TunnelClient
            client = TunnelClient(upstream=f"http://127.0.0.1:{MOCK_UPSTREAM_PORT}")
            client.start(f"ws://127.0.0.1:{MOCK_WS_PORT}")
            await asyncio.sleep(1)  # let client connect and process

            ws_server.close()
            await ws_server.wait_closed()
            await upstream_runner.cleanup()
            client.stop()
            return received_frames

        frames = asyncio.run(_run())
        self.assertEqual(len(frames), 1)
        self.assertIsInstance(frames[0], HttpRespFrame)
        self.assertEqual(frames[0].id, "r1")
        self.assertEqual(frames[0].status, 200)
        self.assertEqual(frames[0].body, b"from-upstream")

    def test_upstream_error_sends_error_frame(self):
        """If upstream is unreachable, TunnelClient sends error frame."""
        received_frames = []

        async def _run():
            async def mock_port_a(websocket):
                req = HttpReqFrame(id="r2", method="GET", path="/x", headers={}, body=b"")
                await websocket.send(req.to_json())
                raw = await asyncio.wait_for(websocket.recv(), timeout=5)
                received_frames.append(parse_frame(raw))

            ws_server = await websockets.serve(mock_port_a, "127.0.0.1", MOCK_WS_PORT)
            # Point to a port with nothing listening
            client = TunnelClient(upstream="http://127.0.0.1:19999")
            client.start(f"ws://127.0.0.1:{MOCK_WS_PORT}")
            await asyncio.sleep(1)
            ws_server.close()
            await ws_server.wait_closed()
            client.stop()

        asyncio.run(_run())
        self.assertEqual(len(received_frames), 1)
        self.assertIsInstance(received_frames[0], ErrorFrame)
        self.assertEqual(received_frames[0].id, "r2")

    def test_client_reconnects_after_disconnect(self):
        """TunnelClient should reconnect when server disconnects."""
        connect_count = {"n": 0}

        async def _run():
            async def mock_port_a(websocket):
                connect_count["n"] += 1
                # Immediately close — force reconnect
                await websocket.close()

            ws_server = await websockets.serve(mock_port_a, "127.0.0.1", MOCK_WS_PORT)
            client = TunnelClient(upstream="http://127.0.0.1:19999")
            client.start(f"ws://127.0.0.1:{MOCK_WS_PORT}")
            await asyncio.sleep(4)  # reconnect delay is 3s
            ws_server.close()
            await ws_server.wait_closed()
            client.stop()

        asyncio.run(_run())
        self.assertGreaterEqual(connect_count["n"], 2)


if __name__ == "__main__":
    unittest.main()
