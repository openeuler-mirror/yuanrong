#!/usr/bin/env python3
import ast
import importlib.util
import io
from pathlib import Path
import sys
import types
from contextlib import redirect_stdout
from unittest import mock
import unittest


class TestCliScripts(unittest.TestCase):
    def load_cli_scripts_with_stubbed_deps(self):
        scripts_path = Path(__file__).resolve().parents[1] / "cli" / "scripts.py"
        spec = importlib.util.spec_from_file_location("yr_cli_scripts_for_test", scripts_path)
        scripts = importlib.util.module_from_spec(spec)

        fake_click = types.ModuleType("click")
        fake_click.option = lambda *args, **kwargs: lambda func: func
        fake_click.argument = lambda *args, **kwargs: lambda func: func
        fake_click.version_option = lambda *args, **kwargs: lambda func: func
        fake_click.pass_context = lambda func: func
        fake_click.Choice = lambda *args, **kwargs: str

        def group_decorator(*args, **kwargs):
            def decorate(func):
                func.command = lambda *a, **kw: lambda command_func: command_func
                func.group = group_decorator
                return func

            return decorate

        fake_click.group = group_decorator

        fake_requests = types.ModuleType("requests")
        fake_requests.Session = object
        fake_requests_exceptions = types.ModuleType("requests.exceptions")
        fake_requests_exceptions.RequestException = Exception
        fake_requests.exceptions = fake_requests_exceptions

        fake_yr = types.ModuleType("yr")
        fake_yr_cli = types.ModuleType("yr.cli")
        fake_yr_cli_exec = types.ModuleType("yr.cli.exec")
        fake_yr_cli_exec.run_client = lambda *args, **kwargs: None

        with mock.patch.dict(
            sys.modules,
            {
                "click": fake_click,
                "requests": fake_requests,
                "requests.exceptions": fake_requests_exceptions,
                "yr": fake_yr,
                "yr.cli": fake_yr_cli,
                "yr.cli.exec": fake_yr_cli_exec,
            },
        ):
            spec.loader.exec_module(scripts)
        return scripts

    def test_cli_defines_insecure_option_once(self):
        scripts_path = Path(__file__).resolve().parents[1] / "cli" / "scripts.py"
        module = ast.parse(scripts_path.read_text())

        cli_func = next(
            node for node in module.body if isinstance(node, ast.FunctionDef) and node.name == "cli"
        )

        insecure_option_count = 0
        for decorator in cli_func.decorator_list:
            if not isinstance(decorator, ast.Call):
                continue
            if not isinstance(decorator.func, ast.Attribute):
                continue
            if decorator.func.attr != "option":
                continue
            if any(isinstance(arg, ast.Constant) and arg.value == "--insecure" for arg in decorator.args):
                insecure_option_count += 1

        self.assertEqual(insecure_option_count, 1)

    def test_query_instances_appends_pagination_params(self):
        scripts = self.load_cli_scripts_with_stubbed_deps()
        setattr(scripts, "__server_address", "frontend.example")

        class FakeHTTPClient:
            def __init__(self, **kwargs):
                pass

            def request(self, url, data, method="POST", headers=None):
                self.__class__.url = url
                self.__class__.method = method
                return {"success": True, "data": []}

        with mock.patch.object(scripts, "HTTPClient", FakeHTTPClient):
            ret, resp = scripts.query_instances("tenant-a", page=2, page_size=50)

        self.assertTrue(ret)
        self.assertEqual(resp, [])
        self.assertEqual(FakeHTTPClient.method, "GET")
        self.assertIn("tenant_id=tenant-a", FakeHTTPClient.url)
        self.assertIn("page=2", FakeHTTPClient.url)
        self.assertIn("page_size=50", FakeHTTPClient.url)

    def test_query_instance_uses_frontend_instances_query_endpoint(self):
        scripts = self.load_cli_scripts_with_stubbed_deps()
        setattr(scripts, "__server_address", "frontend.example")

        class FakeHTTPClient:
            def __init__(self, **kwargs):
                pass

            def request(self, url, data, method="POST", headers=None):
                self.__class__.url = url
                self.__class__.method = method
                return {
                    "success": True,
                    "data": [{"id": "instance-a", "tenantID": "tenant-a"}],
                }

        with mock.patch.object(scripts, "HTTPClient", FakeHTTPClient):
            ret, resp = scripts.query_instance("instance-a", "tenant-a")

        self.assertTrue(ret)
        self.assertEqual(resp["id"], "instance-a")
        self.assertEqual(FakeHTTPClient.method, "GET")
        self.assertIn("/api/instances?", FakeHTTPClient.url)
        self.assertIn("tenant_id=tenant-a", FakeHTTPClient.url)
        self.assertIn("instance_id=instance-a", FakeHTTPClient.url)

    def test_query_instance_ignores_malformed_instance_entries(self):
        scripts = self.load_cli_scripts_with_stubbed_deps()
        setattr(scripts, "__server_address", "frontend.example")

        class FakeHTTPClient:
            def __init__(self, **kwargs):
                pass

            def request(self, url, data, method="POST", headers=None):
                return {
                    "success": True,
                    "data": {"instances": [None, "bad-entry", {"id": "instance-a", "tenantID": "tenant-a"}]},
                }

        with mock.patch.object(scripts, "HTTPClient", FakeHTTPClient):
            ret, resp = scripts.query_instance("instance-a", "tenant-a")

        self.assertTrue(ret)
        self.assertEqual(resp["id"], "instance-a")

    def test_query_instance_rejects_malformed_response(self):
        scripts = self.load_cli_scripts_with_stubbed_deps()
        setattr(scripts, "__server_address", "frontend.example")

        class FakeHTTPClient:
            def __init__(self, **kwargs):
                pass

            def request(self, url, data, method="POST", headers=None):
                return {
                    "success": True,
                    "data": {"error": "unexpected response"},
                }

        with mock.patch.object(scripts, "HTTPClient", FakeHTTPClient):
            ret, resp = scripts.query_instance("instance-a", "tenant-a")

        self.assertFalse(ret)
        self.assertEqual(resp["error"], "invalid instances response: missing instances")

    def test_list_instances_passes_pagination_params(self):
        scripts = self.load_cli_scripts_with_stubbed_deps()
        setattr(scripts, "__user", "tenant-a")
        calls = []

        def fake_query_instances(user=None, page=None, page_size=None):
            calls.append((user, page, page_size))
            return True, {"instances": [{"id": "instance-a", "tenantID": "tenant-a"}]}

        with mock.patch.object(scripts, "query_instances", fake_query_instances), redirect_stdout(io.StringIO()):
            scripts.list(3, 25, "instance")

        self.assertEqual(calls, [("tenant-a", 3, 25)])

    def test_list_instances_rejects_oversized_page_size(self):
        scripts = self.load_cli_scripts_with_stubbed_deps()

        with self.assertRaises(SystemExit) as ctx, redirect_stdout(io.StringIO()) as output:
            scripts.list(None, scripts.QUERY_INSTANCES_MAX_PAGE_SIZE + 1, "instance")

        self.assertEqual(ctx.exception.code, 1)
        self.assertIn("less than or equal to 1000", output.getvalue())

    def test_list_instances_rejects_oversized_page(self):
        scripts = self.load_cli_scripts_with_stubbed_deps()

        with self.assertRaises(SystemExit) as ctx, redirect_stdout(io.StringIO()) as output:
            scripts.list(scripts.QUERY_INSTANCES_MAX_PAGE + 1, None, "instance")

        self.assertEqual(ctx.exception.code, 1)
        self.assertIn("less than or equal to 10000", output.getvalue())

    def test_list_instances_rejects_malformed_response(self):
        scripts = self.load_cli_scripts_with_stubbed_deps()
        setattr(scripts, "__user", "tenant-a")

        def fake_query_instances(user=None, page=None, page_size=None):
            return True, {"error": "unexpected response"}

        with (
            mock.patch.object(scripts, "query_instances", fake_query_instances),
            self.assertRaises(SystemExit) as ctx,
            redirect_stdout(io.StringIO()) as output,
        ):
            scripts.list(None, None, "instance")

        self.assertEqual(ctx.exception.code, 1)
        self.assertIn("invalid instances response: missing instances", output.getvalue())


if __name__ == "__main__":
    unittest.main()
