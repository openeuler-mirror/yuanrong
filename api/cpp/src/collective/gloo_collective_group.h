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

#pragma once
#include <gloo/allreduce.h>
#include <gloo/rendezvous/context.h>

#include "yr/collective/collective.h"

namespace YR::collective {
class GlooCollectiveGroup : public CollectiveGroup {
public:
    GlooCollectiveGroup(std::string groupName, int worldSize, int rank);

    void AllReduce(const void *sendbuf, void *recvbuf, int count, DataType dtype, const ReduceOp &op) override;

    void Reduce(const void *sendbuf, void *recvbuf, int count, DataType dtype, const ReduceOp &op,
                int dstRank) override;

    void AllGather(const void *sendbuf, void *recvbuf, int count, DataType dtype) override;

    void Barrier() override;

    void Scatter(const void *sendbuf, void *recvbuf, int count, DataType dtype, int srcRank) override;

    void Broadcast(const void *sendbuf, void *recvbuf, int count, DataType dtype, int srcRank) override;

    void Recv(void *recvbuf, int count, int srcRank, int tag) override;

    void Send(const void *sendbuf, int count, int dstRank, int tag) override;

private:
    template <typename T>
    void DoAllReduce(const void *sendbuf, void *recvbuf, int count, const ReduceOp &op);

    template <typename T>
    void DoReduce(const void *sendbuf, void *recvbuf, int count, const ReduceOp &op, int dstRank);

    template <typename T>
    void DoAllGather(const void *sendbuf, void *recvbuf, int count);

    template <typename T>
    void DoScatter(const void *sendbuf, void *recvbuf, int count, int srcRank);

    template <typename T>
    void DoBroadcast(const void *sendbuf, void *recvbuf, int count, int srcRank);

    template <typename T>
    static gloo::AllreduceOptions::Func GetReduceOp(const ReduceOp &op);

    std::unordered_map<ReduceOp, gloo::AllreduceOptions::Func> map;

    std::shared_ptr<gloo::rendezvous::Context> context_;
};
}  // namespace YR::collective