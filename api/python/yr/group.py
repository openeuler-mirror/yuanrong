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

from dataclasses import dataclass
from yr.config import GroupOptions
from yr import log, runtime_holder


@dataclass
class Group:
    group_name: str = ""
    group_opts: GroupOptions = None

    def invoke(self):
        runtime_holder.global_runtime.get_runtime().create_group(
            self.group_name, self.group_opts)

    def terminate(self):
        runtime_holder.global_runtime.get_runtime().terminate_group(self.group_name)

    def wait(self):
        runtime_holder.global_runtime.get_runtime().wait_group(self.group_name)

    def suspend(self):
        runtime_holder.global_runtime.get_runtime().suspend_group(self.group_name)

    def resume(self):
        runtime_holder.global_runtime.get_runtime().resume_group(self.group_name)
