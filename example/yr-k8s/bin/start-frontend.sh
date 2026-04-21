#!/usr/bin/env bash
set -euo pipefail

umask 0027

frontend_ip="${RUNTIME_POD_IP:-$(hostname -i | awk '{print $1}')}"
master_ip="${YR_MASTER_IP:?Set YR_MASTER_IP to the master service DNS name or IP}"
etcd_addr_list="${YR_ETCD_ADDR_LIST:?Set YR_ETCD_ADDR_LIST for external etcd, e.g. host1:2379,host2:2379}"
services_path="${YR_SERVICES_PATH:-/home/sn/service-config/services.yaml}"
meta_service_address="${YR_META_SERVICE_ADDRESS:-${master_ip}:31111}"

exec /usr/local/bin/yr start \
  --block true \
  -e \
  --port_policy FIX \
  --etcd_mode outter \
  --etcd_addr_list "${etcd_addr_list}" \
  --enable_faas_frontend true \
  --meta_service_address "${meta_service_address}" \
  -a "${frontend_ip}" \
  -p "${services_path}" \
  "$@"
