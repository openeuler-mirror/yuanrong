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

#include <exception>
#include <string>
#include "Constant.h"

namespace Function {
class FunctionError : public std::exception {
public:
    FunctionError(int code, const std::string message) : errCode((ErrorCode)code), errMsg(message) {}
    virtual ~FunctionError() = default;
    const char *what() const noexcept override;
    ErrorCode GetErrorCode() const;
    const std::string GetMessage() const;
    const std::string GetJsonString() const;

private:
    ErrorCode errCode;
    std::string errMsg;
};
}  // namespace Function
