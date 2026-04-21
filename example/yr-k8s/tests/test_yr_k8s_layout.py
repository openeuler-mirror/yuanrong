import pathlib
import stat
import subprocess
import unittest

import yaml


ROOT = pathlib.Path(__file__).resolve().parents[1]
HELM_BIN = pathlib.Path("/home/wyc/.local/bin/helm")
RELEASE = "yr-k8s"
NAMESPACE = "yr-k8s"

ACTIVE_SCRIPTS = [
    "bin/start-master.sh",
    "bin/start-frontend.sh",
    "bin/start-node.sh",
    "bin/supervisord-node-entrypoint.sh",
]

RETIRED_SCRIPTS = [
    "bin/start-scheduler.sh",
    "bin/start-meta-service.sh",
    "bin/start-iam.sh",
    "bin/start-runtime-manager.sh",
    "bin/start-function-agent.sh",
    "bin/start-function-proxy.sh",
    "bin/start-ds-worker.sh",
]

ACTIVE_TEMPLATES = [
    "charts/yr-k8s/templates/_helpers.tpl",
    "charts/yr-k8s/templates/namespace.yaml",
    "charts/yr-k8s/templates/secrets.yaml",
    "charts/yr-k8s/templates/components-configmap.yaml",
    "charts/yr-k8s/templates/services-configmap.yaml",
    "charts/yr-k8s/templates/master-serviceaccount.yaml",
    "charts/yr-k8s/templates/master-role.yaml",
    "charts/yr-k8s/templates/master-rolebinding.yaml",
    "charts/yr-k8s/templates/master-statefulset.yaml",
    "charts/yr-k8s/templates/master-service.yaml",
    "charts/yr-k8s/templates/node-serviceaccount.yaml",
    "charts/yr-k8s/templates/node-role.yaml",
    "charts/yr-k8s/templates/node-rolebinding.yaml",
    "charts/yr-k8s/templates/node-daemonset.yaml",
    "charts/yr-k8s/templates/frontend-deployment.yaml",
    "charts/yr-k8s/templates/frontend-service.yaml",
    "charts/yr-k8s/templates/traefik-configmap.yaml",
    "charts/yr-k8s/templates/traefik-dynamic-configmap.yaml",
    "charts/yr-k8s/templates/traefik-deployment.yaml",
    "charts/yr-k8s/templates/traefik-service.yaml",
]

RETIRED_TEMPLATES = [
    "charts/yr-k8s/templates/agent-pool-deployment.yaml",
    "charts/yr-k8s/templates/etcd-statefulset.yaml",
    "charts/yr-k8s/templates/etcd-service.yaml",
    "charts/yr-k8s/templates/frontend-configmap.yaml",
    "charts/yr-k8s/templates/function-agent-configmap.yaml",
    "charts/yr-k8s/templates/iam-deployment.yaml",
    "charts/yr-k8s/templates/iam-policy-config.yaml",
    "charts/yr-k8s/templates/iam-service.yaml",
    "charts/yr-k8s/templates/meta-service-configmap.yaml",
    "charts/yr-k8s/templates/meta-service-deployment.yaml",
    "charts/yr-k8s/templates/meta-service-service.yaml",
    "charts/yr-k8s/templates/scheduler-configmap.yaml",
    "charts/yr-k8s/templates/scheduler-deployment.yaml",
    "charts/yr-k8s/templates/scheduler-role.yaml",
    "charts/yr-k8s/templates/scheduler-rolebinding.yaml",
    "charts/yr-k8s/templates/scheduler-service.yaml",
    "charts/yr-k8s/templates/scheduler-serviceaccount.yaml",
]


def assert_paths_exist(test_case: unittest.TestCase, relative_paths: list[str]) -> None:
    for relative_path in relative_paths:
        with test_case.subTest(path=relative_path):
            test_case.assertTrue((ROOT / relative_path).exists(), f"{relative_path} should exist")


