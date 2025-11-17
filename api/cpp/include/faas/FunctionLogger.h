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

namespace Function {
class FunctionLogger {
public:
    FunctionLogger(){};
    virtual ~FunctionLogger(){};
    FunctionLogger(const std::string &traceId, const std::string &invokeId) : traceId(traceId), invokeId(invokeId)
    {
    }

    void setLevel(const std::string &level);

    void Info(std::string message, ...);

    void Warn(std::string message, ...);

    void Debug(std::string message, ...);

    void Error(std::string message, ...);

    void SetInvokeID(std::string invokeID);

    void SetTraceID(std::string traceID);

private:
    std::string traceId;
    std::string invokeId;
    std::string logLevel = "INFO";
    void Log(const std::string &level, const std::string &logMessage);
    bool sendEmptyLog(const std::string &message, const std::string &level);
};
}  // namespace Function
