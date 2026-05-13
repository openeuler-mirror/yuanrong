/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2024-2025. All rights reserved.
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

#include <atomic>
#include <cstdint>
#include <memory>
#include <string>

#include "src/libruntime/libruntime_config.h"
#include "src/libruntime/metricsadaptor/metrics_adaptor.h"
#include "src/proto/libruntime.pb.h"

namespace YR {
namespace Libruntime {

class InvokeCollector {
public:
    explicit InvokeCollector(std::shared_ptr<MetricsAdaptor> metricsAdaptor);

    void OnGaugeMutation(const std::string &metricName);
    void OnUInt64CounterMutation(const std::string &metricName);
    void BeforeInvoke(const libruntime::MetaData &metaData, const LibruntimeConfig &config);
    void AfterInvoke(const libruntime::MetaData &metaData, const LibruntimeConfig &config);
    bool IsDefaultConcurrentMetricOverridden() const;
    bool IsDefaultInvokeMetricOverridden() const;

private:
    bool ShouldCollect(const libruntime::MetaData &metaData, const LibruntimeConfig &config) const;
    void LogMetricError(const ErrorInfo &err, const std::string &action) const;

    std::shared_ptr<MetricsAdaptor> metricsAdaptor_;
    std::atomic<bool> defaultConcurrentMetricOverridden_{false};
    std::atomic<bool> defaultInvokeMetricOverridden_{false};
    std::atomic<int64_t> activeDefaultConcurrentMetricReports_{0};
    std::atomic<int64_t> canceledDefaultConcurrentMetricReports_{0};
};

}  // namespace Libruntime
}  // namespace YR
