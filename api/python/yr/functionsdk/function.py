#!/usr/bin/env python3
# coding=UTF-8
# Copyright (c) Huawei Technologies Co., Ltd. 2024. All rights reserved.
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

"""faas function sdk"""

from dataclasses import dataclass, field
import json
import os
import re
from typing import Tuple, Union, Dict, List

from yr import log
from yr.config import InvokeOptions as YRInvokeOptions
from yr.common import constants
from yr.common.constants import META_PREFIX
from yr.libruntime_pb2 import ApiType, FunctionMeta, LanguageType
from yr.object_ref import ObjectRef
from yr.runtime_holder import global_runtime
from yr.functionsdk.context import Context

DEFAULT_FUNCTION_VERSION = "latest"
DEFAULT_INVOKE_TIMEOUT = 900
DEFAULT_CONNECTION_NUMS = 128
DEFAULT_TENANT_ID = "12345678901234561234567890123456"

FUNC_NAME_REG = r'^[a-zA-Z]([a-zA-Z0-9_-]*[a-zA-Z0-9])?$'
FUNC_NAME_LENGTH_LIMIT = 60

VERSION_NAME_REG = r'^[a-zA-Z0-9]([a-zA-Z0-9_-]*\\.)*[a-zA-Z0-9_-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$'
VERSION_NAME_LENGTH_LIMIT = 32

ALIAS_PREFIX = "!"
ALIAS_NAME_REG = r'^[a-zA-Z]([a-zA-Z0-9_-]*[a-zA-Z0-9])?$'
ALIAS_NAME_LENGTH_LIMIT = 63

FUNCTION_NAME_SEPERATOR = "@"
SPLIT_NUM_OF_FUNC_ID = 1
FAAS_FUNCTION_RETURN_NUM = 1

ENV_KEY_RUNTIME_SERVICE_FUNC_VERSION = "RUNTIME_SERVICE_FUNC_VERSION"

_RUNTIME_MAX_RESP_BODY_SIZE = 6 * 1024 * 1024


@dataclass(init=True, repr=False, eq=False, order=False, unsafe_hash=False)
class InvokeOptions:
    """faas 调度参数。

    Examples:
        >>> from functionsdk import Function, InvokeOptions
        >>> opt = InvokeOptions()
        >>>
        >>> def my_handler(event, context)
        >>>     f = Function(context, "hello")
        >>>     objRef = f.invoke(event)
        >>>     res = objRef.get()
        >>>     return {
        >>>        "statusCode": 200,
        >>>        "isBase64Encoded": False,
        >>>        "body": res,
        >>>        "headers": {
        >>>             "Content-Type": "application/json"
        >>>         }
    """
    cpu: int = 0
    memory: int = 0
    concurrency: int = 100
    custom_resources: Dict[str, float] = field(default_factory=dict)
    pod_labels: Dict[str, str] = field(default_factory=dict)
    labels: List[str] = field(default_factory=list)
    alias_params: Dict[str, str] = field(default_factory=dict)


class CallReq:
    """call req"""

    def __init__(self, header: dict = {}, body: str = "") -> None:
        self.header = header
        self.body = body

    def __dict__(self) -> dict:
        return {'header': self.header, 'body': self.body}

    def encode(self) -> dict:
        """encode faas request """
        json_str = json.dumps(self.__dict__())
        return META_PREFIX + json_str


class Function:
    """function sdk"""

    def __init__(self, function_name: str, context_: Context = None) -> None:
        self.__function_name, self.__function_version = _check_function_name(function_name)
        self.__function_service = _get_service_name_from_env()
        self.__function_id = (f"{DEFAULT_TENANT_ID}/0@{self.__function_service}@{self.__function_name}"
                              f"/{self.__function_version}")
        self.invoke_options = InvokeOptions()
        self.context = context_

    def options(self, invoke_options: InvokeOptions):
        """
        Set user invoke options
        Args:
            invoke_options: invoke options for users to set resources
        """
        self.invoke_options = invoke_options
        return self

    def invoke(self, payload: Union[str, dict] = None) -> ObjectRef:
        """调用 faas 函数。
        Args:
        payload Union[str, dict]: 被调用 faas 函数的参数

        Returns:
            ObjectRef: 此次调用返回数据系统中的对象的 object_ref

        Examples:
            >>> from functionsdk import Function, InvokeOptions
            >>> def my_handler(event, context)
            >>>     f = Function(context, "hello")
            >>>     objRef = f.invoke(event)
            >>>     res = objRef.get()
            >>>     return {
            >>>        "statusCode": 200,
            >>>        "isBase64Encoded": False,
            >>>        "body": res,
            >>>        "headers": {
            >>>             "Content-Type": "application/json"
            >>>         }
        """
        func_meta = FunctionMeta(apiType=ApiType.Faas, functionID=self.__function_id, language=LanguageType.Python)
        payload_str = _check_payload(payload)
        call_req = CallReq(body=payload_str)
        args_list = [CallReq().encode(), call_req.encode()]

        obj_list = global_runtime.get_runtime().invoke_by_name(
            func_meta=func_meta,
            args=args_list,
            opt=_convert_invoke_options(self.invoke_options, self.context),
            return_nums=FAAS_FUNCTION_RETURN_NUM)

        return ObjectRef(obj_list[constants.INDEX_FIRST])


