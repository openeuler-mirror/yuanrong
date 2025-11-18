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

#include "src/libruntime/groupmanager/named_group.h"
#include "src/libruntime/utils/utils.h"

namespace YR {
namespace Libruntime {

NamedGroup::NamedGroup(const std::string &name, const std::string &inputTenantId, GroupOpts &inputOpts,
                       std::shared_ptr<FSClient> client, std::shared_ptr<WaitingObjectManager> waitManager,
                       std::shared_ptr<MemoryStore> memStore, std::shared_ptr<InvokeOrderManager> invokeOrderMgr)
    : RangeGroup(name, inputTenantId, inputOpts, client, waitManager, memStore, invokeOrderMgr)
{
}

CreateRequests NamedGroup::BuildCreateReqs()
{
    CreateRequests reqs;
    reqs.set_tenantid(tenantId);
    reqs.set_requestid(YR::utility::IDGenerator::GenRequestId());
    reqs.set_traceid(createSpecs[0]->traceId);
    for (auto spec : createSpecs) {
        CreateRequest *req = reqs.add_requests();
        *req = spec->requestCreate;
    }
    GroupOptions *options = reqs.mutable_groupopt();
    options->set_groupname(groupName);
    options->set_timeout(opts.timeout);
    options->set_samerunninglifecycle(opts.sameLifecycle);
    options->set_grouppolicy(ConvertStrategyToPolicy(opts.strategy));
    return reqs;
}

void NamedGroup::SetTerminateError()
{
    for (auto &spec : createSpecs) {
        this->memStore_->SetError(spec->returnIds[0].id,
                                  ErrorInfo(ErrorCode::ERR_FINALIZED, ModuleCode::RUNTIME,
                                            "group ins had been terminated, return obj id is: " +
                                                spec->returnIds[0].id + " , instance id is: " + spec->instanceId,
                                            true));
    }
}
}  // namespace Libruntime
}  // namespace YR
