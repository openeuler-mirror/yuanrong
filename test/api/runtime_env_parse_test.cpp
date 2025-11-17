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

#include "api/cpp/src/runtime_env_parse.h"
#include <gtest/gtest.h>
#include <filesystem>
#include <fstream>
#include "src/dto/invoke_options.h"

namespace fs = std::filesystem;
namespace YR {
namespace test {
class RuntimeEnvParseTest : public ::testing::Test {
protected:
    void SetUp() override
    {
        // 创建临时测试用的YAML文件
        validYamlPath_ = fs::temp_directory_path() / "valid_env.yaml";
        std::ofstream yamlFile(validYamlPath_);
        yamlFile << "name: test_env\ndependencies:\n  - python=3.8\n  - pip\n  - pip:\n    - numpy\n";
        yamlFile.close();

        // 创建临时测试用的YAML文件
        validNoNameYamlPath_ = fs::temp_directory_path() / "no_name_valid_env.yaml";
        std::ofstream noNameYamlFile(validNoNameYamlPath_);
        noNameYamlFile << "dependencies:\n  - python=3.8\n  - pip\n  - pip:\n    - numpy\n";
        noNameYamlFile.close();

        // 设置必要的环境变量
        setenv("YR_CONDA_HOME", "/fake/conda/path", 1);
    }

    void TearDown() override
    {
        // 清理临时文件
        if (fs::exists(validYamlPath_)) {
            fs::remove(validYamlPath_);
        }

        // 清理临时文件
        if (fs::exists(validNoNameYamlPath_)) {
            fs::remove(validNoNameYamlPath_);
        }
    }

