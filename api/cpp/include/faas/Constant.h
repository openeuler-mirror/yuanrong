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
enum ErrorCode {
    OK = 0,
    ERROR = 1,

    ILLEGAL_ACCESS = 4001,
    FUNCTION_EXCEPTION = 4002,
    USER_STATE_LARGE_ERROR = 4003,
    ILLEGAL_RETURN = 4004,
    USER_STATE_UNDEFINED_ERROR = 4005,
    USER_INITIALIZATION_FUNCTION_EXCEPTION = 4009,
    USER_LOAD_FUNCTION_EXCEPTION = 4014,
    NO_SUCH_INSTANCE_NAME_ERROR_CODE = 4026,
    INVALID_PARAMETER = 4040,
    NO_SUCH_STATE_ERROR_CODE = 4041,
    INTERNAL_ERROR = 110500,
};
// UserErrorMax is the maximum value of user errors
const int USER_ERROR_MAX = 10000;

const std::string SUCCESS_RESPONSE = "OK";
const std::string ILLEGAL_ACCESS_MESSAGE = "function entry cannot be found";
const std::string FUNCTION_EXCEPTION_MESSAGE = "function invocation exception: ";
const std::string USER_STATE_LARGE_ERROR_MESSAGE = "state content is too large";
const std::string ILLEGAL_RETURN_MESSAGE = "function return value is too large";
const std::string USER_STATE_UNDEFINED_ERROR_MESSAGE = "state is undefined";

const std::string INTERNAL_ERROR_MESSAGE = "internal system error";
const uint32_t MAX_USER_EXCEPTION_LENGTH = 1024;
const uint32_t MAX_USER_STATE_LENGTH = 3584 * 1024;
}  // namespace Function