#!/usr/bin/env python3
# coding=UTF-8
"""E2E validation: InstanceProxy.real_id property"""

import yr


@yr.instance
class Counter:
    def __init__(self):
        self.count = 0

    def inc(self):
        self.count += 1
        return self.count


def main():
    yr.init()
    try:
        ins = Counter.invoke()

        logic_id = ins.instance_id
        real_id = ins.real_id

        print(f"logic_id : {logic_id}")
        print(f"real_id  : {real_id}")

        assert isinstance(real_id, str) and len(real_id) > 0, "real_id should be a non-empty string"
        print("real_id type/length check passed")

        result = yr.get(ins.inc.invoke())
        assert result == 1, f"expected 1, got {result}"
        print(f"actor method invoke passed: count={result}")

        ins.terminate()
        print("PASS: InstanceProxy.real_id e2e validation succeeded")
    finally:
        yr.finalize()


if __name__ == "__main__":
    main()
