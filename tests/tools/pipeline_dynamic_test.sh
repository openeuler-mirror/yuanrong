#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

DEFAULT_PIPELINE="${TMP_DIR}/pipeline-default.yml"
RUST_PIPELINE="${TMP_DIR}/pipeline-rust.yml"

(
  cd "${ROOT_DIR}"
  bash .buildkite/pipeline.dynamic.yml >"${DEFAULT_PIPELINE}"
  ENABLE_RUST_FUNCTIONSYSTEM_ST=true \
  FUNCTIONSYSTEM_REPO=https://gitcode.com/openeuler/yuanrong-functionsystem.git \
  FUNCTIONSYSTEM_BRANCH=rust-rewrite \
  RUST_BUILDER_IMAGE=swr.example/compile-ubuntu2004-rust:test \
    bash .buildkite/pipeline.dynamic.yml >"${RUST_PIPELINE}"

  if ENABLE_RUST_FUNCTIONSYSTEM_ST=true \
    FUNCTIONSYSTEM_BRANCH='rust-rewrite;echo unsafe' \
    bash .buildkite/pipeline.dynamic.yml >"${TMP_DIR}/pipeline-invalid.yml" 2>"${TMP_DIR}/pipeline-invalid.err"; then
    echo "invalid Rust functionsystem branch should fail dynamic pipeline generation" >&2
    exit 1
  fi
)

python3 - "${DEFAULT_PIPELINE}" "${RUST_PIPELINE}" <<'PY'
import sys
import yaml

default_path, rust_path = sys.argv[1:]

with open(default_path, encoding="utf-8") as f:
    default_pipeline = yaml.safe_load(f)
with open(rust_path, encoding="utf-8") as f:
    rust_pipeline = yaml.safe_load(f)

default_labels = [step.get("label") for step in default_pipeline["steps"]]
if ":crab: Rust FunctionSystem E2E" in default_labels:
    raise SystemExit("Rust E2E step should not be emitted by default")

rust_steps = [
    step for step in rust_pipeline["steps"]
    if step.get("label") == ":crab: Rust FunctionSystem E2E"
]
if len(rust_steps) != 1:
    raise SystemExit(f"expected exactly one Rust E2E step, found {len(rust_steps)}")

step = rust_steps[0]
assert step["key"] == "rust-functionsystem-e2e-amd64"
assert step["depends_on"] == "build-all-amd64"
assert step["agents"] == {"queue": "default", "os": "linux", "arch": "amd64"}

command = step["command"]
for token in [
    "buildkite-agent artifact download \"artifacts/release/*\" . --step build-all-amd64",
    "git clone --depth 1",
    "make functionsystem",
    "bash scripts/package_yuanrong.sh",
    "bash test/st/test.sh",
    "buildkite-agent artifact upload \"artifacts/rust-fs-st/**/*\"",
]:
    if token not in command:
        raise SystemExit(f"Rust E2E command missing token: {token}")

pod_spec = step["plugins"][0]["kubernetes"]["podSpec"]
container = pod_spec["containers"][0]
assert container["image"] == "swr.example/compile-ubuntu2004-rust:test"
assert pod_spec["imagePullSecrets"] == [{"name": "swr-pull-secret"}]
PY