def _check_payload(payload: Union[str, dict]) -> str:
    """Checks whether the payload to call a function set by user is valid or not.

    Args:
        payload (Union[str, dict]): Payload set by user as input parameter 'event' in
                                    another function.

    Raises:
        TypeError: If payload is not of 'str' or 'dict' type.
        ValueError: If payload equals to string 'null'.
        TypeError: If payload is not JSON deserializable.

    Returns:
        str: The payload after dumping as a JSON string.
    """
    if not isinstance(payload, (str, dict)):
        msg = f"Invalid type({type(payload)}) of payload, 'str' or 'dict' is expected."
        log.get_logger().error(msg)
        raise TypeError(msg)

    if isinstance(payload, str):
        if payload == 'null':
            msg = f"Invalid value of payload: {payload}, it should not be equal to 'null'."
            log.get_logger().error(msg)
            raise ValueError(msg)
        try:
            json.loads(payload)
        except Exception as err:
            msg = f"Invalid payload: {payload}, it is not JSON deserializable."
            log.get_logger().error(msg)
            raise TypeError(msg) from err
    else:
        payload = json.dumps(payload)

    if len(payload) > _RUNTIME_MAX_RESP_BODY_SIZE:
        msg = f"Event size[{len(payload)}] after serialization should not be larger than {_RUNTIME_MAX_RESP_BODY_SIZE}."
        log.get_logger().error(msg)
        raise ValueError(msg)
    return payload


def _check_function_name(function_name: str) -> Tuple[str, str]:
    """Checks whether the funciton name set by user is valid or not. Parses the
    fucntion name to name and version.

    Args:
        function_name (str): Funciton name set by user when initialize the Function
                            object.

    Raises:
        TypeError: If function_name is not a string.

    Returns:
        Union[str, str]: The function name after parsing the user input 'function_name'
                        in Function object.
    """
    if not isinstance(function_name, str):
        msg = f"Invalid type({type(function_name)}) of parameter 'function_name', 'str' is expected."
        log.get_logger().error(msg)
        raise TypeError(msg)

    names = function_name.split(':', SPLIT_NUM_OF_FUNC_ID)
    if len(names) > SPLIT_NUM_OF_FUNC_ID:
        function, version = names
        _check_reg_length(function, FUNC_NAME_REG, FUNC_NAME_LENGTH_LIMIT)

        if version.startswith(ALIAS_PREFIX):
            alias = version.strip(ALIAS_PREFIX)
            _check_reg_length(alias, ALIAS_NAME_REG, ALIAS_NAME_LENGTH_LIMIT)
            return function, version

        _check_reg_length(version, VERSION_NAME_REG, VERSION_NAME_LENGTH_LIMIT)
        return function, version

    _check_reg_length(function_name, FUNC_NAME_REG, FUNC_NAME_LENGTH_LIMIT)
    return function_name, DEFAULT_FUNCTION_VERSION


def _get_service_name_from_env():
    """Returns service name read from the environment key-value pairs
    using key 'RUNTIME_SERVICE_FUNC_VERSION'.

    Raises:
        RuntimeError: When failure of getting service name of the
                    function from environment key-value pairs happends.
        ValueError: The Service name does not contain seperator '@'.

    Returns:
        str: The service name of functions.
    """
    current_func_id = os.environ.get(ENV_KEY_RUNTIME_SERVICE_FUNC_VERSION)
    if current_func_id is None:
        msg = ("Failed to get service name of the function from environment key-value pairs. "
               f"key: {ENV_KEY_RUNTIME_SERVICE_FUNC_VERSION}")
        log.get_logger().error(msg)
        raise RuntimeError(msg)
    names = current_func_id.split(FUNCTION_NAME_SEPERATOR)

    if len(names) <= SPLIT_NUM_OF_FUNC_ID:
        msg = (f"Invalid Environment value({current_func_id}) of key "
               f"'{ENV_KEY_RUNTIME_SERVICE_FUNC_VERSION}', "
               f"it should contain seperator '{FUNCTION_NAME_SEPERATOR}'")
        log.get_logger().error(msg)
        raise ValueError(msg)

    log.get_logger().debug(f"Succeeded to get service name '{names[SPLIT_NUM_OF_FUNC_ID]}'.")
    return names[SPLIT_NUM_OF_FUNC_ID]


def _check_reg_length(name: str, pattern: str, length_limit: int):
    """Checks the length of a string and whether it conforms to a specific regular expression.

    Args:
        name (str): The string to be checked.
        pattern (str): The regular expression to be conformed.
        length_limit (int): The maximun length of the string.

    Raises:
        ValueError: When the length of the string exceeds the limitation or
            the string does not conform a specific regular expression.
    """
    name_length = len(name)
    if name_length > length_limit or re.match(pattern, name) is None:
        if name_length > length_limit:
            msg = f"Length of '{name}'({name_length}) is larger than the limitation {length_limit}"
        else:
            msg = f"'{name}' does not match regular expression {pattern}"
        log.get_logger().error(msg)
        raise ValueError(msg)


def _convert_invoke_options(options: InvokeOptions, context: Context) -> YRInvokeOptions:
    """convert invoke options to yr options"""
    if options.concurrency == 0:
        options.concurrency = 100
    yr_options = YRInvokeOptions()
    if context is not None:
        yr_options.trace_id = context.get_trace_id()
    yr_options.concurrency = options.concurrency
    yr_options.cpu = options.cpu
    yr_options.memory = options.memory
    yr_options.custom_resources = options.custom_resources
    yr_options.pod_labels = options.pod_labels
    yr_options.labels = options.labels
    yr_options.alias_params = options.alias_params
    return yr_options
