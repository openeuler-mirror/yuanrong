# 应用指标

## 函数实例维度

| 指标名称                      | 指标含义    | 采集周期  | 单位                                                                                                    | 标签属性                                                                   |
|---------------------------|---------|-------|-------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------|
| yr_app_instance_status    | 实例状态信息  | 按实例采集 | state （枚举）；枚举值为： 1（scheduling）， 2（creating），3（Running），4（Failed），5（Exited），6（Fatal），7（ScheduleFailed） | instance_id, node_id, ip, function_key, timestamp, agent_id, parent_id |
| yr_instance_exit_latency | 实例退出时间  | 按实例采集 | ms                                                                                                    | instance_id, status_code, agent_id, start_ms, end_ms                   |
| yr_instance_memory_usage  | 实例运行内存占用 | 按实例采集 | KB                                                                                                    | instance_id, node_id, timestamp, agent_id                              |

### 函数实例维度标签属性说明

| 标签属性         | 说明         | 示例                                                 |
|--------------|------------|----------------------------------------------------|
| instance_id  | 实例 ID      | 317431e-e910-4000-8000-0000003bf404                |
| node_id      | 节点名称       | pekphis355665                                      |
| ip           | 节点 IP 地址   | 127.0.0.1                                          |
| timestamp    | 指标获取的时间（时间戳） | 1694413250917                                      |
| function_key | 函数关键字      | 1234567890123456/0-yr-test38/$lastest              |
| agent_id     | 函数实例所在 agent | function-agnet-large-pool-8000-16384-9dff909-zpz54 |
| parent_id    | 创建实例的对象 ID | fhdisa1-eer0-5748-67934-7r589433bf694              |
| status_code  | 执行步骤的结果状态值 |                                                    |
| start_ms     | 指标获取的开始时间（时间戳） | 1694413250917                                      |
| end_ms       | 指标获取的结束时间（时间戳） | 1694413250999                                      |

## 计费维度

| 指标名称                                   | 指标含义   | 采集周期           | 单位 | 指标属性                                                                                                          |
|----------------------------------------|--------|----------------|------|---------------------------------------------------------------------------------------------------------------|
| yr_app_instance_billing_invoke_latency | 函数调用时长 | 按调用采集          | ms | request_id, function_name, status_code, start_ms, end_ms, interval_ms, pool_label, cpu_type                   |
| yr_instance_running_duration           | 实例运行时长 | 实例运行周期性（15s）采集 | ms | instance_id, cpu_type, xpu_type, init_ms, last_report_ms, report_ms, pool_label, agent_id, required_resources |

### 计费维度标签属性说明

| 标签属性               | 说明              | 示例                                                                                                                                 |
|--------------------|-----------------|------------------------------------------------------------------------------------------------------------------------------------|
| cpu_type           | 处理器类型信息         | Intel(R) Xeon Gold 6278C CPU @ 2.60GHz                                                                                             |
| xpu_type           | NPU 或 GPU 类型信息  | 910B4                                                                                                                              |
| pool_label         | 函数实例 Pod 的标签    | ["NODE_ID:pekphis355665", "app:function-agent-runtime-pools-2000-4000","resource.owner:default",reuse:true","runtimepool5:value1"] |
| status_code        | Invoke 请求的成功与失败码 | 0                                                                                                                                  |
| start_ms           | 函数执行开始时间（时间戳），单位为 ms（毫秒） | 1694413250950                                                                                                                      |
| end_ms             | 函数执行结束时间（时间戳），单位为 ms（毫秒） | 1694413250989                                                                                                                      |
| interval_ms        | 函数执行时间间隔，单位为 ms（毫秒） | 1360                                                                                                                               |
| agent_id           | 函数实例所在 agent    | function-agent-large-pool-8000-16384-9dfd976f6-zpz54                                                                               |
| request_id         | 请求ID            | 4e524b77a5b9352100                                                                                                                 |
| function_name      | 函数名             | 12345678901234567890123456/0@faas001@hello/lastest                                                                                 |
| instance_id        | 实例 ID           | r673481e-e910-4000-8000-0000003bf404                                                                                               |
| init_ms            | 实例创建时间（时间戳），单位为 ms（毫秒） | 1694467803989                                                                                                                      |
| last_report_ms     | 最近一次上报时间（时间戳），单位为 ms（毫秒） | 1694469803989                                                                                                                      |
| report_ms          | 本次上报时间（时间戳）， 单位为 ms（毫秒） | 1694469903989                                                                                                                      |
| required_resources | 实例申请的资源信息       | {"CPU":"500.000000","Memory":"500.000000","NPU/.+/count":"1.000000"}                                                               |
