#!/usr/bin/env python3
import ast
from pathlib import Path
import unittest


class TestCliScripts(unittest.TestCase):
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


if __name__ == "__main__":
    unittest.main()
