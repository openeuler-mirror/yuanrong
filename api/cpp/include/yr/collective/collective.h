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

#include <memory>
#include <mutex>
#include <string>
#include <vector>
#include <unordered_map>
#include <utility>

#include "reduce_op.h"

namespace YR::collective {
enum Backend : uint8_t {
    GLOO = 0,

    INVALID,
};

class CollectiveGroup {
public:
    CollectiveGroup(std::string groupName, int worldSize, int rank, Backend backend)
            : groupName_(std::move(groupName)), rank_(rank), backend_(backend), worldSize_(worldSize)
    {
    }

    virtual void AllReduce(const void *sendbuf, void *recvbuf, int count, DataType dtype, const ReduceOp &op) = 0;

    virtual void Reduce(const void *sendbuf, void *recvbuf, int count, DataType dtype, const ReduceOp &op,
                        int dstRank) = 0;

    virtual void AllGather(const void *sendbuf, void *recvbuf, int count, DataType dtype) = 0;

    virtual void Barrier() = 0;

    virtual void Scatter(const void *sendbuf, void *recvbuf, int count, DataType dtype, int srcRank) = 0;

    virtual void Broadcast(const void *sendbuf, void *recvbuf, int count, DataType dtype, int srcRank) = 0;

    virtual void Recv(void *recvbuf, int count, int srcRank, int tag) = 0;

    virtual void Send(const void *sendbuf, int count, int dstRank, int tag) = 0;

    int GetRank() const;
    std::string GetGroupName();
    Backend GetBackend();
    int GetWorldSize() const;

protected:
    std::string groupName_;

    int rank_;

    Backend backend_;

    int worldSize_;
};

/**
 * init collective group in actor
 *
 * @param worldSize
 * @param rank
 * @param groupName
 */
void InitCollectiveGroup(int worldSize, int rank, const std::string &groupName, Backend backend);

/**
 * create collective group with actor ids in driver
 *
 * @param instanceIDs
 * @param worldSize
 * @param ranks
 * @param groupName
 */
void CreateCollectiveGroup(const std::vector<std::string> &instanceIDs, int worldSize, const std::vector<int> &ranks,
                           const std::string &groupName, Backend backend);
/**
 *
 *
 * @param groupName
 */
void DestroyCollectiveGroup(const std::string &groupName);

void AllReduce(const void *sendbuf, void *recvbuf, int count, DataType dtype, const ReduceOp &op,
               const std::string &groupName);

void Reduce(const void *sendbuf, void *recvbuf, int count, DataType dtype, const ReduceOp &op, int dstRank,
            const std::string &groupName);

void AllGather(const void *sendbuf, void *recvbuf, int count, DataType dtype, const std::string &groupName);

void Barrier(const std::string &groupName);

void Scatter(const void *sendbuf, void *recvbuf, int count, DataType dtype, int srcRank, const std::string &groupName);

void Broadcast(const void *sendbuf, void *recvbuf, int count, DataType dtype, int srcRank,
               const std::string &groupName);

void Recv(void *recvbuf, int count, int srcRank, int tag, const std::string &groupName);

void Send(const void *sendbuf, int count, int dstRank, int tag, const std::string &groupName);

class CollectiveGroupMgr {
public:
    static CollectiveGroupMgr &GetInstance()
    {
        static CollectiveGroupMgr instance;
        return instance;
    }

    std::shared_ptr<CollectiveGroup> CheckAndCreateGroup(const std::string &groupName);

    void InitCollectiveGroup(int worldSize, int rank, const std::string &groupName, Backend backend);

    void DestroyCollectiveGroup(const std::string &groupName);

private:
    CollectiveGroupMgr() = default;

    ~CollectiveGroupMgr() = default;

    std::recursive_mutex mtx_{};

    std::unordered_map<std::string, std::shared_ptr<CollectiveGroup>> groups_{};
};

}  // namespace YR::collective
