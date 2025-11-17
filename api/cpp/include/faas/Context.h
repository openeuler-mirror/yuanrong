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

#include <string>
#include "FunctionLogger.h"

namespace Function {
class Context {
public:
    Context() = default;

    virtual ~Context() = default;

    virtual const std::string GetAccessKey() const = 0;

    virtual const std::string GetSecretKey() const = 0;

    virtual const std::string GetSecurityAccessKey() const = 0;

    virtual const std::string GetSecuritySecretKey() const = 0;

    virtual const std::string GetToken() const = 0;

    virtual const std::string GetAlias() const = 0;

    virtual const std::string GetTraceId() const = 0;

    virtual const std::string GetInvokeId() const = 0;

    virtual const FunctionLogger &GetLogger() = 0;

    virtual const std::string GetState() const = 0;

    virtual const std::string GetInstanceId() const = 0;

    virtual const std::string GetInstanceLabel() const = 0;

    virtual void SetState(const std::string &state) = 0;

    virtual const std::string GetInvokeProperty() const = 0;

    virtual const std::string GetRequestID() const = 0;

    virtual const std::string GetUserData(std::string key) const = 0;

    virtual const std::string GetFunctionName() const = 0;

    virtual int GetRemainingTimeInMilliSeconds() const = 0;

    virtual int GetRunningTimeInSeconds() const = 0;

    virtual const std::string GetVersion() const = 0;

    virtual int GetMemorySize() const = 0;

    virtual int GetCPUNumber() const = 0;

    virtual const std::string GetProjectID() const = 0;

    virtual const std::string GetPackage() const = 0;
};

}  // namespace Function
