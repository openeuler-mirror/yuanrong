/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

#include <functional>
#include <memory>
#include <string>
#include "Constant.h"
#include "Context.h"
#include "RuntimeHandler.h"

namespace Function {
class Runtime {
public:
    Runtime();
    virtual ~Runtime();

    void RegisterHandler(
        std::function<std::string(const std::string &request, Function::Context &context)> handleRequestFunc);

    void RegisterInitializerFunction(std::function<void(Function::Context &context)> initializerFunc);

    void RegisterPreStopFunction(std::function<void(Function::Context &context)> preStopFunc);

    void InitState(std::function<void(const std::string &request, Function::Context &context)> initStateFunc);

    void Start(int argc, char *argv[]);

private:
    std::function<std::string(const std::string &, Function::Context &)> handleRequest;
    std::function<void(Function::Context &)> initializerFunction;
    std::function<void(Function::Context &)> preStopFunction;
    std::function<void(const std::string &, Function::Context &)> initStateFunction;
    void InitRuntimeLogger() const;
    void ReleaseRuntimeLogger() const;
    void BuildRegisterRuntimeHandler() const;
};
}  // namespace Function
