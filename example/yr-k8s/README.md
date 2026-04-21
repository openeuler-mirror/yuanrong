# yr-k8s

`example/yr-k8s` is the Kubernetes deployment surface for the split `yr` images and Helm chart tracked in [docs/superpowers/plans/2026-04-19-yr-k8s.md](/home/wyc/code/sandbox/docs/superpowers/plans/2026-04-19-yr-k8s.md).

This directory currently contains:

- image build scaffolding for `yr-controlplane-base`, `yr-master`, `yr-frontend`, and `yr-node`
- `yr start --block` entrypoint wrappers for `master`, `frontend`, and `node`
- a Helm chart for the three active workloads plus support objects and Traefik
- local and production values overlays

## Build inputs

Run `make all` from the repository root before invoking `example/yr-k8s/build-images.sh`.

The build script reads artifacts directly from the repository outputs and then builds the local `yr-controlplane-base`, `yr-master`, `yr-frontend`, and `yr-node` images.

Required build outputs:

- `openyuanrong-*.whl`
- `openyuanrong_sdk*.whl`
- `yr-frontend*.tar.gz`
- `functionsystem/runtime-launcher/bin/runtime/runtime-launcher`

## Current workflow

1. Produce build artifacts:

```bash
make all
```

2. Build the local images directly from `output/`:

```bash
bash example/yr-k8s/build-images.sh
```

3. Push the built images to SWR when needed:

```bash
bash example/yr-k8s/push-images-swr.sh
```

4. Run the focused scaffold and chart tests:

```bash
python3 -m pytest example/yr-k8s/tests/test_yr_k8s_layout.py -q
```

5. Lint the Helm chart:

```bash
helm lint example/yr-k8s/charts/yr-k8s -f example/yr-k8s/k8s/values.local.yaml
```

6. Render the chart:

```bash
helm template yr-k8s example/yr-k8s/charts/yr-k8s
helm template yr-k8s example/yr-k8s/charts/yr-k8s -f example/yr-k8s/k8s/values.local.yaml
helm template yr-k8s example/yr-k8s/charts/yr-k8s -f example/yr-k8s/k8s/values.prod.yaml
```

7. Deploy to the Beijing4 cluster when ready:

```bash
bash example/yr-k8s/deploy-beijing4.sh
```

## Beijing4 Runbook

This is the shortest reproducible flow that was validated against the `~/.kube/beijing4.yaml` cluster.

1. Rebuild the frontend artifact:

```bash
make frontend
```

2. Rebuild the local role images:

```bash
bash example/yr-k8s/build-images.sh
```

3. Login to SWR:

```bash
docker login -u cn-southwest-2@HPUAJOVNXVGC7T4NRLH9 -p 383e235bee1005fe3062aac27e10c5fbca371e29fced903c369e9825198f8ac3 swr.cn-southwest-2.myhuaweicloud.com
```

4. Push a release tag:

```bash
YR_K8S_IMAGE_TAG=20260421-rerun bash example/yr-k8s/push-images-swr.sh
```

5. Deploy to the Beijing4 cluster:

```bash
helm --kubeconfig ~/.kube/beijing4.yaml upgrade --install yr-k8s example/yr-k8s/charts/yr-k8s \
  -n yr --create-namespace \
  -f example/yr-k8s/k8s/values.local.yaml \
  --set global.namespace.create=false \
  --set global.imageRegistry=swr.cn-southwest-2.myhuaweicloud.com/yuanrong-dev \
  --set global.images.master.repository=yr-master \
  --set global.images.master.tag=20260421-rerun \
  --set global.images.frontend.repository=yr-frontend \
  --set global.images.frontend.tag=20260421-rerun \
  --set global.images.node.repository=yr-node \
  --set global.images.node.tag=20260421-rerun \
  --set global.images.traefik.repository=traefik \
  --set global.images.traefik.tag=20260421-rerun
```

6. Check the workload state:

```bash
kubectl --kubeconfig ~/.kube/beijing4.yaml get pods -n yr -o wide
kubectl --kubeconfig ~/.kube/beijing4.yaml get svc -n yr
```

7. Validate the external SDK flow with only `YR_SERVER_ADDRESS`:

```bash
export YR_SERVER_ADDRESS=114.116.246.103:18888
export YR_ENABLE_TLS=false
export YR_IN_CLUSTER=false
unset YR_DS_ADDRESS

python3 - <<'PY'
import yr

yr.init()
sb = yr.sandbox.create()
try:
    print(yr.get(sb.exec('python3 -c "print(40+2)"', 60)))
finally:
    sb.terminate()
    yr.finalize()
PY
```

The verified success signal is that the sandbox command exits with `returncode: 0` and prints `42`.

## Values overlays

`example/yr-k8s/k8s/values.local.yaml`

- local image tags
- local registry override
- developer-oriented namespace and service exposure defaults

`example/yr-k8s/k8s/values.prod.yaml`

- stable image tags
- production registry override
- higher replica counts for control-plane workloads
- production storage and scale defaults

## Notes

- the active workload model is `master`, `frontend`, and `node`
- `master` uses `yr start --master --block true -e` and additionally enables scheduler, meta-service, and iam-server
- `frontend` uses `yr start --block true -e --enable_faas_frontend true`
- `node` uses `yr start --block true -e`
- `etcd` is external in this model and is passed in through `global.externalEtcd`
- `datasystem` validates `etcd_address` strictly, so local deployments should use an IP or full service FQDN such as `yr-etcd.yr.svc.cluster.local:2379`, not a bare short service name
- `helm` is required locally for linting and rendering. If `helm` is missing, chart verification is incomplete.