def assert_paths_absent(test_case: unittest.TestCase, relative_paths: list[str]) -> None:
    for relative_path in relative_paths:
        with test_case.subTest(path=relative_path):
            test_case.assertFalse((ROOT / relative_path).exists(), f"{relative_path} should be removed")


def load_yaml_file(path: pathlib.Path):
    return yaml.safe_load(path.read_text())


def render_chart(*extra_args: str) -> list[dict]:
    result = subprocess.run(
        [
            str(HELM_BIN),
            "template",
            RELEASE,
            str(ROOT / "charts/yr-k8s"),
            "--namespace",
            NAMESPACE,
            *extra_args,
        ],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    return [doc for doc in yaml.safe_load_all(result.stdout) if doc]


def find_manifest(manifests: list[dict], kind: str, name: str) -> dict:
    for manifest in manifests:
        if manifest.get("kind") == kind and manifest.get("metadata", {}).get("name") == name:
            return manifest
    raise AssertionError(f"{kind} {name} should exist")


def find_container(manifest: dict, container_name: str) -> dict:
    for container in manifest["spec"]["template"]["spec"]["containers"]:
        if container["name"] == container_name:
            return container
    raise AssertionError(f"container {container_name} should exist")


def find_env(container: dict, name: str) -> str:
    for env_var in container.get("env", []):
        if env_var["name"] == name:
            return env_var["value"]
    raise AssertionError(f"env {name} should exist")


def expected_image(values: dict, image_name: str) -> str:
    registry = values["global"]["imageRegistry"].rstrip("/")
    image = values["global"]["images"][image_name]
    return f"{registry}/{image['repository']}:{image['tag']}"


class YrK8sLayoutTests(unittest.TestCase):
    def test_surface_tree_matches_three_workload_model(self):
        assert_paths_exist(
            self,
            [
                "README.md",
                "build-images.sh",
                "push-images-swr.sh",
                "deploy-beijing4.sh",
                "images/Dockerfile.controlplane-base",
                "images/Dockerfile.master",
                "images/Dockerfile.frontend",
                "images/Dockerfile.node",
                "supervisor/supervisord-node.conf",
                "charts/yr-k8s/Chart.yaml",
                "charts/yr-k8s/values.yaml",
                "k8s/values.local.yaml",
                "k8s/values.prod.yaml",
                *ACTIVE_SCRIPTS,
                *ACTIVE_TEMPLATES,
            ],
        )
        assert_paths_absent(self, RETIRED_SCRIPTS + RETIRED_TEMPLATES)

    def test_build_and_readme_document_real_image_builds(self):
        build_script = (ROOT / "build-images.sh").read_text()
        readme = (ROOT / "README.md").read_text().lower()

        for token in [
            "output",
            "functionsystem/runtime-launcher/bin/runtime/runtime-launcher",
            "docker build",
            "yr-controlplane-base",
            "yr-master",
            "yr-frontend",
            "yr-node",
        ]:
            with self.subTest(token=token):
                self.assertIn(token, build_script)

        self.assertIn("bash example/yr-k8s/build-images.sh", readme)
        self.assertIn("push-images-swr.sh", readme)
        self.assertIn("deploy-beijing4.sh", readme)
        self.assertNotIn("scaffold only", readme)
        self.assertNotIn("example/yr-k8s/pkg/", readme)

    def test_master_start_script_drops_optional_alias_helper(self):
        text = (ROOT / "bin/start-master.sh").read_text()
        self.assertNotIn("control_plane_alias.sh", text)
        self.assertNotIn("YR_CONTROLPLANE_HELPER_DIR", text)

    def test_dockerfiles_read_artifacts_directly_from_repo_outputs(self):
        controlplane_base = (ROOT / "images/Dockerfile.controlplane-base").read_text()
        frontend_dockerfile = (ROOT / "images/Dockerfile.frontend").read_text()
        build_script = (ROOT / "build-images.sh").read_text()

        self.assertIn("COPY output/openyuanrong-*.whl /tmp/pkg/", controlplane_base)
        self.assertIn("COPY output/openyuanrong_sdk*.whl /tmp/pkg/", controlplane_base)
        self.assertIn(
            "COPY functionsystem/runtime-launcher/bin/runtime/runtime-launcher /openyuanrong/runtime-launcher",
            controlplane_base,
        )
        self.assertNotIn("YR_CONTROLPLANE_HELPER_DIR", controlplane_base)
        self.assertIn("COPY output/yr-frontend*.tar.gz /tmp/pkg/", frontend_dockerfile)
        self.assertNotIn("PKG_DIR=", build_script)
        self.assertIn('OUTPUT_DIR="${YR_K8S_OUTPUT_DIR:-${REPO_ROOT}/output}"', build_script)

    def test_container_ep_only_lives_in_node_image(self):
        controlplane_base = (ROOT / "images/Dockerfile.controlplane-base").read_text()
        node_dockerfile = (ROOT / "images/Dockerfile.node").read_text()
        node_supervisord = (ROOT / "supervisor/supervisord-node.conf").read_text()
        node_entrypoint = (ROOT / "bin/supervisord-node-entrypoint.sh").read_text()

        self.assertNotIn("CONTAINER_EP=", controlplane_base)
        self.assertIn("CONTAINER_EP=", node_dockerfile)
        self.assertIn("supervisor", node_dockerfile)
        self.assertIn("docker.io", node_dockerfile)
        self.assertIn("supervisord-node-entrypoint.sh", node_dockerfile)
        self.assertIn("COPY supervisor/supervisord-node.conf", node_dockerfile)
        self.assertIn("[program:runtime-launcher]", node_supervisord)
        self.assertIn("[program:yr-node]", node_supervisord)
        self.assertIn("dockerd", node_entrypoint)
        self.assertIn("docker info", node_entrypoint)

    def test_start_scripts_use_yr_start_block_model(self):
        expectations = {
            "bin/start-master.sh": [
                "/usr/local/bin/yr start",
                "--master",
                "--block true",
                "-e",
                "--port_policy FIX",
                "--etcd_mode outter",
                "--enable_function_scheduler true",
                "--enable_meta_service true",
                "--enable_iam_server true",
            ],
            "bin/start-frontend.sh": [
                "/usr/local/bin/yr start",
                "--block true",
                "-e",
                "--port_policy FIX",
                "--etcd_mode outter",
                "--enable_faas_frontend true",
            ],
            "bin/start-node.sh": [
                "/usr/local/bin/yr start",
                "--block true",
                "-e",
                "--port_policy FIX",
                "--etcd_mode outter",
            ],
        }
        for relative_path, tokens in expectations.items():
            path = ROOT / relative_path
            text = path.read_text()
            mode = path.stat().st_mode
            with self.subTest(path=relative_path):
                self.assertTrue(mode & stat.S_IXUSR, f"{relative_path} should be executable")
                self.assertTrue(text.startswith("#!/usr/bin/env bash"))
                self.assertIn("set -euo pipefail", text)
                self.assertNotIn("python3 -m yr.cli.main", text)
                for token in tokens:
                    self.assertIn(token, text)

    def test_values_surface_matches_external_etcd_three_workload_model(self):
        values = load_yaml_file(ROOT / "charts/yr-k8s/values.yaml")

        for section in ["global", "master", "frontend", "node", "traefik", "debug"]:
            self.assertIn(section, values)
        for retired in ["scheduler", "meta-service", "iam", "agent-pool", "etcd"]:
            self.assertNotIn(retired, values)

        self.assertIn("externalEtcd", values["global"])
        self.assertEqual(sorted(values["global"]["images"].keys()), ["frontend", "master", "node", "traefik"])
        self.assertEqual(
            sorted(values["node"]["ports"].keys()),
            ["dsWorker", "functionProxy", "functionProxyGrpc"],
        )
        self.assertNotIn("envPorts", values["frontend"])
        self.assertNotIn("faasFrontend", values["frontend"])

    def test_rendered_manifests_match_three_workload_model(self):
        values = load_yaml_file(ROOT / "charts/yr-k8s/values.yaml")
        manifests = render_chart()

        names = {doc["metadata"]["name"] for doc in manifests if "metadata" in doc}
        master_name = "yr-master"
        master_headless_name = "yr-master-headless"
        master_access_name = "yr-master-access"
        frontend_name = "yr-frontend"
        node_name = "yr-node"
        traefik_name = "yr-traefik"
        services_name = "yr-services"

        for retired_name in [
            "yr-scheduler",
            "yr-meta-service",
            "yr-iam-adaptor",
            "yr-agent-pool",
            "yr-etcd",
        ]:
            self.assertNotIn(retired_name, names)

        master_sts = find_manifest(manifests, "StatefulSet", master_name)
        master_headless_svc = find_manifest(manifests, "Service", master_headless_name)
        master_access_svc = find_manifest(manifests, "Service", master_access_name)
        frontend_dep = find_manifest(manifests, "Deployment", frontend_name)
        frontend_svc = find_manifest(manifests, "Service", frontend_name)
        node_ds = find_manifest(manifests, "DaemonSet", node_name)
        traefik_dep = find_manifest(manifests, "Deployment", traefik_name)
        traefik_svc = find_manifest(manifests, "Service", traefik_name)
        services_cm = find_manifest(manifests, "ConfigMap", services_name)

        self.assertEqual(master_sts["spec"]["serviceName"], master_headless_name)
        self.assertEqual(master_headless_svc["spec"]["clusterIP"], "None")
        self.assertEqual(
            [p["port"] for p in master_headless_svc["spec"]["ports"]],
            [
                values["master"]["service"]["ports"]["master"],
                values["master"]["service"]["ports"]["metaService"],
                values["master"]["service"]["ports"]["iamServer"],
            ],
        )
        self.assertNotIn("clusterIP", master_access_svc["spec"])
        self.assertEqual(
            [p["port"] for p in master_access_svc["spec"]["ports"]],
            [
                values["master"]["service"]["ports"]["master"],
                values["master"]["service"]["ports"]["metaService"],
                values["master"]["service"]["ports"]["iamServer"],
            ],
        )
        self.assertIn("services.yaml", services_cm["data"])

        master_container = find_container(master_sts, "master")
        self.assertEqual(master_container["image"], expected_image(values, "master"))
        self.assertEqual(master_container["command"], ["/usr/local/bin/start-master.sh"])
        self.assertEqual(find_env(master_container, "YR_ETCD_ADDR_LIST"), values["global"]["externalEtcd"]["addrList"])
        master_mounts = {m["mountPath"] for m in master_container.get("volumeMounts", [])}
        self.assertIn(values["debug"]["sidecar"]["sessionDir"], master_mounts)

        frontend_container = find_container(frontend_dep, "frontend")
        self.assertEqual(frontend_container["image"], expected_image(values, "frontend"))
        self.assertEqual(frontend_container["command"], ["/usr/local/bin/start-frontend.sh"])
        self.assertEqual(find_env(frontend_container, "YR_MASTER_IP"), master_access_name)
        self.assertEqual(find_env(frontend_container, "YR_ETCD_ADDR_LIST"), values["global"]["externalEtcd"]["addrList"])
        self.assertEqual(find_env(frontend_container, "YR_META_SERVICE_ADDRESS"), f"{master_access_name}:31111")
        self.assertEqual(find_env(frontend_container, "IAM_SERVER_ADDRESS"), f"{master_access_name}:31112")
        self.assertEqual(frontend_svc["spec"]["ports"][0]["port"], values["frontend"]["service"]["port"])
        frontend_mounts = {m["mountPath"] for m in frontend_container.get("volumeMounts", [])}
        self.assertIn("/home/sn/service-config/services.yaml", frontend_mounts)
        self.assertIn("/etc/yuanrong/config.toml", frontend_mounts)
        self.assertIn(values["debug"]["sidecar"]["sessionDir"], frontend_mounts)
        self.assertNotIn("/home/sn/iam-config", frontend_mounts)
        frontend_env_names = {env_var["name"] for env_var in frontend_container.get("env", [])}
        self.assertNotIn("YR_FAAS_FRONTEND_HTTP_PORT", frontend_env_names)
        self.assertNotIn("FUNCTION_PROXY_PORT", frontend_env_names)
        self.assertNotIn("FUNCTION_PROXY_GRPC_PORT", frontend_env_names)
        self.assertNotIn("DS_WORKER_PORT", frontend_env_names)

        node_container = find_container(node_ds, "node")
        self.assertEqual(node_container["image"], expected_image(values, "node"))
        self.assertNotIn("command", node_container)
        self.assertTrue(node_ds["spec"]["template"]["spec"]["hostNetwork"])
        self.assertTrue(node_container["securityContext"]["privileged"])
        self.assertEqual(find_env(node_container, "YR_MASTER_IP"), master_access_name)
        self.assertEqual(find_env(node_container, "YR_ETCD_ADDR_LIST"), values["global"]["externalEtcd"]["addrList"])
        node_env_names = {env_var["name"] for env_var in node_container.get("env", [])}
        self.assertNotIn("FUNCTION_PROXY_PORT", node_env_names)
        self.assertNotIn("FUNCTION_PROXY_GRPC_PORT", node_env_names)
        self.assertNotIn("DS_WORKER_PORT", node_env_names)
        self.assertEqual(
            sorted(p["hostPort"] for p in node_container["ports"]),
            sorted([
                values["node"]["ports"]["functionProxy"]["hostPort"],
                values["node"]["ports"]["functionProxyGrpc"]["hostPort"],
                values["node"]["ports"]["dsWorker"]["hostPort"],
            ]),
        )
        node_mounts = {m["mountPath"] for m in node_container.get("volumeMounts", [])}
        self.assertNotIn("/proc/1", node_mounts)
        self.assertIn(values["debug"]["sidecar"]["sessionDir"], node_mounts)

        traefik_cfg = find_manifest(manifests, "ConfigMap", "yr-traefik-configmap")
        traefik_dynamic_cfg = find_manifest(manifests, "ConfigMap", "yr-traefik-dynamic")
        traefik_text = traefik_cfg["data"]["traefik.yaml"]
        traefik_dynamic_text = traefik_dynamic_cfg["data"]["config.yml"]
        self.assertIn(values["global"]["externalEtcd"]["addrList"], traefik_text)
        self.assertIn(values["traefik"]["etcd"]["rootKey"], traefik_text)
        self.assertIn("/etc/traefik/dynamic", traefik_text)
        self.assertIn(frontend_name, traefik_dynamic_text)
        self.assertIn("/serverless/v1/componentshealth", traefik_dynamic_text)
        self.assertIn("/invocations", traefik_dynamic_text)
        self.assertEqual(find_container(traefik_dep, "traefik")["image"], expected_image(values, "traefik"))
        self.assertEqual(traefik_svc["spec"]["ports"][0]["port"], values["traefik"]["service"]["port"])

        for manifest in [master_sts, frontend_dep, node_ds]:
            debug_container = find_container(manifest, "debug-busybox")
            debug_mounts = {m["mountPath"] for m in debug_container.get("volumeMounts", [])}
            self.assertIn(values["debug"]["sidecar"]["sessionDir"], debug_mounts)

        override_manifests = render_chart("--set", "frontend.iamServerAddress=iam.example.com:31112")
        override_frontend = find_manifest(override_manifests, "Deployment", frontend_name)
        override_frontend_container = find_container(override_frontend, "frontend")
        self.assertEqual(find_env(override_frontend_container, "IAM_SERVER_ADDRESS"), "iam.example.com:31112")


if __name__ == "__main__":
    unittest.main()
