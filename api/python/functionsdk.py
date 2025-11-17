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

import yr


class Function():
    """Class Function"""
    def __init__(self, context, function_name=None, instance_name=None) -> None:
        self.__context = context
        self.__function_name = function_name
        self.__instance_name = instance_name
        self.__function = yr.Function(function_name)

    def invoke(self, payload):
        """Invoke stateless or stateful function."""
        return self.__function.invoke(payload)
