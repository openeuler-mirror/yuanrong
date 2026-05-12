/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2026. All rights reserved.
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

#include <gmock/gmock.h>
#include <gtest/gtest.h>
#include <chrono>
#include <cstdlib>
#include <memory>
#include <string>
#include <thread>

#define private public
#include "src/dto/config.h"
#include "src/libruntime/metricsadaptor/invoke_collector.h"
#include "src/libruntime/metricsadaptor/metrics_adaptor.h"
#undef private
#include "src/utility/logger/fileutils.h"
#include "src/utility/logger/logger.h"

using namespace testing;
using namespace YR::utility;
using namespace YR::Libruntime;

namespace YR {
namespace test {
namespace {
std::shared_ptr<MetricsAdaptor> BuildSampleOnlyMetricsAdaptor()
{
    auto metricsAdaptor = std::make_shared<MetricsAdaptor>();
    metricsAdaptor->userEnable_ = true;
    metricsAdaptor->prometheusPullExporterEnabled_ = true;
    return metricsAdaptor;
}

libruntime::MetaData BuildInvokeMetaData()
{
    libruntime::MetaData metaData;
    metaData.set_invoketype(libruntime::InvokeType::InvokeFunction);
    return metaData;
}

LibruntimeConfig BuildInvokeConfig()
{
    LibruntimeConfig config;
    config.enableMetrics = true;
    config.selfApiType = libruntime::ApiType::Faas;
    return config;
}
}  // namespace

class InvokeCollectorTest : public testing::Test {
public:
    void SetUp() override
    {
        setenv("ENABLE_METRICS", "true", 1);
        Config::c = Config();
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

    void TearDown() override
    {
        unsetenv("ENABLE_METRICS");
        Config::c = Config();
    }
};

TEST_F(InvokeCollectorTest, DefaultMetricsReportTest)
{
    auto metricsAdaptor = BuildSampleOnlyMetricsAdaptor();
    InvokeCollector collector(metricsAdaptor);
    auto metaData = BuildInvokeMetaData();
    auto config = BuildInvokeConfig();

    collector.BeforeInvoke(metaData, config);
    collector.AfterInvoke(metaData, config);

    GaugeData gauge;
    gauge.name = "yr_custom_concurrent_num";
    auto gaugeValue = metricsAdaptor->GetValueGauge(gauge);
    ASSERT_TRUE(gaugeValue.first.OK());
    ASSERT_EQ(gaugeValue.second, 0);

    UInt64CounterData counter;
    counter.name = "yr_custom_invoke_num";
    auto counterValue = metricsAdaptor->GetValueUInt64Counter(counter);
    ASSERT_TRUE(counterValue.first.OK());
    ASSERT_EQ(counterValue.second, 1);
}

TEST_F(InvokeCollectorTest, BusinessOverrideStopsDefaultMetricsTest)
{
    auto metricsAdaptor = BuildSampleOnlyMetricsAdaptor();
    InvokeCollector collector(metricsAdaptor);
    auto metaData = BuildInvokeMetaData();
    auto config = BuildInvokeConfig();

    collector.BeforeInvoke(metaData, config);

    GaugeData gauge;
    gauge.name = "yr_custom_concurrent_num";
    gauge.description = "business override concurrent";
    gauge.unit = "count";
    gauge.value = 9;
    collector.OnGaugeMutation(gauge.name);
    ASSERT_EQ(metricsAdaptor->IncreaseGauge(gauge).Code(), ErrorCode::ERR_OK);
    ASSERT_TRUE(collector.IsDefaultConcurrentMetricOverridden());

    UInt64CounterData counter;
    counter.name = "yr_custom_invoke_num";
    counter.description = "business override invoke";
    counter.unit = "count";
    counter.value = 4;
    collector.OnUInt64CounterMutation(counter.name);
    ASSERT_EQ(metricsAdaptor->IncreaseUInt64Counter(counter).Code(), ErrorCode::ERR_OK);
    ASSERT_TRUE(collector.IsDefaultInvokeMetricOverridden());

    collector.AfterInvoke(metaData, config);

    GaugeData currentGauge;
    currentGauge.name = "yr_custom_concurrent_num";
    auto gaugeValue = metricsAdaptor->GetValueGauge(currentGauge);
    ASSERT_TRUE(gaugeValue.first.OK());
    ASSERT_EQ(gaugeValue.second, 9);

    UInt64CounterData currentCounter;
    currentCounter.name = "yr_custom_invoke_num";
    auto counterValue = metricsAdaptor->GetValueUInt64Counter(currentCounter);
    ASSERT_TRUE(counterValue.first.OK());
    ASSERT_EQ(counterValue.second, 4);
}
}  // namespace test
}  // namespace YR
