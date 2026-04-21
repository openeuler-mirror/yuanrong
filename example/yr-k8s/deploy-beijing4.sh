#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
HELM_BIN="${HELM_BIN:-/home/wyc/.local/bin/helm}"
KUBECTL_BIN="${KUBECTL_BIN:-kubectl}"

KUBECONFIG_PATH="${YR_K8S_KUBECONFIG:-${HOME}/.kube/beijing4.yaml}"
RELEASE_NAME="${YR_K8S_RELEASE:-yr-k8s}"
NAMESPACE="${YR_K8S_NAMESPACE:-yr}"
VALUES_FILE="${YR_K8S_VALUES_FILE:-${SCRIPT_DIR}/k8s/values.local.yaml}"

REGISTRY_SERVER="${YR_K8S_REGISTRY_SERVER:-swr.cn-southwest-2.myhuaweicloud.com}"
REGISTRY_REPO="${YR_K8S_REGISTRY_REPO:-swr.cn-southwest-2.myhuaweicloud.com/yuanrong-dev}"
IMAGE_TAG="${YR_K8S_IMAGE_TAG:?Set YR_K8S_IMAGE_TAG to the pushed image tag}"
PULL_SECRET_NAME="${YR_K8S_PULL_SECRET_NAME:-yr-swr-pull}"

SWR_USERNAME="${SWR_USERNAME:-}"
SWR_PASSWORD="${SWR_PASSWORD:-}"

require_bin() {
  local bin_name="$1"
  if ! command -v "${bin_name}" >/dev/null 2>&1; then
    printf 'Missing required CLI: %s\n' "${bin_name}" >&2
    exit 1
  fi
}

create_namespace() {
  "${KUBECTL_BIN}" --kubeconfig "${KUBECONFIG_PATH}" create namespace "${NAMESPACE}" --dry-run=client -o yaml \
    | "${KUBECTL_BIN}" --kubeconfig "${KUBECONFIG_PATH}" apply -f -
}

create_or_update_pull_secret() {
  if [ -z "${SWR_USERNAME}" ] || [ -z "${SWR_PASSWORD}" ]; then
    printf 'Skipping pull secret creation because SWR_USERNAME/SWR_PASSWORD are not set.\n' >&2
    return 0
  fi

  "${KUBECTL_BIN}" --kubeconfig "${KUBECONFIG_PATH}" create secret docker-registry "${PULL_SECRET_NAME}" \
    --namespace "${NAMESPACE}" \
    --docker-server="${REGISTRY_SERVER}" \
    --docker-username="${SWR_USERNAME}" \
    --docker-password="${SWR_PASSWORD}" \
    --dry-run=client -o yaml \
    | "${KUBECTL_BIN}" --kubeconfig "${KUBECONFIG_PATH}" apply -f -
}

helm_deploy() {
  "${HELM_BIN}" upgrade --install "${RELEASE_NAME}" "${SCRIPT_DIR}/charts/yr-k8s" \
    --kubeconfig "${KUBECONFIG_PATH}" \
    --namespace "${NAMESPACE}" \
    --create-namespace \
    -f "${VALUES_FILE}" \
    --set global.namespace.create=false \
    --set global.namespace.name="${NAMESPACE}" \
    --set global.imageRegistry="${REGISTRY_REPO}" \
    --set global.images.master.repository="yr-master" \
    --set global.images.master.tag="${IMAGE_TAG}" \
    --set global.images.frontend.repository="yr-frontend" \
    --set global.images.frontend.tag="${IMAGE_TAG}" \
    --set global.images.node.repository="yr-node" \
    --set global.images.node.tag="${IMAGE_TAG}" \
    --set global.images.traefik.repository="traefik" \
    --set global.images.traefik.tag="${IMAGE_TAG}"
}

patch_workloads_with_pull_secret() {
  if [ -z "${SWR_USERNAME}" ] || [ -z "${SWR_PASSWORD}" ]; then
    printf 'Skipping workload imagePullSecrets patch because no registry credentials were provided.\n' >&2
    return 0
  fi

  local resource
  while IFS= read -r resource; do
    [ -n "${resource}" ] || continue
    "${KUBECTL_BIN}" --kubeconfig "${KUBECONFIG_PATH}" patch "${resource}" \
      --namespace "${NAMESPACE}" \
      --type merge \
      -p "{\"spec\":{\"template\":{\"spec\":{\"imagePullSecrets\":[{\"name\":\"${PULL_SECRET_NAME}\"}]}}}}"
  done < <("${KUBECTL_BIN}" --kubeconfig "${KUBECONFIG_PATH}" get deploy,statefulset,daemonset -n "${NAMESPACE}" -o name)
}

wait_for_rollout() {
  local resource
  while IFS= read -r resource; do
    [ -n "${resource}" ] || continue
    "${KUBECTL_BIN}" --kubeconfig "${KUBECONFIG_PATH}" rollout status "${resource}" \
      --namespace "${NAMESPACE}" \
      --timeout="${YR_K8S_ROLLOUT_TIMEOUT:-10m}"
  done < <("${KUBECTL_BIN}" --kubeconfig "${KUBECONFIG_PATH}" get deploy,statefulset,daemonset -n "${NAMESPACE}" -o name)
}

show_status() {
  printf '\nNamespace: %s\n' "${NAMESPACE}" >&2
  "${KUBECTL_BIN}" --kubeconfig "${KUBECONFIG_PATH}" get pods,svc,statefulset,deploy,daemonset -n "${NAMESPACE}"
}

main() {
  require_bin "${KUBECTL_BIN}"
  require_bin "${HELM_BIN}"

  if [ ! -f "${KUBECONFIG_PATH}" ]; then
    printf 'Missing kubeconfig: %s\n' "${KUBECONFIG_PATH}" >&2
    exit 1
  fi

  create_namespace
  create_or_update_pull_secret
  helm_deploy
  patch_workloads_with_pull_secret
  wait_for_rollout
  show_status
}

main "$@"
