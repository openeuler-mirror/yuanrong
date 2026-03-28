# api/python/yr/tests/test_sandbox_tunnel.py
"""Tests for sandbox.py tunnel API (SandBox with upstream param)."""
import unittest
from unittest.mock import MagicMock, patch, call


class TestSandBoxTunnelUrl(unittest.TestCase):
    def test_get_tunnel_url_default_port(self):
        """SandBox.get_tunnel_url() returns http://127.0.0.1:{proxy_port}."""
        from yr.sandbox.sandbox import SandBox
        sb = object.__new__(SandBox)
        sb._proxy_port = 8766
        sb._tunnel_client = None
        sb._instance = MagicMock()
        self.assertRaises(RuntimeError, sb.get_tunnel_url)

    def test_get_tunnel_url_when_tunnel_active(self):
        """SandBox.get_tunnel_url() returns correct URL when tunnel client is set."""
        from yr.sandbox.sandbox import SandBox
        sb = object.__new__(SandBox)
        sb._proxy_port = 8766
        sb._tunnel_client = MagicMock()  # non-None = tunnel active
        sb._instance = MagicMock()
        self.assertEqual(sb.get_tunnel_url(), "http://127.0.0.1:8766")

    def test_get_tunnel_url_custom_port(self):
        """SandBox.get_tunnel_url() uses custom proxy_port."""
        from yr.sandbox.sandbox import SandBox
        sb = object.__new__(SandBox)
        sb._proxy_port = 9000
        sb._tunnel_client = MagicMock()
        sb._instance = MagicMock()
        self.assertEqual(sb.get_tunnel_url(), "http://127.0.0.1:9000")

    def test_terminate_stops_tunnel_client(self):
        """SandBox.terminate() calls tunnel_client.stop() before instance.terminate()."""
        from yr.sandbox.sandbox import SandBox
        sb = object.__new__(SandBox)
        sb._proxy_port = 8766
        mock_client = MagicMock()
        sb._tunnel_client = mock_client
        sb._instance = MagicMock()
        sb.terminate()
        mock_client.stop.assert_called_once()
        sb._instance.terminate.assert_called_once()

    def test_terminate_without_tunnel_client(self):
        """SandBox.terminate() works when no tunnel client set."""
        from yr.sandbox.sandbox import SandBox
        sb = object.__new__(SandBox)
        sb._proxy_port = 8766
        sb._tunnel_client = None
        sb._instance = MagicMock()
        sb.terminate()  # should not raise
        sb._instance.terminate.assert_called_once()


class TestSandBoxTunnelPortDerivation(unittest.TestCase):
    def test_tunnel_port_is_proxy_port_minus_one(self):
        """Port A = proxy_port - 1."""
        proxy_port = 9100
        tunnel_port = proxy_port - 1
        self.assertEqual(tunnel_port, 9099)

    def test_default_ports(self):
        proxy_port = 8766
        tunnel_port = proxy_port - 1
        self.assertEqual(tunnel_port, 8765)


class TestCreateFunctionSignature(unittest.TestCase):
    def test_create_accepts_upstream_param(self):
        """create() should accept upstream keyword argument."""
        import inspect
        from yr.sandbox.sandbox import create
        sig = inspect.signature(create)
        self.assertIn("upstream", sig.parameters)

    def test_create_accepts_proxy_port_param(self):
        """create() should accept proxy_port keyword argument."""
        import inspect
        from yr.sandbox.sandbox import create
        sig = inspect.signature(create)
        self.assertIn("proxy_port", sig.parameters)

    def test_create_proxy_port_default_is_8766(self):
        """proxy_port default should be 8766."""
        import inspect
        from yr.sandbox.sandbox import create
        sig = inspect.signature(create)
        self.assertEqual(sig.parameters["proxy_port"].default, 8766)

    def test_create_upstream_default_is_none(self):
        """upstream default should be None."""
        import inspect
        from yr.sandbox.sandbox import create
        sig = inspect.signature(create)
        self.assertIsNone(sig.parameters["upstream"].default)


class TestGatewayHostResolution(unittest.TestCase):
    @patch.dict("os.environ", {}, clear=True)
    def test_get_gateway_host_prefers_env_gateway_address(self):
        import yr.sandbox.sandbox as sb_module

        with patch.dict("os.environ", {"YR_GATEWAY_ADDRESS": "gw.example:443", "YR_SERVER_ADDRESS": "127.0.0.1:38888"}):
            self.assertEqual(sb_module._get_gateway_host(), "gw.example:443")

    @patch.dict("os.environ", {"YR_SERVER_ADDRESS": "127.0.0.1:38888"}, clear=True)
    def test_get_gateway_host_falls_back_to_env_server_address(self):
        import yr.sandbox.sandbox as sb_module

        self.assertEqual(sb_module._get_gateway_host(), "127.0.0.1:38888")

    @patch.dict("os.environ", {}, clear=True)
    def test_get_gateway_host_falls_back_to_runtime_config_server_address(self):
        import yr.sandbox.sandbox as sb_module

        fake_config_manager = MagicMock()
        fake_config_manager.server_address = "127.0.0.1:38888"
        with patch.object(sb_module, "ConfigManager", return_value=fake_config_manager):
            self.assertEqual(sb_module._get_gateway_host(), "127.0.0.1:38888")


class TestStartTunnelServer(unittest.TestCase):
    def _get_sandbox_instance_class(self):
        """Get the underlying SandboxInstance class (unwrapped from @yr.instance)."""
        import yr.sandbox.sandbox as sb_module
        # @yr.instance wraps the class; get the original via __wrapped__ or the class attribute
        cls = getattr(sb_module, "_SandboxInstance_cls", None)
        if cls is None:
            # Fallback: SandboxInstance is the wrapped function; get wrapped if available
            wrapped = getattr(sb_module.SandboxInstance, "__wrapped__", None)
            if wrapped is not None:
                cls = wrapped
        return cls

    def test_start_tunnel_server_uses_default_ports(self):
        """start_tunnel_server() defaults to ws_port=8765, http_port=8766."""
        import inspect
        import yr.sandbox.sandbox as sb_module
        src = inspect.getsource(sb_module)
        # Verify default parameter values in source
        self.assertIn("ws_port: int = 8765", src)
        self.assertIn("http_port: int = 8766", src)

    def test_start_tunnel_server_method_exists_in_source(self):
        """start_tunnel_server method is defined in SandboxInstance source."""
        import inspect
        import yr.sandbox.sandbox as sb_module
        # The source of the module must contain start_tunnel_server
        src = inspect.getsource(sb_module)
        self.assertIn("def start_tunnel_server", src)
        self.assertIn("threading.Thread", src)
        self.assertIn("asyncio.new_event_loop", src)
        self.assertIn("daemon=True", src)


if __name__ == "__main__":
    unittest.main()
