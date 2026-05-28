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
/**
 * @brief This class provides interfaces for setting runtime environment for actors.
 */
class RuntimeEnv {
public:
    /**
     * @brief Set a runtime environment field via JSON string.
     *
     * @param name The name of the runtime environment parameter.
     * @param jsonStr A JSON-formatted string representing the field value.
     */
    void SetJsonStr(const std::string &name, const std::string &jsonStr);

    /**
     * @brief Set the name and value of the runtime environment.
     *
     * Example:
     * @code
     * #include <iostream>
     * #include "yr/yr.h"
     * #include "yr/api/runtime_env.h"
     * int main(int argc, char **argv) {
     *     std::string pyFunctionUrn =
     *         "sn:cn:yrk:12345678901234561234567890123456:function:0-yr-mypython:$latest";
     *     YR::Config conf;
     *     YR::Init(conf, argc, argv);
     *     YR::InvokeOptions opts;
     *     YR::RuntimeEnv runtimeEnv;
     *     runtimeEnv.Set<std::string>("conda", "pytorch_p39");
     *     runtimeEnv.Set<std::map<std::string, std::string>>("env_vars",
     *         {{"OMP_NUM_THREADS", "32"}, {"TF_WARNINGS", "none"},
     *          {"YR_CONDA_HOME", "/home/snuser/.conda"}});
     *     opts.runtimeEnv = runtimeEnv;
     *     auto resFutureSquare = YR::PyFunction<int>("calculator", "square")
     *         .SetUrn(pyFunctionUrn).Options(opts).Invoke(2);
     *     auto resSquare = *YR::Get(resFutureSquare);
     *     std::cout << resSquare << std::endl;
     *     return 0;
     * }
     * @endcode
     *
     * @note
     * RuntimeEnv only processes the following named keys.
     * Other keys will neither take effect nor report errors:
     * - **pip** ``String[]/Iterable<String>``: Python dependencies for the
     *   environment (mutually exclusive with conda key).
     *   @code
     *   YR::RuntimeEnv env;
     *   env.Set<std::vector<std::string>>("pip", {"numpy=2.3.0", "pandas"});
     *   @endcode
     * - **working_dir** ``str``: Specified code path, currently only supports
     *   C++ actors.
     *   @code
     *   YR::RuntimeEnv env;
     *   env.Set<std::string>("working_dir",
     *       "/opt/mycode/cpp-invoke-python/calculator");
     *   @endcode
     * - **env_vars** ``JSON``: Environment variables (values must be string
     *   type).
     *   @code
     *   YR::RuntimeEnv runtimeEnv;
     *   runtimeEnv.Set<std::map<std::string, std::string>>("env_vars",
     *       {{"OMP_NUM_THREADS", "32"}, {"TF_WARNINGS", "none"}});
     *   @endcode
     * - **conda** ``str/JSON``: Conda configuration (requires YR_CONDA_HOME
     *   environment variable).
     *
     *   (1). Specify scheduling to an existing conda environment.
     *   @code
     *   YR::RuntimeEnv env;
     *   runtimeEnv.Set<std::string>("conda", "pytorch_p39");
     *   @endcode
     *
     *   (2). Create a new environment and specify environment dependencies
     *   and environment name (optional).
     *   @code
     *   runtimeEnv.Set<nlohmann::json>("conda",
     *       {{"name", "pytorch_p39"},
     *        {"channels", {"conda-forge"}},
     *        {"dependencies", {"python=3.9", "matplotlib",
     *         "msgpack-python=1.0.5", "protobuf", "libgcc-ng", "numpy",
     *         "pandas", "cloudpickle=2.0.0", "cython=3.0.10",
     *         "pyyaml=6.0.2"}}});
     *   runtimeEnv.Set<std::map<std::string, std::string>>("env_vars",
     *       {{"OMP_NUM_THREADS", "32"}, {"TF_WARNINGS", "none"},
     *        {"YR_CONDA_HOME", "/home/snuser/.conda"}});
     *   @endcode
     *
     *   (3). Create a new environment, and specify environment dependencies
     *   and environment name via file.
     *   @code
     *   YR::RuntimeEnv runtimeEnv;
     *   // yaml file demo
     *   // name: myenv3
     *   // channels:
     *   //  - conda-forge
     *   //  - defaults
     *   // dependencies:
     *   //  - python=3.9
     *   //  - numpy
     *   //  - pandas
     *   //  - cloudpickle=2.2.1
     *   //  - msgpack-python=1.0.5
     *   //  - protobuf
     *   //  - cython=3.0.10
     *   //  - pyyaml=6.0.2
     *   runtimeEnv.Set<std::string>("conda", "/opt/conda/env-xpf.yaml");
     *   @endcode
     * - **venv** ``JSON``: Python built-in virtual environment venv
     *   configuration (mutually exclusive with pip key and conda key).
     *
     *   (1). Download dependencies via pip install.
     *   @code
     *   YR::RuntimeEnv runtimeEnv;
     *   runtimeEnv.Set<nlohmann::json>("venv", {
     *     // Specify virtual environment name, leave empty or "" to
     *     // auto-generate uuid as virtual environment name
     *     {"name", "testVenv"},
     *     // Configure parameters for pip download to virtual environment,
     *     // optional
     *     {"dependencies", {
     *       // Package names to pip install, optional
     *       // Supports pure package name (requests), exact version
     *       // (requests==2.28.1), version constraint (requests>=2.0,<3.0),
     *       // does not support url
     *       {"pypi", {"pandas", "pyarrow", "requests", "cloudpickle==3.0.0"}},
     *       // Optional: mark specified host (or "host:port") as trusted, even
     *       // without valid HTTPS certificate or using HTTP (non-encrypted)
     *       // protocol, does not support multiple addresses
     *       {"trust_host", "mirrors.tools.nobody.com"},
     *       // Optional: specify PyPI mirror source URL for pip installation
     *       // instead of default official source, does not support multiple
     *       // addresses
     *       {"index_url", "http://mirrors.tools.nobody.com/pypi/simple/"}
     *     }}
     *   });
     *   @endcode
     *
     *   (2). Reuse site-packages by specifying obs address, skip pip
     *   installation.
     *   @code
     *   YR::InvokeOptions opts;
     *   YR::RuntimeEnv runtimeEnv;
     *
     *   runtimeEnv.Set<nlohmann::json>("venv", {
     *     // Specify virtual environment name, leave empty or "" to
     *     // auto-generate uuid as virtual environment name
     *     {"name", "testVenv"},
     *     {"path", {
     *       // Specify site-packages address to download, starting with obs://
     *       // bucketId and objectId must be consistent with
     *       // opts.customExtensions["DELEGATE_DOWNLOAD"], otherwise download
     *       // will fail
     *       {"site_package_path", "obs://bucketId/objectId"}
     *     }},
     *   });
     *
     *   opts.runtimeEnv = runtimeEnv;
     *   // Required when configuring site_package_path, otherwise download
     *   // will fail
     *   // DELEGATE_DOWNLOAD configuration refers to InvokeOptions struct
     *   // section
     *   opts.customExtensions["DELEGATE_DOWNLOAD"] =
     *       "{\"storage_type\":\"s3\",\"hostName\":\"obs.xxxx.com\","
     *       "\"bucketId\":\"bucketId\",\"objectId\":\"objectid\","
     *       "\"sha256\":\"xxxxx\","
     *       "\"temporaryAccessKey\":\"HST3UXZO1UWEXG6ZGUPV\","
     *       "\"temporarySecretKey\":\"xxxxxx\","
     *       "\"securityToken\":\"xxxxxxxx\"}";
     *   @endcode
     *
     *   (3). Reuse site-packages by specifying obs address, incremental
     *   installation of PyPI packages.
     *   @code
     *   YR::InvokeOptions opts;
     *   YR::RuntimeEnv runtimeEnv;
     *
     *   runtimeEnv.Set<nlohmann::json>("venv", {
     *     {"name", "testVenv"},
     *     {"dependencies", {
     *       // Add pandas download based on existing site-packages, user must
     *       // ensure no version conflicts between packages
     *       {"pypi", {"pandas"}},
     *       {"trust_host", "mirrors.tools.nobody.com"},
     *       {"index_url", "http://mirrors.tools.nobody.com/pypi/simple/"}
     *     }},
     *     {"path", {
     *       {"site_package_path", "obs://bucketId/site-package.zip"}
     *     }},
     *   });
     *
     *   opts.runtimeEnv = runtimeEnv;
     *   opts.customExtensions["DELEGATE_DOWNLOAD"] =
     *       "{\"storage_type\":\"s3\",\"hostName\":\"obs.xxxx.com\","
     *       "\"bucketId\":\"bucketId\","
     *       "\"objectId\":\"site-package.zip\",\"sha256\":\"xxxxx\","
     *       "\"temporaryAccessKey\":\"HST3UXZO1UWEXG6ZGUPV\","
     *       "\"temporarySecretKey\":\"xxxxxx\","
     *       "\"securityToken\":\"xxxxxxxx\"}";
     *   @endcode
     *
     * @tparam T The type of the second parameter.
     * @param name The name of the runtime environment parameter.
     * @param value A json-serializable object of nlohmann/json type.
     */
    template <typename T>
    void Set(const std::string &name, const T &value);

