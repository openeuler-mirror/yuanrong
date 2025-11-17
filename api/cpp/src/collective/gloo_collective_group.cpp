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

#include "gloo_collective_group.h"

#include <gloo/algorithm.h>
#include <gloo/allgather.h>
#include <gloo/barrier.h>
#include <gloo/broadcast.h>
#include <gloo/reduce.h>
#include <gloo/rendezvous/prefix_store.h>
#include <gloo/scatter.h>
#include <gloo/transport/tcp/device.h>
#include <gloo/transport/ibverbs//device.h>

#include "api/cpp/src/utils/utils.h"
#include "src/dto/config.h"
#include "yr/yr.h"

namespace YR::collective {

#define EXECUTE_BY_TYPE(dType, OPERATION, ...)                                \
    switch (dType) {                                                          \
        case DataType::INT:                                                   \
            OPERATION<int>(__VA_ARGS__);                                      \
            break;                                                            \
        case DataType::DOUBLE:                                                \
            OPERATION<double>(__VA_ARGS__);                                   \
            break;                                                            \
        case DataType::INVALID:                                               \
        default:                                                              \
            throw YR::Exception(YR::Libruntime::ErrorCode::ERR_PARAM_INVALID, \
                                "invalid dType: " + std::to_string(dType));   \
    }

struct DsStore : public gloo::rendezvous::Store {
public:
    DsStore() = default;
    ~DsStore() override = default;

    void set(const std::string &key, const std::vector<char> &data) override
    {
        YR::KVManager::Set(key, std::string(data.begin(), data.end()));
    }

    std::vector<char> get(const std::string &key) override
    {
        std::vector<char> out;
        std::string result = YR::KVManager::Get(key);
        out.assign(result.begin(), result.end());
        return out;
    }

    void wait(const std::vector<std::string> &keys) override
    {
        YR::KVManager::Get(keys);
    }

