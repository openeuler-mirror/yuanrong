/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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

#include "http_utils.h"
namespace YR {
namespace Libruntime {
std::string HashToHex(const std::string &message)
{
    std::stringstream ss;
    SHA256AndHex(message, ss);
    return ss.str();
}

std::string GenerateRequestDigest(const std::unordered_map<std::string, std::string> &headers, const std::string &body,
                                  const std::string &url)
{
    std::stringstream ss;
    ss << url << "\n";
    if (auto iter = headers.find(TIMESTAMP_KEY); iter != headers.end()) {
        ss << iter->first << ": " << iter->second << "\n";
    }
    if (auto iter = headers.find(ACCESS_KEY); iter != headers.end()) {
        ss << iter->first << ": " << iter->second << "\n";
    }
    ss << body;
    return ss.str();
}

void SignHttpRequest(const std::string &accessKey, const SensitiveValue &secretKey,
                     std::unordered_map<std::string, std::string> &headers, const std::string &body,
                     const std::string &url)
{
    auto timestamp = GetCurrentUTCTime();
    headers[TIMESTAMP_KEY] = timestamp;
    headers[ACCESS_KEY] = accessKey;

    auto digest = GenerateRequestDigest(headers, body, url);
    auto digestHashHex = HashToHex(digest);
    auto signature = GetHMACSha256(secretKey, digestHashHex);
    std::stringstream rss;
    rss << "HMAC-SHA256 timestamp=" << headers.at(TIMESTAMP_KEY) << ",access_key=" << headers.at(ACCESS_KEY)
        << ",signature=" << signature;
    headers[AUTHORIZATION_KEY] = rss.str();
}

bool VerifyHttpRequest(const std::string &accessKey, const SensitiveValue &secretKey,
                       std::unordered_map<std::string, std::string> &headers, const std::string &body,
                       const std::string &url)
{
    auto tenantAccessKey = headers.find(ACCESS_KEY);
    if (tenantAccessKey == headers.end() || tenantAccessKey->second.empty()) {
        YRLOG_ERROR("failed to verify http request: failed to find ACCESS_KEY in headers");
        return false;
    }
    auto timestamp = headers.find(TIMESTAMP_KEY);
    if (timestamp == headers.end() || timestamp->second.empty()) {
        YRLOG_ERROR("failed to verify http request: failed to find TIMESTAMP in headers");
        return false;
    }

    auto currentTimestamp = GetCurrentUTCTime();
    if (IsLaterThan(currentTimestamp, timestamp->second, TIMESTAMP_EXPIRE_DURATION_SECONDS)) {
        YRLOG_ERROR("failed to verify http request: failed to verify timestamp, difference is more than 1 min {} vs {}",
                    currentTimestamp, timestamp->second);
        return false;
    }

    auto authorizationValue = headers.find(AUTHORIZATION_KEY);
    if (authorizationValue == headers.end() || authorizationValue->second.empty()) {
        YRLOG_ERROR("failed to verify http request: failed to find Authorization in headers");
        return false;
    }
    std::string key = "signature=";
    size_t pos = authorizationValue->second.find(key);
    if (pos == std::string::npos) {
        return false;
    }
    std::string signature = authorizationValue->second.substr(pos + key.length());
    auto digest = GenerateRequestDigest(headers, body, url);
    auto digestHashHex = HashToHex(digest);
    if (GetHMACSha256(secretKey, digestHashHex) != signature) {
        YRLOG_ERROR("failed to verify http request");
        return false;
    }
    return true;
}
}  // namespace Libruntime
}  // namespace YR