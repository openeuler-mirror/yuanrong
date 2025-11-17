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

#include "trace_adapter.h"
#include <json.hpp>

#include "src/dto/config.h"
#include "src/libruntime/traceadaptor/exporter/log_file_exporter_factory.h"
#include "src/utility/logger/logger.h"

namespace YR {
namespace Libruntime {

TraceAdapter::~TraceAdapter() noexcept
{
}

void TraceAdapter::InitTrace(const std::string &serviceName, const bool &enableTrace, const std::string &traceConfig)
{
    enableTrace_ = enableTrace;
    YRLOG_DEBUG("init trace, enableTrace is {}, traceConfig is {}", enableTrace, traceConfig);
    if (!enableTrace_) {
        return;
    }
    std::vector<std::unique_ptr<trace_sdk::SpanProcessor>> processors;
    try {
        auto confJson = nlohmann::json::parse(traceConfig);
        for (auto &element : confJson.items()) {
            if (element.key() == OTLP_GRPC_EXPORTER) {
                if (!element.value().contains("enable") || !element.value().at("enable").get<bool>()) {
                    YRLOG_INFO("Trace exporter {} is not enabled", OTLP_GRPC_EXPORTER);
                    continue;
                }
                if (!element.value().contains("endpoint")
                    || element.value().at("endpoint").get<std::string>().empty()) {
                    YRLOG_INFO("Trace exporter {} endpoint is not valid", OTLP_GRPC_EXPORTER);
                    continue;
                    }
                OtelGrpcExporterConfig config;
                config.endpoint = element.value().at("endpoint").get<std::string>();
                opentelemetry::sdk::trace::BatchSpanProcessorOptions batchSpanProcessorOptions;
                YRLOG_INFO("OtelGrpcExporter is enable, endpoint is {}", config.endpoint);
                processors.push_back(
                    std::unique_ptr<trace_sdk::SpanProcessor>(trace_sdk::BatchSpanProcessorFactory::Create(
                        std::move(InitOtlpGrpcExporter(config)), batchSpanProcessorOptions)));
            } else if (element.key() == LOG_FILE_EXPORTER) {
                if (!element.value().contains("enable")
                    || !element.value().at("enable").get<bool>()) {
                    YRLOG_INFO("Trace exporter {} is not enabled", LOG_FILE_EXPORTER);
                    continue;
                    }
                opentelemetry::sdk::trace::BatchSpanProcessorOptions batchSpanProcessorOptions;
                YRLOG_INFO("logFileExporter is enable");
                processors.push_back(
                    std::unique_ptr<trace_sdk::SpanProcessor>(trace_sdk::BatchSpanProcessorFactory::Create(
                        std::move(InitLogFileExporter()), batchSpanProcessorOptions)));
            }
        }
    } catch (nlohmann::detail::parse_error &e) {
        YRLOG_ERROR("Failed to arse trace config json, error: {}", e.what());
        enableTrace_ = false;
        return;
    } catch (std::exception &e) {
        YRLOG_ERROR("Failed to parse trace config json, error: {}", e.what());
        enableTrace_ = false;
        return;
    }
    if (processors.empty()) {
        YRLOG_WARN("There is no supported exporter in config");
        enableTrace_ = false;
        return;
    }
    opentelemetry::sdk::resource::ResourceAttributes attributes = {
        { opentelemetry::sdk::resource::SemanticConventions::kTelemetrySdkLanguage, "" },
        { opentelemetry::sdk::resource::SemanticConventions::kTelemetrySdkName, "" },
        { opentelemetry::sdk::resource::SemanticConventions::kTelemetrySdkVersion, "" },
        { opentelemetry::sdk::resource::SemanticConventions::kServiceName, serviceName },
    };
    auto provider = std::shared_ptr<trace_api::TracerProvider>(std::make_shared<trace_sdk::TracerProvider>(
        std::move(processors), opentelemetry::sdk::resource::Resource::Create(attributes)));
    trace_api::Provider::SetTracerProvider(provider);
}

void TraceAdapter::ShutDown()
{
    if (!enableTrace_) {
        return;
    }
    YRLOG_INFO("enter traceAdapter shutDown");
    enableTrace_ = false;
    auto provider = trace_api::Provider::GetTracerProvider();
    auto traceProvider = static_cast<trace_sdk::TracerProvider*>(provider.get());
    if (traceProvider != nullptr && !traceProvider->ForceFlush()) {
        YRLOG_WARN("traceProvider shutDown failed");
    }
    std::shared_ptr<trace_api::TracerProvider> none;
    trace_api::Provider::SetTracerProvider(none);
}

void TraceAdapter::SetAttr(const std::string &attr, const std::string &value)
{
    attribute_.insert_or_assign(attr, value);
}

OtelSpan TraceAdapter::StartSpan(const std::string &name, const opentelemetry::common::KeyValueIterable &attributes,
                                 const opentelemetry::trace::SpanContextKeyValueIterable &links,
                                 const opentelemetry::trace::StartSpanOptions &startSpanOptions)
{
    if (enableTrace_) {
        auto tracer = GetTracer();
        if (tracer != nullptr) {
            return tracer->StartSpan(name, attributes, links, startSpanOptions);
        }
    }
    std::shared_ptr<trace_api::Tracer> noopTracer = std::make_shared<trace_api::NoopTracer>();
    return opentelemetry::nostd::shared_ptr<trace_api::Span>(new trace_api::NoopSpan(noopTracer));
}

OtelSpan TraceAdapter::StartSpan(const std::string &name,
                                 const opentelemetry::trace::StartSpanOptions &startSpanOptions)
{
    return StartSpan(name, opentelemetry::common::NoopKeyValueIterable(), opentelemetry::trace::NullSpanContext(),
                     startSpanOptions);
}

OtelSpan TraceAdapter::StartSpan(
    const std::string &name,
    std::vector<std::pair<const std::string, const opentelemetry::common::AttributeValue>> attrs,
    const opentelemetry::trace::StartSpanOptions &startSpanOptions)
{
    // preset system attr
    for (const auto &it : attribute_) {
        attrs.emplace_back(it);
    }
    return StartSpan(name, opentelemetry::common::KeyValueIterableView(attrs), opentelemetry::trace::NullSpanContext(),
                     startSpanOptions);
}

opentelemetry::nostd::shared_ptr<opentelemetry::trace::Tracer> TraceAdapter::GetTracer(const std::string &name,
                                                                                       const std::string &version)
{
    auto provider = opentelemetry::trace::Provider::GetTracerProvider();
    return provider->GetTracer(name, version);
}

std::unique_ptr<opentelemetry::sdk::trace::SpanExporter> TraceAdapter::InitLogFileExporter()
{
    return LogFileExporterFactory::Create();
}

std::unique_ptr<opentelemetry::sdk::trace::SpanExporter> TraceAdapter::InitOtlpGrpcExporter(
    const OtelGrpcExporterConfig &conf)
{
    if (conf.endpoint.empty()) {
        return nullptr;
    }
    opentelemetry::exporter::otlp::OtlpGrpcExporterOptions options;
    options.endpoint = conf.endpoint;
    return opentelemetry::exporter::otlp::OtlpGrpcExporterFactory::Create(options);
}
}
}