    void wait(const std::vector<std::string> &keys, const std::chrono::milliseconds &timeout) override
    {
        YR::KVManager::Get(keys, static_cast<int>(timeout.count() / 1000));
    }
};

GlooCollectiveGroup::GlooCollectiveGroup(std::string groupName, int worldSize, int rank)
    : CollectiveGroup(std::move(groupName), worldSize, rank, Backend::GLOO)
{
    auto dsStore = std::make_shared<DsStore>();
    auto prefixStore = std::make_shared<gloo::rendezvous::PrefixStore>(groupName_, dsStore);

    std::shared_ptr<gloo::transport::Device> dev;
    auto backend = GetEnv("GLOO_BACKEND_TYPE");
    if (backend.empty() || backend == "TCP") {
        gloo::transport::tcp::attr attr;
        attr.hostname = YR::Libruntime::Config::Instance().HOST_IP();
        dev = gloo::transport::tcp::CreateDevice(attr);
    } else if (backend == "IBVERBS") {
        gloo::transport::ibverbs::attr attr;
        attr.name = GetEnv("GLOO_IBVERBS_NAME");
        dev = gloo::transport::ibverbs::CreateDevice(attr);
    } else {
        throw YR::Exception(YR::Libruntime::ErrorCode::ERR_PARAM_INVALID,
                            "invalid backend type: " + backend);
    }

    context_ = std::make_shared<gloo::rendezvous::Context>(rank_, worldSize_);
    context_->connectFullMesh(prefixStore, dev);
}

void GlooCollectiveGroup::AllReduce(const void *sendbuf, void *recvbuf, int count, DataType dtype, const ReduceOp &op)
{
    EXECUTE_BY_TYPE(dtype, DoAllReduce, sendbuf, recvbuf, count, op);
}

void GlooCollectiveGroup::Reduce(const void *sendbuf, void *recvbuf, int count, DataType dtype, const ReduceOp &op,
                                 int dstRank)
{
    EXECUTE_BY_TYPE(dtype, DoReduce, sendbuf, recvbuf, count, op, dstRank);
}

void GlooCollectiveGroup::AllGather(const void *sendbuf, void *recvbuf, int count, DataType dtype)
{
    EXECUTE_BY_TYPE(dtype, DoAllGather, sendbuf, recvbuf, count);
}

void GlooCollectiveGroup::Barrier()
{
    gloo::BarrierOptions opts(context_);
    gloo::barrier(opts);
}

void GlooCollectiveGroup::Scatter(const void *sendbuf, void *recvbuf, int count, DataType dtype, int srcRank)
{
    EXECUTE_BY_TYPE(dtype, DoScatter, sendbuf, recvbuf, count, srcRank);
}

void GlooCollectiveGroup::Broadcast(const void *sendbuf, void *recvbuf, int count, DataType dtype, int srcRank)
{
    EXECUTE_BY_TYPE(dtype, DoBroadcast, sendbuf, recvbuf, count, srcRank);
}

void GlooCollectiveGroup::Recv(void *recvbuf, int count, int srcRank, int tag)
{
    auto ubuf = context_->createUnboundBuffer(recvbuf, count);
    ubuf->recv(srcRank, tag);
    ubuf->waitRecv();
}

void GlooCollectiveGroup::Send(const void *sendbuf, int count, int dstRank, int tag)
{
    auto ubuf = context_->createUnboundBuffer(const_cast<void *>(sendbuf), count);
    ubuf->send(dstRank, tag);
    ubuf->waitSend();
}

template <typename T>
gloo::AllreduceOptions::Func GlooCollectiveGroup::GetReduceOp(const ReduceOp &op)
{
    void (*fn)(void *, const void *, const void *, long unsigned int) = &gloo::sum<T>;
    switch (op) {
        case ReduceOp::SUM:
            fn = &gloo::sum<T>;
            break;
        case ReduceOp::PRODUCT:
            fn = &gloo::product<T>;
            break;
        case ReduceOp::MIN:
            fn = &gloo::min<T>;
            break;
        case ReduceOp::MAX:
            fn = &gloo::max<T>;
            break;
    }
    return fn;
}

template <typename T>
void GlooCollectiveGroup::DoAllReduce(const void *sendbuf, void *recvbuf, int count, const ReduceOp &op)
{
    gloo::AllreduceOptions opts(context_);
    opts.setInput(const_cast<T *>((T *)sendbuf), count);
    opts.setOutput(static_cast<T *>(recvbuf), count);
    opts.setReduceFunction(GetReduceOp<T>(op));
    gloo::allreduce(opts);
}

template <typename T>
void GlooCollectiveGroup::DoReduce(const void *sendbuf, void *recvbuf, int count, const ReduceOp &op, int dstRank)
{
    gloo::ReduceOptions opts(context_);
    opts.setInput(const_cast<T *>((T *)sendbuf), count);
    opts.setOutput(static_cast<T *>(recvbuf), count);
    opts.setReduceFunction(GetReduceOp<T>(op));
    gloo::reduce(opts);
}

template <typename T>
void GlooCollectiveGroup::DoBroadcast(const void *sendbuf, void *recvbuf, int count, int srcRank)
{
    gloo::BroadcastOptions opts(context_);
    opts.setInput(const_cast<T *>((T *)sendbuf), count);
    opts.setOutput(static_cast<T *>(recvbuf), count);
    opts.setRoot(srcRank);
    gloo::broadcast(opts);
}

template <typename T>
void GlooCollectiveGroup::DoScatter(const void *sendbuf, void *recvbuf, int count, int srcRank)
{
    gloo::ScatterOptions opts(context_);
    std::vector<T *> inputs = {const_cast<T *>((T *)sendbuf)};
    opts.setInputs(inputs, count);
    opts.setOutput(static_cast<T *>(recvbuf), count);
    opts.setRoot(srcRank);
    gloo::scatter(opts);
}

template <typename T>
void GlooCollectiveGroup::DoAllGather(const void *sendbuf, void *recvbuf, int count)
{
    gloo::AllgatherOptions opts(context_);
    opts.setInput(const_cast<T *>((T *)sendbuf), count);
    opts.setOutput(static_cast<T *>(recvbuf), count);
    gloo::allgather(opts);
}
}  // namespace YR::collective