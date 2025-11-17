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

#if __has_include(<filesystem>)
#include <filesystem>
#else
#include <experimental/filesystem>
#endif
#include <iostream>

#include <yaml-cpp/yaml.h>
#include <json.hpp>

#include "yr/api/exception.h"
#include "src/libruntime/err_type.h"
#include "api/cpp/src/utils/utils.h"
#include "yr/api/runtime_env.h"
#include "src/dto/invoke_options.h"
#include "api/cpp/src/runtime_env_parse.h"

#ifdef __cpp_lib_filesystem
namespace filesystem = std::filesystem;
#elif __cpp_lib_experimental_filesystem
namespace filesystem = std::experimental::filesystem;
#endif

namespace YR {
const std::string CONDA = "conda";
const std::string PIP = "pip";
const std::string YR_CONDA_HOME = "YR_CONDA_HOME";
const std::string WORKER_DIR = "working_dir";
const std::string ENV_VARS = "env_vars";
const std::string SHARED_DIR = "shared_dir";

const std::string POST_START_EXEC = "POST_START_EXEC";
const std::string CONDA_PREFIX = "CONDA_PREFIX";
const std::string CONDA_CONFIG = "CONDA_CONFIG";
const std::string CONDA_COMMAND = "CONDA_COMMAND";
const std::string CONDA_DEFAULT_ENV = "CONDA_DEFAULT_ENV";

std::string GetCondaBinExecutable()
{
    if (auto envStr = GetEnv(YR_CONDA_HOME); !envStr.empty()) {
        return envStr;
    }
    if (auto envStr = GetEnv(CONDA_PREFIX); !envStr.empty()) {
        return envStr;
    }
    throw YR::Exception(YR::Libruntime::ErrorCode::ERR_PARAM_INVALID,
                        "please configure YR_CONDA_HOME environment variable which contain a bin subdirectory");
}

std::string YamlToJson(YAML::Node &node)
{
    YAML::Emitter emitter;
    emitter << YAML::DoubleQuoted << YAML::Flow << YAML::BeginSeq << node;

    return std::string(emitter.c_str() + 1);
}
void HandleCondaConfig(YR::Libruntime::InvokeOptions& invokeOptions, const nlohmann::json& condaConfig);

void HandleSharedDirConfig(YR::Libruntime::InvokeOptions &invokeOptions, const nlohmann::json &sharedDirConfig)
{
    if (sharedDirConfig.is_object()) {
        // 处理JSON对象类型的conda配置
        std::string name = sharedDirConfig.value("name", "");
        if (name.empty()) {
            throw YR::Exception(YR::Libruntime::ErrorCode::ERR_PARAM_INVALID, "shared dir name must be string");
        }
        int ttl = sharedDirConfig.value("TTL", 0);
        invokeOptions.createOptions["DELEGATE_SHARED_DIRECTORY"] = name;
        invokeOptions.createOptions["DELEGATE_SHARED_DIRECTORY_TTL"] = std::to_string(ttl);
    } else {
        throw YR::Exception(YR::Libruntime::ErrorCode::ERR_PARAM_INVALID, "shared dir format must be json");
    }
}

void ParseRuntimeEnv(YR::Libruntime::InvokeOptions& invokeOptions, const YR::RuntimeEnv& runtimeEnv)
{
    if (runtimeEnv.Empty()) {
        return;
    }

    if (runtimeEnv.Contains(CONDA) && runtimeEnv.Contains(PIP)) {
        throw YR::Exception(YR::Libruntime::ErrorCode::ERR_PARAM_INVALID,
                            "The 'pip' field and 'conda' field of runtime_env cannot both be specified.\n"
                            "To use pip with conda, please only set the 'conda' "
                            "field, and specify your pip dependencies "
                            "within the conda YAML config dict");
    }
    if (runtimeEnv.Contains(PIP)) {
        try {
            const auto &pipPackages = runtimeEnv.Get<std::vector<std::string>>(PIP);
            std::ostringstream  pipCommand;
            pipCommand << "pip3 install";
            for (size_t i = 0; i < pipPackages.size(); i++) {
                pipCommand << " " << pipPackages[i];
            }
            invokeOptions.createOptions[POST_START_EXEC] = pipCommand.str();
        }  catch (std::exception &e) {
            throw YR::Exception(YR::Libruntime::ErrorCode::ERR_PARAM_INVALID,
                                std::string("Failed to parse pip field of RuntimeEnv: ") + e.what());
        }
    }
    if (runtimeEnv.Contains(WORKER_DIR)) {
        try {
            const std::string &workingDir = runtimeEnv.Get<std::string>(WORKER_DIR);
            invokeOptions.workingDir = workingDir;
        }  catch (std::exception &e) {
            throw YR::Exception(YR::Libruntime::ErrorCode::ERR_PARAM_INVALID,
                                std::string("`working_dir` must be a string: ") + e.what());
        }
    }
    if (runtimeEnv.Contains(ENV_VARS)) {
        try {
            const auto& envVars = runtimeEnv.Get<std::map<std::string, std::string>>(ENV_VARS);
            for (const auto& pair : envVars) {
                if (invokeOptions.envVars.find(pair.first) == invokeOptions.envVars.end()) {
                    invokeOptions.envVars.insert(pair);
                }
            }
        }  catch (std::exception &e) {
            throw YR::Exception(YR::Libruntime::ErrorCode::ERR_PARAM_INVALID,
                                std::string("`envs` must be a map[string: string]: ") + e.what());
        }
    }
    if (runtimeEnv.Contains(CONDA)) {
        nlohmann::json condaJson;
        try {
        condaJson = runtimeEnv.Get<nlohmann::json>(CONDA);
        } catch(std::exception &e) {
            throw YR::Exception(YR::Libruntime::ErrorCode::ERR_PARAM_INVALID,
                                std::string("`get conda to nlohmann:json format failed: ") + e.what());
        }
        HandleCondaConfig(invokeOptions, condaJson);
    }
    if (runtimeEnv.Contains(SHARED_DIR)) {
        nlohmann::json sharedDirJson;
        try {
            sharedDirJson = runtimeEnv.Get<nlohmann::json>(SHARED_DIR);
            HandleSharedDirConfig(invokeOptions, sharedDirJson);
        } catch (std::exception &e) {
            throw YR::Exception(YR::Libruntime::ErrorCode::ERR_PARAM_INVALID,
                                std::string("`get shared dir to nlohmann:json format failed: ") + e.what());
        }
    }
}

void HandleCondaConfig(YR::Libruntime::InvokeOptions& invokeOptions, const nlohmann::json& condaConfig)
{
    invokeOptions.createOptions[CONDA_PREFIX] = GetCondaBinExecutable();

    if (condaConfig.is_string()) {
        // 处理字符串类型的conda配置（YAML文件路径或环境名称）
        const std::string& condaStr = condaConfig.get<std::string>();
        const filesystem::path condaPath(condaStr);

        if (condaPath.extension() == ".yaml" || condaPath.extension() == ".yml") {
            if (!filesystem::exists(condaPath)) {
                throw YR::Exception(YR::Libruntime::ErrorCode::ERR_PARAM_INVALID,
                                    std::string("Can't find conda YAML file ") + condaStr);
            }

            YAML::Node yamlNode = YAML::LoadFile(condaStr);
            std::string envName;
            try {
                envName = yamlNode["name"].as<std::string>();
            } catch (std::exception& e) {
                // 支持传空，以及类型异常处理，yaml-cpp高低版本之间判断是否是string类型的函数不一致，使用try catch保持一致
            }
            if (envName.empty()) {
                envName = "virtual_env-" + YR::utility::IDGenerator::GenRequestId();
            }

            invokeOptions.createOptions[CONDA_CONFIG] = YamlToJson(yamlNode);
            invokeOptions.createOptions[CONDA_COMMAND] = "conda env create -f env.yaml";
            invokeOptions.createOptions[CONDA_DEFAULT_ENV] = envName;
        } else {
            // 直接使用环境名称
            invokeOptions.createOptions[CONDA_COMMAND] = "conda activate " + condaStr;
            invokeOptions.createOptions[CONDA_DEFAULT_ENV] = condaStr;
        }
    } else if (condaConfig.is_object()) {
        // 处理JSON对象类型的conda配置
        std::string envName = condaConfig.value("name", "");
        if (envName.empty()) {
            envName = "virtual_env-" + YR::utility::IDGenerator::GenRequestId();
        }

        invokeOptions.createOptions[CONDA_CONFIG] = condaConfig.dump();
        invokeOptions.createOptions[CONDA_COMMAND] = "conda env create -f env.yaml";
        invokeOptions.createOptions[CONDA_DEFAULT_ENV] = envName;
    } else {
        throw YR::Exception(YR::Libruntime::ErrorCode::ERR_PARAM_INVALID,
                            "conda format must be string or json");
    }
}
}
