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
#include "Context.h"

namespace Function {
/*
 * Customer implement this interface to handle runtime message
 */
class RuntimeHandler {
public:
    RuntimeHandler() = default;
    virtual ~RuntimeHandler() = default;

    virtual std::string HandleRequest(const std::string &request, Context &context) = 0;

    virtual void InitState(const std::string &request, Context &context) = 0;

    virtual void PreStop(Context &context) = 0;

    virtual void Initializer(Context &context) = 0;
};
}  // namespace Function