    fs::path validYamlPath_;
    fs::path validNoNameYamlPath_;
};

TEST_F(RuntimeEnvParseTest, ShouldThrowWhenCondaHomeNotSet)
{
    unsetenv("YR_CONDA_HOME");
    YR::Libruntime::InvokeOptions options;
    YR::RuntimeEnv env;
    env.Set<nlohmann::json>("conda", {{"name", "test"}});

    EXPECT_THROW(YR::ParseRuntimeEnv(options, env), YR::Exception);
}

TEST_F(RuntimeEnvParseTest, ShouldProcessPipPackagesCorrectly)
{
    YR::Libruntime::InvokeOptions options;
    YR::RuntimeEnv env;
    env.Set<std::vector<std::string>>("pip", {"numpy", "pandas"});

    YR::ParseRuntimeEnv(options, env);
    EXPECT_EQ(options.createOptions["POST_START_EXEC"], "pip3 install numpy pandas");
}

TEST_F(RuntimeEnvParseTest, ShouldRejectBothPipAndConda)
{
    YR::Libruntime::InvokeOptions options;
    YR::RuntimeEnv env;
    env.Set<std::vector<std::string>>("pip", {"numpy"});
    env.Set<std::string>("conda", "env_name");

    EXPECT_THROW(YR::ParseRuntimeEnv(options, env), YR::Exception);
}

TEST_F(RuntimeEnvParseTest, ShouldHandleWorkingDirCorrectly)
{
    YR::Libruntime::InvokeOptions options;
    YR::RuntimeEnv env;
    env.Set<std::string>("working_dir", "/tmp/test_dir");

    YR::ParseRuntimeEnv(options, env);
    EXPECT_EQ(options.workingDir, "/tmp/test_dir");
}

TEST_F(RuntimeEnvParseTest, ShouldMergeEnvVarsProperly)
{
    YR::Libruntime::InvokeOptions options;
    options.envVars["EXISTING"] = "original";

    YR::RuntimeEnv env;
    env.Set<std::map<std::string, std::string>>("env_vars", {{"NEW", "value"}, {"EXISTING", "new"}});

    YR::ParseRuntimeEnv(options, env);
    EXPECT_EQ(options.envVars["NEW"], "value");
    EXPECT_EQ(options.envVars["EXISTING"], "original");  // 应保留原有值
}

TEST_F(RuntimeEnvParseTest, ShouldProcessCondaYamlFile)
{
    YR::Libruntime::InvokeOptions options;
    YR::RuntimeEnv env;
    env.Set<std::string>("conda", validYamlPath_.string());

    YR::ParseRuntimeEnv(options, env);
    EXPECT_EQ(options.createOptions["CONDA_PREFIX"], "/fake/conda/path");
    EXPECT_EQ(options.createOptions["CONDA_COMMAND"], "conda env create -f env.yaml");
    EXPECT_EQ(options.createOptions["CONDA_CONFIG"], "{\"name\": \"test_env\", \"dependencies\": [\"python=3.8\", \"pip\", {\"pip\": [\"numpy\"]}]}");
}

TEST_F(RuntimeEnvParseTest, ShouldProcessCondaNoNameYamlFile)
{
    YR::Libruntime::InvokeOptions options;
    YR::RuntimeEnv env;
    env.Set<std::string>("conda", validNoNameYamlPath_.string());

    YR::ParseRuntimeEnv(options, env);
    EXPECT_EQ(options.createOptions["CONDA_PREFIX"], "/fake/conda/path");
    EXPECT_EQ(options.createOptions["CONDA_COMMAND"], "conda env create -f env.yaml");
    EXPECT_FALSE(options.createOptions["CONDA_DEFAULT_ENV"].empty());
}

TEST_F(RuntimeEnvParseTest, ShouldProcessCondaEnvName)
{
    YR::Libruntime::InvokeOptions options;
    YR::RuntimeEnv env;
    env.Set<std::string>("conda", "existing_env");

    YR::ParseRuntimeEnv(options, env);
    EXPECT_EQ(options.createOptions["CONDA_COMMAND"], "conda activate existing_env");
    EXPECT_EQ(options.createOptions["CONDA_DEFAULT_ENV"], "existing_env");
}

TEST_F(RuntimeEnvParseTest, ShouldProcessCondaJsonConfig)
{
    YR::Libruntime::InvokeOptions options;
    YR::RuntimeEnv env;
    nlohmann::json config = {{"name", "json_env"}, {"dependencies", {"python=3.8", "numpy"}}};
    env.Set<nlohmann::json>("conda", config);

    YR::ParseRuntimeEnv(options, env);
    EXPECT_EQ(options.createOptions["CONDA_COMMAND"], "conda env create -f env.yaml");
    EXPECT_TRUE(options.createOptions["CONDA_DEFAULT_ENV"].find("json_env") != std::string::npos);
}

TEST_F(RuntimeEnvParseTest, ShouldRejectInvalidYamlFile)
{
    YR::Libruntime::InvokeOptions options;
    YR::RuntimeEnv env;
    env.Set<std::string>("conda", "/nonexistent/path.yaml");

    EXPECT_THROW(YR::ParseRuntimeEnv(options, env), YR::Exception);
}

TEST_F(RuntimeEnvParseTest, ShouldRejectInvalidPipType)
{
    YR::Libruntime::InvokeOptions options;
    YR::RuntimeEnv env;
    env.Set<std::string>("pip", "this_should_be_array");

    EXPECT_THROW(YR::ParseRuntimeEnv(options, env), YR::Exception);
}

TEST_F(RuntimeEnvParseTest, ShouldGenerateRandomNameForEmptyCondaName)
{
    YR::Libruntime::InvokeOptions options;
    YR::RuntimeEnv env;
    nlohmann::json config = {{"name", ""}, {"dependencies", {"python"}}};
    env.Set<nlohmann::json>("conda", config);

    YR::ParseRuntimeEnv(options, env);
    EXPECT_FALSE(options.createOptions["CONDA_DEFAULT_ENV"].empty());
}

TEST_F(RuntimeEnvParseTest, ShouldAddSharedDirInCreateOpt)
{
    YR::Libruntime::InvokeOptions options;
    YR::RuntimeEnv env;
    nlohmann::json config = {{"name", "abc"}, {"TTL", 5}};
    env.Set<nlohmann::json>("shared_dir", config);

    YR::ParseRuntimeEnv(options, env);
    EXPECT_EQ(options.createOptions["DELEGATE_SHARED_DIRECTORY"], "abc");
    EXPECT_EQ(options.createOptions["DELEGATE_SHARED_DIRECTORY_TTL"], "5");

    YR::Libruntime::InvokeOptions options2;
    YR::RuntimeEnv env2;
    nlohmann::json config2 = {{"name", "abc"}};
    env2.Set<nlohmann::json>("shared_dir", config2);

    YR::ParseRuntimeEnv(options2, env2);
    EXPECT_EQ(options2.createOptions["DELEGATE_SHARED_DIRECTORY"], "abc");
    EXPECT_EQ(options2.createOptions["DELEGATE_SHARED_DIRECTORY_TTL"], "0");
}

TEST_F(RuntimeEnvParseTest, ShouldThrowExceptionWhenSharedDirConfigInvaild)
{
    YR::Libruntime::InvokeOptions options;
    YR::RuntimeEnv env;
    nlohmann::json config = {{"name", ""}};
    env.Set<nlohmann::json>("shared_dir", config);
    EXPECT_THROW(YR::ParseRuntimeEnv(options, env), YR::Exception);

    YR::Libruntime::InvokeOptions options2;
    YR::RuntimeEnv env2;
    env2.Set<nlohmann::json>("shared_dir", "str");
    EXPECT_THROW(YR::ParseRuntimeEnv(options2, env2), YR::Exception);
}
}
}