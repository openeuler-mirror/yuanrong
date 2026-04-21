#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

REGISTRY_REPO="${YR_K8S_REGISTRY_REPO:-swr.cn-southwest-2.myhuaweicloud.com/yuanrong-dev}"
IMAGE_TAG="${YR_K8S_IMAGE_TAG:-$(date +%Y%m%d-%H%M%S)-$(git -C "${REPO_ROOT}" rev-parse --short HEAD)}"
DOCKER_BIN="${DOCKER_BIN:-docker}"
TRAEFIK_SOURCE_IMAGE="${YR_K8S_TRAEFIK_SOURCE_IMAGE:-traefik:v2.11.14}"

declare -A LOCAL_TO_REMOTE=(
  ["yr-controlplane-base"]="${REGISTRY_REPO}/yr-controlplane-base:${IMAGE_TAG}"
  ["yr-master"]="${REGISTRY_REPO}/yr-master:${IMAGE_TAG}"
  ["yr-frontend"]="${REGISTRY_REPO}/yr-frontend:${IMAGE_TAG}"
  ["yr-node"]="${REGISTRY_REPO}/yr-node:${IMAGE_TAG}"
  ["traefik"]="${REGISTRY_REPO}/traefik:${IMAGE_TAG}"
)

require_local_image() {
  local image_name="$1"
  if ! "${DOCKER_BIN}" image inspect "${image_name}:latest" >/dev/null 2>&1; then
    printf 'Missing local image: %s:latest\n' "${image_name}" >&2
    printf 'Build it first with: bash example/yr-k8s/build-images.sh\n' >&2
    exit 1
  fi
}

ensure_traefik_image() {
  if "${DOCKER_BIN}" image inspect "traefik:latest" >/dev/null 2>&1; then
    return 0
  fi

  printf 'Local traefik:latest not found, pulling %s\n' "${TRAEFIK_SOURCE_IMAGE}" >&2
  "${DOCKER_BIN}" pull "${TRAEFIK_SOURCE_IMAGE}"
  "${DOCKER_BIN}" tag "${TRAEFIK_SOURCE_IMAGE}" "traefik:latest"
}

push_image() {
  local local_image="$1"
  local remote_image="$2"

  printf 'Tagging %s:latest -> %s\n' "${local_image}" "${remote_image}" >&2
  "${DOCKER_BIN}" tag "${local_image}:latest" "${remote_image}"

  printf 'Pushing %s\n' "${remote_image}" >&2
  "${DOCKER_BIN}" push --platform linux/amd64 "${remote_image}"
}

main() {
  if ! command -v "${DOCKER_BIN}" >/dev/null 2>&1; then
    printf 'Missing container CLI: %s\n' "${DOCKER_BIN}" >&2
    exit 1
  fi

  printf 'Using target repository: %s\n' "${REGISTRY_REPO}" >&2
  printf 'Using image tag: %s\n' "${IMAGE_TAG}" >&2

  ensure_traefik_image

  for local_image in yr-controlplane-base yr-master yr-frontend yr-node traefik; do
    require_local_image "${local_image}"
  done

  for local_image in yr-controlplane-base yr-master yr-frontend yr-node traefik; do
    push_image "${local_image}" "${LOCAL_TO_REMOTE[${local_image}]}"
  done

  printf '\nPushed images:\n' >&2
  for local_image in yr-controlplane-base yr-master yr-frontend yr-node traefik; do
    printf '  %s\n' "${LOCAL_TO_REMOTE[${local_image}]}" >&2
  done

  printf '\nSuggested deploy command:\n' >&2
  printf '  helm upgrade --install yr-k8s example/yr-k8s/charts/yr-k8s \\\n' >&2
  printf '    -n yr --create-namespace \\\n' >&2
  printf '    -f example/yr-k8s/k8s/values.local.yaml \\\n' >&2
  printf '    --set global.imageRegistry=%s \\\n' "${REGISTRY_REPO}" >&2
  printf '    --set global.images.master.repository=yr-master \\\n' >&2
  printf '    --set global.images.master.tag=%s \\\n' "${IMAGE_TAG}" >&2
  printf '    --set global.images.frontend.repository=yr-frontend \\\n' >&2
  printf '    --set global.images.frontend.tag=%s \\\n' "${IMAGE_TAG}" >&2
  printf '    --set global.images.node.repository=yr-node \\\n' >&2
  printf '    --set global.images.node.tag=%s \\\n' "${IMAGE_TAG}" >&2
  printf '    --set global.images.traefik.repository=traefik \\\n' >&2
  printf '    --set global.images.traefik.tag=%s\n' "${IMAGE_TAG}" >&2
}

main "$@"
