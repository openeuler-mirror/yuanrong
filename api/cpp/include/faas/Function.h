/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#pragma once

#include <map>
#include <memory>
#include <string>
#include <unordered_map>
#include <vector>
#include "Constant.h"
#include "Context.h"
#include "ObjectRef.h"

namespace Function {

struct InvokeOptions {
    // unit is ms
    int cpu = 0;
    // unit is MB
    int memory = 0;

    std::unordered_map<std::string, std::string> aliasParams;
};

class Function {
public:
    explicit Function(Context &context);
    explicit Function(Context &context, const std::string &funcName);
    explicit Function(Context &context, const std::string &funcName, const std::string &instanceName);

    virtual ~Function() = default;

    Function(const Function &) = delete;

    Function &operator=(const Function &) = delete;

    ObjectRef Invoke(const std::string &payload);

    Function &Options(const InvokeOptions &opt);

    const std::string GetObjectRef(ObjectRef &objectRef);

    void GetInstance(const std::string &functionName, const std::string &instanceName);

    void GetLocalInstance(const std::string &functionName, const std::string &instanceName);

    ObjectRef Terminate();

    void SaveState();

    const std::shared_ptr<Context> GetContext() const;

    std::string GetInstanceId() const;

private:
    std::shared_ptr<Context> context_;
    std::string funcName_;
    std::string instanceName_;
    std::string instanceID_;
    InvokeOptions options_;
};
}  // namespace Function
