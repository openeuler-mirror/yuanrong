#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

FAKE_BIN="${TMP_DIR}/bin"
ARTIFACTS_DIR="${TMP_DIR}/artifacts"
PAYLOAD_FILE="${TMP_DIR}/payload.json"
mkdir -p "${FAKE_BIN}" "${ARTIFACTS_DIR}"

cat >"${FAKE_BIN}/git" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == "rev-parse --abbrev-ref HEAD" ]]; then
  echo "feature/test"
elif [[ "$*" == "rev-parse HEAD" ]]; then
  echo "abcdef1234567890"
else
  echo "unexpected git args: $*" >&2
  exit 1
fi
EOF
chmod +x "${FAKE_BIN}/git"

cat >"${FAKE_BIN}/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

out_file=""
data=""
url=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    -o|--output)
      out_file="$2"
      shift 2
      ;;
    -d|--data|--data-binary)
      data="$2"
      shift 2
      ;;
    -H|-X)
      shift 2
      ;;
    -*)
      shift
      ;;
    *)
      url="$1"
      shift
      ;;
  esac
done

case "${url}" in
  */organizations/test-org/pipelines/test-pipeline/builds)
    printf '%s\n' "${data}" >"${CURL_PAYLOAD_FILE}"
    cat <<JSON
{"number":42,"web_url":"https://buildkite.example/builds/42"}
JSON
    ;;
  */organizations/test-org/pipelines/test-pipeline/builds/42)
    state="${FAKE_BUILD_STATE:-passed}"
    job_state="passed"
    if [[ "${state}" != "passed" ]]; then
      job_state="failed"
    fi
    cat <<JSON
{"state":"${state}","jobs":[{"id":"job-pass","state":"${job_state}","label":"rust e2e"}]}
JSON
    ;;
  */organizations/test-org/pipelines/test-pipeline/builds/42/jobs/job-pass/log.txt)
    printf 'failed rust e2e log tail\n'
    ;;
  */organizations/test-org/pipelines/test-pipeline/builds/42/artifacts)
    cat <<JSON
[
  {
    "id":"artifact-pass",
    "job_id":"job-pass",
    "download_url":"https://download.example/artifact-pass",
    "state":"finished",
    "path":"artifacts/rust-fs-st/driver/python_output.txt",
    "filename":"python_output.txt"
  },
  {
    "id":"artifact-skip",
    "job_id":"job-pass",
    "download_url":"https://download.example/artifact-skip",
    "state":"new",
    "path":"artifacts/rust-fs-st/driver/incomplete.txt",
    "filename":"incomplete.txt"
  },
  {
    "id":"artifact-other",
    "job_id":"job-pass",
    "download_url":"https://download.example/artifact-other",
    "state":"finished",
    "path":"artifacts/release/openyuanrong.whl",
    "filename":"openyuanrong.whl"
  }
]
JSON
    ;;
  https://download.example/artifact-pass)
    mkdir -p "$(dirname "${out_file}")"
    printf 'PYTHON ST PASS\n' >"${out_file}"
    ;;
  *)
    echo "unexpected curl url: ${url}" >&2
    exit 1
    ;;
esac
EOF
chmod +x "${FAKE_BIN}/curl"

PATH="${FAKE_BIN}:${PATH}" \
BUILDKITE_API_TOKEN=fake-token \
CURL_PAYLOAD_FILE="${PAYLOAD_FILE}" \
bash "${ROOT_DIR}/tools/trigger_build.sh" \
  --org test-org \
  --pipeline test-pipeline \
  --branch feature/test \
  --commit abcdef1234567890 \
  --message "test trigger" \
  --rust-functionsystem-e2e \
  --functionsystem-repo https://gitcode.com/openeuler/yuanrong-functionsystem.git \
  --functionsystem-branch rust-rewrite \
  --rust-builder-image swr.example/compile-ubuntu2004-rust:test \
  --poll-interval 0 \
  --artifacts-dir "${ARTIFACTS_DIR}"

python3 - "${PAYLOAD_FILE}" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    payload = json.load(f)

expected_env = {
    "BUILD_TARGET": "linux",
    "ENABLE_RUST_FUNCTIONSYSTEM_ST": "true",
    "FUNCTIONSYSTEM_REPO": "https://gitcode.com/openeuler/yuanrong-functionsystem.git",
    "FUNCTIONSYSTEM_BRANCH": "rust-rewrite",
    "RUST_BUILDER_IMAGE": "swr.example/compile-ubuntu2004-rust:test",
}

for key, value in expected_env.items():
    actual = payload["env"].get(key)
    if actual != value:
        raise SystemExit(f"payload env {key}={actual!r}, expected {value!r}")
PY

EXPECTED="${ARTIFACTS_DIR}/artifacts/rust-fs-st/driver/python_output.txt"
if [[ "$(cat "${EXPECTED}")" != "PYTHON ST PASS" ]]; then
  echo "artifact was not downloaded to the expected path" >&2
  exit 1
fi

if [[ -e "${ARTIFACTS_DIR}/artifacts/rust-fs-st/driver/incomplete.txt" ]]; then
  echo "unfinished artifact should not be downloaded" >&2
  exit 1
fi

if [[ -e "${ARTIFACTS_DIR}/artifacts/release/openyuanrong.whl" ]]; then
  echo "artifact pattern should filter unrelated artifacts" >&2
  exit 1
fi


FAILED_ARTIFACTS_DIR="${TMP_DIR}/failed-artifacts"
set +e
PATH="${FAKE_BIN}:${PATH}" \
BUILDKITE_API_TOKEN=fake-token \
CURL_PAYLOAD_FILE="${TMP_DIR}/failed-payload.json" \
FAKE_BUILD_STATE=failed \
bash "${ROOT_DIR}/tools/trigger_build.sh" \
  --org test-org \
  --pipeline test-pipeline \
  --branch feature/test \
  --commit abcdef1234567890 \
  --message "test trigger failed" \
  --rust-functionsystem-e2e \
  --poll-interval 0 \
  --artifacts-dir "${FAILED_ARTIFACTS_DIR}"
FAILED_EXIT=$?
set -e

if [[ "${FAILED_EXIT}" -eq 0 ]]; then
  echo "failed build should make trigger script exit non-zero" >&2
  exit 1
fi

FAILED_EXPECTED="${FAILED_ARTIFACTS_DIR}/artifacts/rust-fs-st/driver/python_output.txt"
if [[ "$(cat "${FAILED_EXPECTED}")" != "PYTHON ST PASS" ]]; then
  echo "failed build artifacts should be downloaded before non-zero exit" >&2
  exit 1
fi

set +e
PATH="${FAKE_BIN}:${PATH}" \
BUILDKITE_API_TOKEN=fake-token \
bash "${ROOT_DIR}/tools/trigger_build.sh" --poll-interval >"${TMP_DIR}/missing-value.out" 2>"${TMP_DIR}/missing-value.err"
MISSING_VALUE_EXIT=$?
set -e

if [[ "${MISSING_VALUE_EXIT}" -eq 0 ]]; then
  echo "missing option value should fail" >&2
  exit 1
fi

if ! grep -q -- "--poll-interval requires a value" "${TMP_DIR}/missing-value.err"; then
  echo "missing option value should report a clear error" >&2
  exit 1
fi
