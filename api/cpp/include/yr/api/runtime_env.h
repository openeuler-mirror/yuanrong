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

#include <list>

#include "json.hpp"

#include "yr/api/constant.h"
#include "yr/api/exception.h"

namespace YR {

/*!
 * @class RuntimeEnv
 * @brief This class provides interfaces of setting runtime environments for python actor.
 */
class RuntimeEnv {
public:
    /*!
     * @brief Set a runtime env field by name and Object.
     *
     * @tparam T The second param type of the value.
     * @param name The runtime env plugin name.
     * @param value An object with jsonable type of nlohmann/json.
     *
     * @note Constraints
     * - RuntimeEnv only processes keys named conda, pip, working_dir, and env_vars; other keys neither take effect
     * nor trigger errors.
     * - [1] **pip** `String[]/Iterable<String>` Specifies Python dependencies (mutually exclusive with conda),
     * for example:
     * @snippet{trimleft} runtime_env_example1.cpp set pip
     * - [2] **working_dir** `str` Specify the code path, but this feature is not enabled yet. for example:
     * @snippet{trimleft} runtime_env_example1.cpp set working_dir
     * - [3] **env_vars** `JSON` Environment variables (values must be strings), for example:
     * @snippet{trimleft} runtime_env_example1.cpp set env_vars
     * - [4] **conda** `str/JSON` Conda configuration (requires YR_CONDA_HOME), for example:<br>
     *   (1). Specify scheduling to an existing conda environment.
     *   @snippet{trimleft} runtime_env_example1.cpp set existed conda environ
     *   (2). Create a new environment and specify its dependencies and environment name (optional).
     *   @snippet{trimleft} runtime_env_example1.cpp set conda environ with dependency
     *   (3). Create a new environment, specify dependencies and environment names through files.
     *   @snippet{trimleft} runtime_env_example1.cpp set conda environ with yaml file
     *
     * @snippet{trimleft} runtime_env_example.cpp runtime env demo
     */
    template <typename T>
    void Set(const std::string &name, const T &value);

    /**
     * @brief  Get the object of a runtime env field.
     * @tparam T The return type of the function.
     * @param name The runtime env plugin name.
     * @return A runtime env field with T type.
     */
    template <typename T>
    T Get(const std::string &name) const;

    /**
     * @brief  Set a runtime env field by name and json string.
     * @param name The runtime env plugin name.
     * @param jsonStr A json string represents the runtime env field.
     */
    void SetJsonStr(const std::string &name, const std::string &jsonStr);

    /**
     * @brief  Get the json string of a runtime env field.
     * @param name The runtime env plugin name.
     * @return A string type object with runtime env field.
     */
    std::string GetJsonStr(const std::string &name) const;

    /**
     * @brief  Whether a field is contained.
     * @param name The runtime env plugin name.
     * @return Whether the filed is contained.
     */
    bool Contains(const std::string &name) const;

    /**
     * @brief  Remove a field by name.
     * @param name The runtime env plugin name.
     * @return true if remove an existing field, otherwise false.
     */
    bool Remove(const std::string &name);

    /**
     * @brief  Whether the runtime env is empty.
     * @return  Whether the runtime env is empty.
     */
    bool Empty() const
    {
        return fields_.empty();
    }

private:
    nlohmann::json fields_;
};

template <typename T>
inline void RuntimeEnv::Set(const std::string &name, const T &value)
{
    try {
        nlohmann::json valueJ = value;
        fields_[name] = valueJ;
    } catch (std::exception &e) {
        throw Exception::InvalidParamException("Failed to set the field " + name + ": " + e.what());
    }
}

template <typename T>
inline T RuntimeEnv::Get(const std::string &name) const
{
    if (!Contains(name)) {
        throw Exception::InvalidParamException("The field " + name + " not found.");
    }
    try {
        return fields_[name].get<T>();
    } catch (std::exception &e) {
        throw Exception::InvalidParamException("Failed to get the field " + name + ": " + e.what());
    }
}
} // namespace YR