    /**
     * @brief Check if the runtime environment is empty.
     *
     * @return true if no fields have been set, false otherwise.
     */
    bool Empty() const
    {
        return jsons_.empty();
    }

    /**
     * @brief Get a runtime environment field value.
     *
     * @tparam T The type of the return value.
     * @param name The name of the runtime environment parameter.
     * @return The parameter value of type T.
     * @throws YR::Exception::InvalidParamException if the field is not found or type conversion fails.
     */
    template <typename T>
    T Get(const std::string &name) const;

    /**
     * @brief Get the JSON string representation of a specified field.
     *
     * @param name The name of the runtime environment parameter.
     * @return JSON string representation of the field.
     */
    std::string GetJsonStr(const std::string &name) const;

    /**
     * @brief Check if a specified field exists.
     *
     * @param name The field name to check.
     * @return true if the field exists, false otherwise.
     */
    bool Contains(const std::string &name) const;

    /**
     * @brief Remove a specified field.
     *
     * @param name The field name to remove.
     * @return true if successfully removed, false if the field does not exist.
     */
    bool Remove(const std::string &name);

private:
    nlohmann::json jsons_;
};

template <typename T>
inline T RuntimeEnv::Get(const std::string &name) const
{
    if (!Contains(name)) {
        throw YR::Exception::InvalidParamException("The field " + name + " not found.");
    }
    try {
        return jsons_[name].get<T>();
    } catch (std::exception &e) {
        throw YR::Exception::InvalidParamException("Failed to get the field " + name + ": " + e.what());
    }
}

template <typename T>
inline void RuntimeEnv::Set(const std::string &name, const T &value)
{
    try {
        nlohmann::json valueJ = value;
        jsons_[name] = valueJ;
    } catch (std::exception &e) {
        throw YR::Exception::InvalidParamException("Failed to set the field " + name + ": " + e.what());
    }
}
}  // namespace YR