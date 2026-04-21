#!/usr/bin/env bash
set -euo pipefail

umask 0027

master_ip="${RUNTIME_POD_IP:-$(hostname -i | awk '{print $1}')}"
etcd_addr_list="${YR_ETCD_ADDR_LIST:?Set YR_ETCD_ADDR_LIST for external etcd, e.g. host1:2379,host2:2379}"
services_path="${YR_SERVICES_PATH:-/home/sn/service-config/services.yaml}"

exec /usr/local/bin/yr start \
  --master \
  --block true \
  -e \
  --port_policy FIX \
  --etcd_mode outter \
  --etcd_addr_list "${etcd_addr_list}" \
  --enable_function_scheduler true \
  --enable_meta_service true \
  --enable_iam_server true \
  -a "${master_ip}" \
  -p "${services_path}" \
  "$@"
