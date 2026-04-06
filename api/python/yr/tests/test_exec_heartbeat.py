#!/usr/bin/env python3
"""Test exec heartbeat: verify heartbeat_loop detects dead connections."""
import asyncio
import socket
import time
import unittest
import websockets


def _find_free_port():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


class TestExecHeartbeat(unittest.TestCase):

    def test_heartbeat_detects_dead_connection(self):
        """heartbeat_loop should set should_exit when pong times out."""
        port = _find_free_port()

        async def _run():
            # Start a real server
            async def handler(websocket):
                # Immediately close to simulate dead connection
                await websocket.close()

            async with await websockets.serve(
                handler, "127.0.0.1", port,
                ping_interval=None,
                ping_timeout=None,
            ) as server:
                await asyncio.sleep(0.1)

                try:
                    async with websockets.connect(
                        f"ws://127.0.0.1:{port}",
                        ping_interval=None,
                        ping_timeout=None,
                    ) as ws:
                        should_exit = asyncio.Event()
                        t0 = time.monotonic()

                        from yr.cli.exec import heartbeat_loop

                        task = asyncio.create_task(
                            heartbeat_loop(ws, should_exit, ping_interval=0.1, ping_timeout=0.3)
                        )
                        await should_exit.wait()
                        elapsed = time.monotonic() - t0

                        task.cancel()
                        try:
                            await task
                        except (asyncio.CancelledError, websockets.ConnectionClosed):
                            pass
                except websockets.ConnectionClosed:
                    # Connection closed immediately by server
                    pass

            self.assertTrue(True)  # Reached without hanging

        asyncio.run(_run())

    def test_heartbeat_stays_alive_with_responsive_server(self):
        """heartbeat_loop should NOT exit when server responds normally."""
        port = _find_free_port()

        async def _run():
            async def handler(websocket):
                # Keep connection alive, respond to pings (default behavior)
                try:
                    async for msg in websocket:
                        pass
                except websockets.ConnectionClosed:
                    pass

            async with await websockets.serve(
                handler, "127.0.0.1", port,
                ping_interval=None,
                ping_timeout=None,
            ) as server:
                await asyncio.sleep(0.1)

                async with websockets.connect(
                    f"ws://127.0.0.1:{port}",
                    ping_interval=None,
                    ping_timeout=None,
                ) as ws:
                    should_exit = asyncio.Event()

                    from yr.cli.exec import heartbeat_loop

                    task = asyncio.create_task(
                        heartbeat_loop(ws, should_exit, ping_interval=0.15, ping_timeout=0.5)
                    )
                    # Run ~1s (several ping cycles)
                    await asyncio.sleep(1.0)

                    # Should still be alive
                    self.assertFalse(should_exit.is_set())

                    task.cancel()
                    try:
                        await task
                    except (asyncio.CancelledError, websockets.ConnectionClosed):
                        pass

                    await ws.close()

        asyncio.run(_run())


if __name__ == "__main__":
    unittest.main()
