#!/usr/bin/env python3
# coding=UTF-8
# Copyright (c) Huawei Technologies Co., Ltd. 2025. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from datetime import datetime
import os
import shutil
import sys
import subprocess
import click
import json
import logging
import requests
from requests.exceptions import RequestException
from typing import Any, Dict, Optional

import yr


__server_address = None
__ds_address = None
__metaservice_address = None


class HTTPClient:
    """HTTP客户端"""

    def __init__(self, timeout: int = 30):
        self.timeout = timeout
        self.session = requests.Session()

    def request(
        self,
        url: str,
        data: Dict[str, Any],
        headers: Optional[Dict[str, str]] = None,
        method: str = "POST",
    ) -> Dict[str, Any]:
        """
        发送POST JSON请求

        Args:
            url: 目标URL
            data: JSON数据
            headers: 请求头

        Returns:
            响应数据
        """

        default_headers = {
            "Content-Type": "application/json",
            "User-Agent": "DeploymentScript/1.0",
        }
        if headers:
            default_headers.update(headers)

        logging.debug(f"发送POST请求到: {url}")
        logging.debug(f"请求数据: {json.dumps(data, indent=2, ensure_ascii=False)}")

        try:
            response = self.session.request(
                method.upper(),
                url,
                json=data,
                headers=default_headers,
                timeout=self.timeout,
            )
            response.raise_for_status()

            result = response.json() if response.content else {}
            logging.debug(f"请求成功，状态码: {response.status_code}, 响应数据: {json.dumps(result, indent=2, ensure_ascii=False)}")

            return {
                "success": True,
                "status_code": response.status_code,
                "data": result,
                "headers": dict(response.headers),
            }

        except RequestException as e:
            logging.debug(f"HTTP请求失败: {str(e)}")
            return {
                "success": False,
                "error": str(e),
                "status_code": (
                    getattr(e.response, "status_code", None)
                    if hasattr(e, "response")
                    else None
                ),
            }


class YRContext:
    def __init__(self, server_address, ds_address):
        self.__server_address = server_address
        self.__ds_address = ds_address

    def __enter__(self):
        cfg = yr.Config()
        cfg.log_dir = "/tmp/yr_sessions/driver"
        if self.__server_address and self.__ds_address:
            cfg.server_address = self.__server_address
            cfg.ds_address = self.__ds_address
            return yr.init(cfg)
        if self.__server_address:
            cfg.server_address = self.__server_address
            cfg.in_cluster = False
            return yr.init(cfg)
        return yr.init(cfg)

    def __exit__(self, exc_type, exc_value, exc_traceback):
        yr.finalize()
        return False


def deploy_function(function_json):
    http_client = HTTPClient(timeout=30)
    url = f"http://{__metaservice_address}/serverless/v1/functions"
    resp = http_client.request(url, function_json, method="POST")
    if resp["success"]:
        return True, resp["data"]["function"]["functionVersionUrn"]
    else:
        return False, resp


def update_function(function_json):
    name = function_json.get("name")
    if not name:
        raise RuntimeError("function name is required to update function.")
    http_client = HTTPClient(timeout=30)
    url = f"http://{__metaservice_address}/serverless/v1/functions/{name}"
    resp = http_client.request(url, function_json, method="PUT")
    if resp["success"]:
        return True, resp["data"]["result"]["functionVersionUrn"]
    else:
        return False, resp


def delete_function(function_name, version=None):
    http_client = HTTPClient(timeout=30)
    url = f"http://{__metaservice_address}/serverless/v1/functions/{function_name}?versionNumber={version if version else 'latest'}"
    resp = http_client.request(url, {}, method="DELETE")
    if resp["success"]:
        return True, None
    else:
        return False, resp


def query_function(function_name):
    http_client = HTTPClient(timeout=30)
    url = f"http://{__metaservice_address}/serverless/v1/functions/{function_name}?versionNumber=latest"
    resp = http_client.request(url, {}, method="GET")
    if resp["success"]:
        return True, resp["data"]["function"]
    else:
        return False, resp


def publish_function(function_name, publish_json):
    http_client = HTTPClient(timeout=30)
    url = f"http://{__metaservice_address}/serverless/v1/functions/{function_name}/versions"
    resp = http_client.request(url, publish_json, method="POST")
    if resp["success"]:
        return True, resp["data"]["function"]
    else:
        return False, resp


def package(backend, code_path, format):
    real_code_path = os.path.realpath(code_path)
    file_name = f"code-{datetime.now().strftime('%Y%m%d%H%M')}"
    archive_file = os.path.join("/tmp", file_name)
    if format == "zip":
        shutil.make_archive(archive_file, format, real_code_path)
    elif format == "img":
        result = subprocess.run(
            [
                "mkfs.erofs",
                "-E",
                "noinline_data",
                f"{archive_file}.{format}",
                real_code_path,
            ]
        )
        if result.returncode != 0:
            sys.exit(result.returncode)
    else:
        print(f"unkown format: {format}")
        sys.exit(1)
    if backend == "ds":
        with YRContext(__server_address, __ds_address):
            with open(f"{archive_file}.{format}", "rb") as f:
                yr.kv_write(file_name, f.read())

        package_key = f"ds://{file_name}.{format}"
    else:
        print("not support backend: %s" % backend)
        sys.exit(1)
    return real_code_path, package_key


