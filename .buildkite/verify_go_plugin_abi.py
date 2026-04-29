#!/usr/bin/env python3
# coding=UTF-8
"""Verify Go runtime and system-function plugins share package ABI hashes."""

import pathlib
import re
import subprocess
import sys


ROOT = pathlib.Path("output/openyuanrong")
RUNTIME = ROOT / "runtime/service/go/bin/goruntime"
PLUGINS = [
    ROOT / "pattern/pattern_faas/faasscheduler/faasscheduler.so",
    ROOT / "pattern/pattern_faas/faasfrontend/faasfrontend.so",
]
SYMBOL = "go:link.pkghashbytes.go.uber.org/multierr"
SECTION_RE = re.compile(
    r"^\s*\[\s*(\d+)\]\s+(\S+)\s+\S+\s+([0-9a-fA-F]+)\s+([0-9a-fA-F]+)\s+([0-9a-fA-F]+)"
)
SYMBOL_RE = re.compile(
    r"^\s*\d+:\s+([0-9a-fA-F]+)\s+(\d+)\s+\S+\s+\S+\s+\S+\s+(\d+)\s+"
    + re.escape(SYMBOL)
    + r"$"
)


def command_output(args):
    return subprocess.check_output(args).decode("latin1")


def pkghash(path):
    sections = []
    for line in command_output(["readelf", "-SW", str(path)]).splitlines():
        match = SECTION_RE.match(line)
        if match:
            idx, name, addr, off, size = match.groups()
            sections.append((int(idx), name, int(addr, 16), int(off, 16), int(size, 16)))
    data = path.read_bytes()
    for line in command_output(["readelf", "-sW", str(path)]).splitlines():
        match = SYMBOL_RE.match(line)
        if not match:
            continue
        addr = int(match.group(1), 16)
        size = int(match.group(2))
        section_idx = int(match.group(3))
        _, _, section_addr, section_off, _ = next(item for item in sections if item[0] == section_idx)
        offset = section_off + (addr - section_addr)
        return data[offset : offset + size].hex()
    raise RuntimeError(f"{SYMBOL} not found in {path}")


def main():
    runtime_hash = pkghash(RUNTIME)
    print(f"{RUNTIME}: {runtime_hash}")
    failed = False
    for plugin in PLUGINS:
        plugin_hash = pkghash(plugin)
        print(f"{plugin}: {plugin_hash}")
        if plugin_hash != runtime_hash:
            print(
                f"Go plugin ABI mismatch: {plugin} has {plugin_hash}, "
                f"but goruntime has {runtime_hash}",
                file=sys.stderr,
            )
            failed = True
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
