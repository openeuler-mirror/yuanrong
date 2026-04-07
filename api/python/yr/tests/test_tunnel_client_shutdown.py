import subprocess
import sys
import textwrap
import unittest
from pathlib import Path


class TestTunnelClientShutdown(unittest.TestCase):
    def test_stop_does_not_leave_asyncio_shutdown_errors(self):
        sandbox_dir = Path(__file__).resolve().parents[1] / "sandbox"
        script = textwrap.dedent(
            f"""
            import asyncio
            import importlib.util
            import pathlib
            import sys
            import types

            from aiohttp import web

            root = pathlib.Path({str(sandbox_dir)!r})
            sys.modules['yr'] = types.ModuleType('yr')
            sys.modules['yr.sandbox'] = types.ModuleType('yr.sandbox')

            for name in ['tunnel_protocol', 'tunnel_server', 'tunnel_client']:
                path = root / f'{{name}}.py'
                spec = importlib.util.spec_from_file_location(f'yr.sandbox.{{name}}', path)
                mod = importlib.util.module_from_spec(spec)
                sys.modules[f'yr.sandbox.{{name}}'] = mod
                spec.loader.exec_module(mod)

            TunnelServer = sys.modules['yr.sandbox.tunnel_server'].TunnelServer
            TunnelClient = sys.modules['yr.sandbox.tunnel_client'].TunnelClient

            WS_PORT = 58765
            HTTP_PORT = 58766
            UPSTREAM_PORT = 58800

            async def handler(request):
                return web.Response(status=200, body=b'hello-from-c')

            async def main():
                server = TunnelServer(WS_PORT, HTTP_PORT)
                await server.start()
                app = web.Application()
                app.router.add_route('GET', '/ping', handler)
                runner = web.AppRunner(app)
                await runner.setup()
                await web.TCPSite(runner, '127.0.0.1', UPSTREAM_PORT).start()

                client = TunnelClient(upstream=f'http://127.0.0.1:{{UPSTREAM_PORT}}')
                client.start(f'ws://127.0.0.1:{{WS_PORT}}')
                try:
                    await asyncio.sleep(1)
                    import aiohttp
                    async with aiohttp.ClientSession() as session:
                        async with session.get(f'http://127.0.0.1:{{HTTP_PORT}}/ping') as resp:
                            body = await resp.read()
                            print(resp.status, body)
                finally:
                    client.stop()
                    await runner.cleanup()
                    await server.stop()

            asyncio.run(main())
            """
        )

        result = subprocess.run(
            [sys.executable, "-c", script],
            capture_output=True,
            text=True,
            timeout=20,
            check=False,
        )

        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertIn("200 b'hello-from-c'", result.stdout)
        self.assertNotIn("Task was destroyed but it is pending!", result.stderr)
        self.assertNotIn("Event loop is closed", result.stderr)
        self.assertNotIn("coroutine ignored GeneratorExit", result.stderr)


if __name__ == "__main__":
    unittest.main()