@click.group()
@click.option(
    "--server-address",
    required=False,
    type=str,
)
@click.option(
    "--ds-address",
    required=False,
    type=str,
)
@click.option(
    "--metaservice-address", required=False, type=str, envvar="YR_METASERVICE_ADDRESS"
)
@click.option("--log-level", required=False, type=str, default="INFO")
@click.version_option(package_name="openyuanrong-sdk")
def cli(server_address, ds_address, metaservice_address, log_level):
    """
    run command
    """
    if server_address:
        global __server_address
        __server_address = server_address
    if ds_address:
        global __ds_address
        __ds_address = ds_address
    if metaservice_address:
        global __metaservice_address
        __metaservice_address = metaservice_address
    logging.basicConfig(level=getattr(logging, log_level.upper(), None))


@cli.command()
@click.option("--backend", required=False, type=str, default="ds")
@click.option("--code-path", required=False, type=str, default=".")
@click.option("--format", required=False, type=str, default="zip")
@click.option("--function-json", required=False, type=str, default=None)
@click.option("--skip-package", required=False, type=bool, default=False)
def deploy(backend, code_path, format, function_json, skip_package):
    if function_json:
        with open(function_json, "r") as f:
            function_json = json.load(f)
    if not skip_package:
        real_code_path, package_key = package(backend, code_path, format)
        if function_json:
            function_json["storageType"] = "working_dir"
            function_json["codePath"] = package_key
        else:
            print(
                f"""already upload {real_code_path} to {backend}.
export YR_WORKING_DIR={package_key} to set this package.
yrcli clear {package_key} to delete this package.
yrcli download {package_key} to download this package."""
            )
            return
    if function_json:
        name = function_json.get("name")
        if name is None:
            print("function name is required to deploy function.")
            sys.exit(1)
        query_ret, function_info = query_function(name)
        if query_ret:
            function_json["revisionId"] = function_info.get("revisionId")
            ret = update_function(function_json)
            if ret[0]:
                print(f"succeed to update function: {ret[1]}")
            else:
                print(f"failed to update function: {ret[1]['error']}")
        else:
            ret = deploy_function(function_json)
            if ret[0]:
                print(f"succeed to deploy function: {ret[1]}")
            else:
                print(f"failed to deploy function: {ret[1]['error']}")
    else:
        print("function json is required to deploy function.")


@cli.command()
@click.option("--function-name", required=False, type=str, default=None)
@click.option("--version", required=False, type=str, default=None)
@click.option("--kind", required=False, type=str, default=None)
def publish(function_name, version, kind):
    publish_json = {}
    query_ret, function_info = query_function(function_name)
    if query_ret == False:
        print(f"failed to query function: {function_info}")
        return
    print(f"succeed to get function: {function_info}")
    publish_json["revisionId"] = function_info.get("revisionId")
    publish_json["kind"] = kind if kind else "yrlib"
    if version:
        publish_json["versionNumber"] = version
    print(f"publish function: {publish_json}")
    ret = publish_function(function_name, publish_json)
    if ret[0]:
        print(f"succeed to publish function: {ret[1]}")
    else:
        print(f"failed to publish function: {ret[1]}")


@cli.command()
@click.option("--function-name", required=False, type=str, default=None)
@click.option("--clear-package-also", required=False, type=bool, default=True)
@click.option("--version", required=False, type=str, default=None)
def delete(function_name, clear_package_also, version):
    if clear_package_also:
        function_info = query_function(function_name)
        code_path = function_info.get("codePath")
        if code_path and code_path.startswith("ds://"):
            key = code_path.strip("ds://").split(".")[0]
            with YRContext(__server_address, __ds_address):
                yr.kv_del(key)
            print(f"succeed to del package {code_path}")
    ret = delete_function(function_name, version)
    print(f"succeed to delete function: {function_name}")


@cli.command
@click.argument("package", type=str)
def clear(package):
    if package.startswith("ds://"):
        key = package.strip("ds://").split(".")[0]
        with YRContext(__server_address, __ds_address):
            yr.kv_del(key)
        print(f"succeed to del {package}")


@cli.command
@click.argument("package", type=str)
def download(package):
    if package.startswith("ds://"):
        key = package.strip("ds://").split(".")[0]
        file_name = package.strip("ds://")
        with YRContext(__server_address, __ds_address):
            with open(file_name, "wb") as f:
                value = yr.kv_get(key)
                f.write(value)
        print(f"save {package} to {file_name}")


def main():
    return cli()


if __name__ == "__main__":
    main()
