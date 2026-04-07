# api/python/yr/tests/test_tunnel_integration.py
"""End-to-end: TunnelServer (in sandbox) + TunnelClient (local) + mock upstream."""
import asyncio
import threading
import unittest
from aiohttp import web
import websockets
from yr.sandbox.tunnel_server import TunnelServer
from yr.sandbox.tunnel_client import TunnelClient

SRV_WS_PORT = 38765
SRV_HTTP_PORT = 38766
UPSTREAM_PORT = 38800


class TestIntegration(unittest.TestCase):
    def setUp(self):
        """Start TunnelServer in a background asyncio loop."""
        self._server_loop = asyncio.new_event_loop()
        self._server = TunnelServer(ws_port=SRV_WS_PORT, http_port=SRV_HTTP_PORT)
        ready = threading.Event()

        def _run_server():
            asyncio.set_event_loop(self._server_loop)
            self._server_loop.run_until_complete(self._server.start())
            ready.set()
            self._server_loop.run_forever()

        self._server_thread = threading.Thread(target=_run_server, daemon=True)
        self._server_thread.start()
        ready.wait(timeout=5)

    def tearDown(self):
        # Properly close server sockets before stopping the loop so ports are
        # released immediately and the next test can bind to the same ports.
        stop_fut = asyncio.run_coroutine_threadsafe(self._server.stop(), self._server_loop)
        stop_fut.result(timeout=5)
        self._server_loop.call_soon_threadsafe(self._server_loop.stop)
        self._server_thread.join(timeout=5)

    # ------------------------------------------------------------------
    # Helpers: start/stop an upstream aiohttp app inside _server_loop so
    # it stays alive between asyncio.run() calls in the test body.
    # ------------------------------------------------------------------

    def _start_upstream(self, app: web.Application) -> web.AppRunner:
        """Start an aiohttp app on UPSTREAM_PORT inside the shared server loop."""
        async def _start():
            runner = web.AppRunner(app)
            await runner.setup()
            await web.TCPSite(runner, "127.0.0.1", UPSTREAM_PORT).start()
            return runner
        return asyncio.run_coroutine_threadsafe(_start(), self._server_loop).result(timeout=5)

    def _stop_upstream(self, runner: web.AppRunner) -> None:
        asyncio.run_coroutine_threadsafe(runner.cleanup(), self._server_loop).result(timeout=5)

    # ------------------------------------------------------------------

    def test_http_get_roundtrip_through_tunnel(self):
        """Full roundtrip: Port B HTTP GET → WS tunnel → TunnelClient → mock upstream → response."""
        import time

        async def handler(request):
            return web.Response(status=200, body=b"hello-from-c",
                                headers={"Content-Type": "text/plain"})
        app = web.Application()
        app.router.add_route("GET", "/ping", handler)
        upstream_runner = self._start_upstream(app)

        async def _fetch_via_portb():
            import aiohttp
            async with aiohttp.ClientSession() as session:
                async with session.get(f"http://127.0.0.1:{SRV_HTTP_PORT}/ping") as resp:
                    return resp.status, await resp.read()

        try:
            client = TunnelClient(upstream=f"http://127.0.0.1:{UPSTREAM_PORT}")
            client.start(f"ws://127.0.0.1:{SRV_WS_PORT}")
            time.sleep(0.5)  # let client connect

            status, body = asyncio.run(_fetch_via_portb())
            self.assertEqual(status, 200)
            self.assertEqual(body, b"hello-from-c")
        finally:
            client.stop()
            self._stop_upstream(upstream_runner)

    def test_http_post_with_body_roundtrip(self):
        """POST with body is forwarded and body reaches upstream."""
        import time
        received_body = {}

        async def handler(request):
            received_body["data"] = await request.read()
            return web.Response(status=201, body=b"created")
        app = web.Application()
        app.router.add_route("POST", "/items", handler)
        upstream_runner = self._start_upstream(app)

        async def _post_via_portb():
            import aiohttp
            async with aiohttp.ClientSession() as session:
                async with session.post(
                    f"http://127.0.0.1:{SRV_HTTP_PORT}/items",
                    data=b'{"name":"test"}',
                    headers={"Content-Type": "application/json"},
                ) as resp:
                    return resp.status

        try:
            client = TunnelClient(upstream=f"http://127.0.0.1:{UPSTREAM_PORT}")
            client.start(f"ws://127.0.0.1:{SRV_WS_PORT}")
            time.sleep(0.5)

            status = asyncio.run(_post_via_portb())
            self.assertEqual(status, 201)
            self.assertEqual(received_body["data"], b'{"name":"test"}')
        finally:
            client.stop()
            self._stop_upstream(upstream_runner)

    def test_ws_message_relay_through_tunnel(self):
        """WebSocket messages are relayed end-to-end: Port B client ↔ upstream WS server."""
        import time

        async def ws_handler(request):
            ws = web.WebSocketResponse()
            await ws.prepare(request)
            async for msg in ws:
                if msg.type == web.WSMsgType.TEXT:
                    await ws.send_str(f"echo:{msg.data}")
            return ws
        app = web.Application()
        app.router.add_route("GET", "/ws", ws_handler)
        upstream_runner = self._start_upstream(app)

        async def _ws_via_portb():
            async with websockets.connect(f"ws://127.0.0.1:{SRV_HTTP_PORT}/ws") as ws:
                await ws.send("hello")
                reply = await asyncio.wait_for(ws.recv(), timeout=5)
                return reply

        try:
            client = TunnelClient(upstream=f"http://127.0.0.1:{UPSTREAM_PORT}")
            client.start(f"ws://127.0.0.1:{SRV_WS_PORT}")
            time.sleep(0.5)

            reply = asyncio.run(_ws_via_portb())
            self.assertEqual(reply, "echo:hello")
        finally:
            client.stop()
            self._stop_upstream(upstream_runner)


    def test_concurrent_http_requests(self):
        """Multiple HTTP requests in parallel are all handled correctly."""
        import time

        async def handler(request):
            idx = request.match_info["idx"]
            return web.Response(status=200, body=f"response-{idx}".encode())

        app = web.Application()
        app.router.add_route("GET", "/item/{idx}", handler)
        upstream_runner = self._start_upstream(app)

        async def _fetch_all():
            import aiohttp
            async with aiohttp.ClientSession() as session:
                tasks = [
                    session.get(f"http://127.0.0.1:{SRV_HTTP_PORT}/item/{i}")
                    for i in range(10)
                ]
                responses = await asyncio.gather(*tasks)
                results = []
                for resp in responses:
                    body = await resp.read()
                    results.append((resp.status, body))
                    resp.close()
                return results

        try:
            client = TunnelClient(upstream=f"http://127.0.0.1:{UPSTREAM_PORT}")
            client.start(f"ws://127.0.0.1:{SRV_WS_PORT}")
            time.sleep(0.5)

            results = asyncio.run(_fetch_all())
            self.assertEqual(len(results), 10)
            for status, body in results:
                self.assertEqual(status, 200)
            bodies = {body for _, body in results}
            self.assertEqual(len(bodies), 10)  # all distinct responses
        finally:
            client.stop()
            self._stop_upstream(upstream_runner)

    def test_ws_channel_cleanup_on_client_disconnect(self):
        """WS channel is removed from _ws_channels when client disconnects."""
        import time
        from yr.sandbox.tunnel_protocol import WsConnectFrame, make_id

        async def ws_handler(request):
            ws = web.WebSocketResponse()
            await ws.prepare(request)
            async for _ in ws:
                pass
            return ws

        app = web.Application()
        app.router.add_route("GET", "/ws", ws_handler)
        upstream_runner = self._start_upstream(app)

        async def _connect_and_disconnect():
            async with websockets.connect(f"ws://127.0.0.1:{SRV_HTTP_PORT}/ws") as ws:
                await asyncio.sleep(0.1)
                # connection opens and closes
            await asyncio.sleep(0.2)

        try:
            client = TunnelClient(upstream=f"http://127.0.0.1:{UPSTREAM_PORT}")
            client.start(f"ws://127.0.0.1:{SRV_WS_PORT}")
            time.sleep(0.5)

            asyncio.run(_connect_and_disconnect())

            # After disconnect, no lingering ws channels on client side
            self.assertEqual(len(client._ws_channels), 0)
        finally:
            client.stop()
            self._stop_upstream(upstream_runner)


if __name__ == "__main__":
    unittest.main()
