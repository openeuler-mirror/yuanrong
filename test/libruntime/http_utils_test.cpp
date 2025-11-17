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

#include "src/libruntime/utils/http_utils.h"
#include <gmock/gmock.h>
#include <gtest/gtest.h>

namespace YR {
namespace test {
using namespace YR::utility;
using namespace YR::Libruntime;
class HttpUtilTest : public ::testing::Test {
protected:
    void SetUp() override
    {
        Mkdir("/tmp/log");
        LogParam g_logParam = {
            .logLevel = "DEBUG",
            .logDir = "/tmp/log",
            .nodeName = "test-runtime",
            .modelName = "test",
            .maxSize = 100,
            .maxFiles = 1,
            .logFileWithTime = false,
            .logBufSecs = 30,
            .maxAsyncQueueSize = 1048510,
            .asyncThreadCount = 1,
            .alsoLog2Stderr = true,
        };
        InitLog(g_logParam);
    }

    void TearDown() override {}
};

TEST_F(HttpUtilTest, SignHttpRequest)
{
    std::string url =
        "/serverless/v2/functions/"
        "wisefunction:cn:iot:8d86c63b22e24d9ab650878b75408ea6:function:0@faas@python:latest/invocations";
    std::string accessKey = "access_key";
    SensitiveValue secretKey = std::string("secret_key");
    std::unordered_map<std::string, std::string> headers;
    headers[TRACE_ID_KEY_NEW] = "traceId";
    headers[INSTANCE_CPU_KEY] = "500";
    headers[INSTANCE_MEMORY_KEY] = "300";
    std::string body = "123";
    SignHttpRequest(accessKey, secretKey, headers, body, url);
    YRLOG_DEBUG(headers[AUTHORIZATION_KEY]);
    ASSERT_FALSE(headers[AUTHORIZATION_KEY].empty());
    bool ok = VerifyHttpRequest(accessKey, secretKey, headers, body, url);
    ASSERT_TRUE(ok);
    auto digest = GenerateRequestDigest(headers, body, url);
    YRLOG_DEBUG(digest);
    ASSERT_TRUE(!digest.empty());
}
}  // namespace test
}  // namespace YR