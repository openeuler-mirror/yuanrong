#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${ROOT_DIR}/../.." && pwd)"
OUTPUT_DIR="${YR_K8S_OUTPUT_DIR:-${REPO_ROOT}/output}"
RUNTIME_LAUNCHER_PATH="${YR_K8S_RUNTIME_LAUNCHER_PATH:-${REPO_ROOT}/functionsystem/runtime-launcher/bin/runtime/runtime-launcher}"
DOCKER_BIN="${DOCKER_BIN:-docker}"
CONTROLPLANE_BASE_IMAGE="${YR_CONTROLPLANE_BASE_IMAGE:-yr-controlplane-base}"
MASTER_IMAGE="${YR_MASTER_IMAGE:-yr-master}"
FRONTEND_IMAGE="${YR_FRONTEND_IMAGE:-yr-frontend}"
NODE_IMAGE="${YR_NODE_IMAGE:-yr-node}"

resolve_single_artifact() {
  local source_dir="$1"
  local pattern="$2"
  local matches=()

  mapfile -t matches < <(find "${source_dir}" -maxdepth 1 -type f -name "${pattern}" -printf '%p\n' | sort -V)
  if [ "${#matches[@]}" -eq 0 ]; then
    return 1
  fi
  if [ "${#matches[@]}" -ne 1 ]; then
    printf 'Expected exactly one artifact matching %s in %s, found %s\n' "${pattern}" "${source_dir}" "${#matches[@]}" >&2
    printf '%s\n' "${matches[@]}" >&2
    exit 1
  fi

  printf '%s\n' "${matches[0]}"
}

require_file() {
  local path="$1"
  if [ ! -f "${path}" ]; then
    printf 'Missing required artifact: %s\n' "${path}" >&2
    printf 'Fail-fast: required build output not found. Run: make all\n' >&2
    exit 1
  fi
}

build_image() {
  local image_name="$1"
  local dockerfile_path="$2"

  printf 'Building %s from %s\n' "${image_name}" "${dockerfile_path}" >&2
  "${DOCKER_BIN}" build \
    -t "${image_name}" \
    -f "${ROOT_DIR}/${dockerfile_path}" \
    "${REPO_ROOT}"
}

main() {
  local openyuanrong_wheel
  local sdk_wheel
  local frontend_package

  if ! command -v "${DOCKER_BIN}" >/dev/null 2>&1; then
    printf 'Missing container build CLI: %s\n' "${DOCKER_BIN}" >&2
    printf 'Fail-fast: docker build entrypoint cannot run without a working container CLI.\n' >&2
    exit 1
  fi

  if [ ! -d "${OUTPUT_DIR}" ]; then
    printf 'Missing output directory: %s\n' "${OUTPUT_DIR}" >&2
    printf 'Fail-fast: required build output directory not found. Run: make all\n' >&2
    exit 1
  fi

  openyuanrong_wheel="$(resolve_single_artifact "${OUTPUT_DIR}" 'openyuanrong-*.whl')"
  sdk_wheel="$(resolve_single_artifact "${OUTPUT_DIR}" 'openyuanrong_sdk*.whl')"
  frontend_package="$(resolve_single_artifact "${OUTPUT_DIR}" 'yr-frontend*.tar.gz')"
  require_file "${RUNTIME_LAUNCHER_PATH}"

  printf 'yr-k8s build entrypoint: building docker images from %s.\n' "${REPO_ROOT}" >&2
  printf 'Using artifacts from %s\n' "${OUTPUT_DIR}" >&2
  printf '  openyuanrong wheel: %s\n' "${openyuanrong_wheel}" >&2
  printf '  openyuanrong sdk wheel: %s\n' "${sdk_wheel}" >&2
  printf '  frontend package: %s\n' "${frontend_package}" >&2
  printf '  runtime-launcher: %s\n' "${RUNTIME_LAUNCHER_PATH}" >&2

  build_image "${CONTROLPLANE_BASE_IMAGE}" "images/Dockerfile.controlplane-base"
  build_image "${MASTER_IMAGE}" "images/Dockerfile.master"
  build_image "${FRONTEND_IMAGE}" "images/Dockerfile.frontend"
  build_image "${NODE_IMAGE}" "images/Dockerfile.node"

  printf 'Image builds completed: %s, %s, %s, %s\n' \
    "${CONTROLPLANE_BASE_IMAGE}" \
    "${MASTER_IMAGE}" \
    "${FRONTEND_IMAGE}" \
    "${NODE_IMAGE}" >&2
}

main "$@"
