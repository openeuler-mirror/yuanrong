# Create Pod Resource Pool

## Function Introduction

Create pod resource pool for running functions when deploying openYuanrong cluster on K8s. This interface is an asynchronous interface. After creation is completed, you can obtain the Pod resource pool status through the [Query Interface](get_pod_pool.md).

## Interface Constraints

Pool affinity is implemented based on K8s scheduling capabilities, see [Assigning Pods to Nodes](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/){target="_blank"} for details. Usage constraints are as follows.

- Multiple pools are deployed with anti-affinity by default. If you need to modify this, you can configure it through the `affinities` parameter in [PoolInfo structure](api-data-struct-poolinfo).
- When configuring pool affinity attributes, both `node_selector` and `affinities` parameters in [PoolInfo structure](api-data-struct-poolinfo) will take effect simultaneously. When configuration conflicts, it may cause the pool to be in `pending` state and unable to start. It is recommended to configure only one of them.

Pool automatic scaling is controlled by `size` and `max_size` parameters in [PoolInfo structure](api-data-struct-poolinfo). For principles, see [Horizontal Pod Autoscaling](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/){target="_blank"}. Usage constraints are as follows.

- When automatic scaling is enabled: Each instance creation request triggers Pod scaling at most once, but the scaled Pods are not bound to that instance. There may be cases where scaled Pods are occupied by other instances. In this case, instances that cannot obtain available Pods will return creation failure after 2 minutes timeout.
- When fixing resource pool size: You can configure Pod recycling time by adding key-value pair `{"yr-idle-to-recycle":5}` to the `labels` parameter in [PoolInfo structure](api-data-struct-poolinfo), with value unit in seconds. If never recycling, configure as `{"yr-idle-to-recycle":"unlimited"}`.

## URI

POST /serverless/v1/podpools

## Request Parameters

Request Header Parameters

| **Parameter** | **Required** | **Parameter Type** | **Description** |
| ---------- | ----- | ---------- | ------------------------- |
| Content-Type | Yes | String | Message body type.<br> **Value:** It is recommended to fill in `application/json`. |

<br> Request Body Parameters

| **Parameter** | **Required** | **Parameter Type** | **Description** |
| ---------- | ----- | ---------- | ------------------------- |
| pools | Yes | Array of [PoolInfo](api-data-struct-poolinfo) objects | Pool information array.<br> **Constraint:** Array length greater than `0` |

(api-data-struct-poolinfo)=

<br> PoolInfo Type Parameters

| **Parameter** | **Required** | **Parameter Type** | **Description**|
| ---------- | ----- | ---------- | ------------------------- |
| id                             | Yes | String              | Pool ID, must be globally unique.<br> **Value:** Supports lowercase letters, numbers and hyphens, allowed length 1-40, cannot start or end with hyphen |
| group                          | No | String              | Pool group.<br> **Value:** Supports uppercase and lowercase letters, numbers and hyphens, allowed length 1-40, cannot start or end with hyphen, uses default value default when not set |
| reuse                          | No | Boolean             | Whether pool is reusable. When Pod is exclusive to function instance, if `reuse` is false, pool will immediately replenish new Pod, otherwise need to wait until instance Pod is destroyed before replenishing new pod.<br> **Value:** true or false, default `false`. |
| size                           | No | int                 | Pool size. When parameter `max_size` value is greater than `size`, `size` is the minimum replica count.<br> **Value:** Greater than or equal to 0, default value is `0`. |
| max_size                       | No | int                 | Pool maximum replica count. When `max_size` value is greater than parameter `size`, pool enables automatic scaling.<br> **Value:** Greater than or equal to 0 and greater than or equal to parameter `size` value. When `size` is not equal to 0, `max_size` is set to 0 or not set, `max_size` actual value will be consistent with `size`. |
| image                          | No | String              | runtime-manager component image tag, uses version default image when not set.<br> **Value:** Maximum length 200. |
| init_image                     | No | String              | function-agent-init component image tag, uses version default image when not set.<br> **Value:** Maximum length 200. |
| labels                         | No | Map[String,String]  | Custom pool labels.<br> **Value:** key-value form, follows K8s specification (key must be less than or equal to 63 characters, starts and ends with alphanumeric, contains hyphens (-), underscores (_), dots (.) and letters or numbers). |
| environment                    | No | Map[String,String]  | Custom runtime-manager container environment variables.<br> **Value:** key-value form, follows K8s specification (key consists of letters, numbers, underscores, dots or hyphens, but first character cannot be a number). |
| volumes                        | No | String              | Volume declarations needed by pool, supports HostPath, PVC.<br> **Value:** Follows K8s specification (name must be less than or equal to 63 characters, starts and ends with alphanumeric, contains hyphens (-), underscores (_), dots (.) and letters or numbers).<br> **Constraint:** If cannot be parsed normally, this configuration parameter will be ignored when creating pool. |
| volume_mounts                  | No | String              | Volume mount declarations needed by pool.<br> **Value:** Follows K8s specification (name must match volumes name; mountPath cannot contain ':').<br> **Constraint:** If cannot be parsed normally, this configuration parameter will be ignored when creating pool. |
| resources                      | Yes | [ResourceRequirement](api-data-struct-resourcerequirement) object | Pool resource declaration. |
| affinities                     | No | String              | Pool affinity declaration.<br> **Value:** Follows K8s specification (when configuring Pod affinity strategy, topologyKey cannot be empty; weight range is 1-100).<br> **Constraint:** If cannot be parsed normally, this configuration parameter will be ignored when creating pool. |
| node_selector                  | No | Map[String,String]  | Node label matching, pool will select specific nodes for scheduling.<br> **Value:** Follows K8s specification. |
| runtime_class_name             | No | String              | Container runtime class name.<br> **Value:** Not exceeding 64 characters and follows K8s specification. |
| tolerations                    | No | String              | Taints that pool can tolerate.<br> **Value:** Follows K8s specification. |
| horizontal_pod_autoscaler_spec | No | String              | Pool HPA declaration.<br> **Value:** Follows k8s specification (maxReplicas cannot be less than minReplicas). |
| topology_spread_constraints    | No | String              | Pool topology spread constraints.<br> **Value:** Follows k8s specification.<br> **Constraint:** When parameter `max_size` is greater than parameter `size` value, this parameter is not supported. |
| pod_pending_duration_threshold | No | int                 | Pool Pod pending state duration alert threshold. When pool Pod continuously stays in pending state longer than this value, alert will be triggered.<br> **Value:** Greater than or equal to 0, when equal to 0 actually uses default value `120` seconds. |
| idle_recycle_time              | No | [IdleRecyclePolicy](api-data-struct-idlerecyclepolicy) object | Configure automatic scaling pod idle recycling time.<br> **Constraint:** Only takes effect when parameter max_size is greater than parameter size value. When tenant isolation feature is enabled, idle recycling time configuration priority is higher than tenant Pod reuse window configuration. |

(api-data-struct-idlerecyclepolicy)=

<br> IdleRecyclePolicy Type Parameters

| **Parameter** | **Required** | **Parameter Type** | **Description** |
| ---------- | ----- | ---------- | ------------------------- |
| reserved | No | int | Reserved pod idle recycling time.<br> **Value:** Greater than or equal to -1, unit in seconds. -1 means never recycle, 0 means idle recycling not configured. |
| scaled   | No | int | Scaled pod idle recycling time.<br> **Value:** Greater than or equal to -1, unit in seconds. -1 means never recycle, 0 means idle recycling not configured. |

(api-data-struct-resourcerequirement)=

<br> ResourceRequirement Type Parameters

| **Parameter** | **Required** | **Parameter Type** | **Description** |
| ---------- | ----- | ---------- | ------------------------- |
| requests | Yes | Map[String,String] | Pod request resource definition.<br> **Value:** key-value format, follows k8s specification. cpu unit m, represents milli-core; mem unit Mi or Gi. |
| limits   | Yes | Map[String,String] | Pod limit resource definition.<br> **Value:** key-value format, follows k8s specification. cpu unit m, represents milli-core; mem unit Mi or Gi. |

## Response Parameters

| **Parameter** | **Required** | **Parameter Type** | **Description** |
| ---------- | ----- | ---------- | ------------------------- |
| code    | Yes | int                 | Error code. 0 indicates success, non-0 indicates failure. |
| message | Yes | String              | Error message. |
| result  | Yes | [CreateResult](api-data-struct-createresult) object | Yes | Creation result. |

(api-data-struct-createresult)=

<br> CreateResult Type Parameters

| **Parameter** | **Required** | **Type** | **Description** |
| ---------- | ----- | ---------- | ------------------------- |
| failed_pools | Array of String objects | Failed ID list |

## Request Example

POST {[meta service endpoint](api-meta-service-endpoint)}/serverless/v1/podpools

```json
{
    "pools": [
        {
            "id": "pool1",
            "size": 2,
            "max_size": 3,
            "group": "rg1",
            "reuse": true,
            "image": "runtime-manager:v1",
            "init_image": "function-agent-init:v1",
            "labels": {
              "label1": "val1"
            },
            "environment": {
              "env1": "key1"
            },
           "volumes": "[{\"name\":\"pv-function\",\"persistentVolumeClaim\":{\"claimName\":\"test-client-pvc-claim\"}}]",
      "volume_mounts": "[{\"name\":\"pv-function\",\"mountPath\":\"/home/sn/function-packages\"}]",
            "resources": {
                "limits": {
                    "cpu": "600m",
                    "memory": "512Mi"
                },
                "requests": {
                    "cpu": "600m",
                    "memory": "512Mi"
                }
            },
            "node_selector": {
              "node-role.kubernetes.io/controlplane": "true"
            },
            "runtime_class_name": "runc",
            "idle_recycle_time": {
                "reserved": -1,
                "scaled": 10
            },
            "tolerations": "[{\"key\":\"is-ds-worker-unready\",\"operator\": \"Equal\", \"value\": \"true\", \"effect\": \"NoSchedule\"}]",
            "affinities": "{\"nodeAffinity\": {\"requiredDuringSchedulingIgnoredDuringExecution\":{\"nodeSelectorTerms\":[{\"matchExpressions\":[{\"key\":\"node-type\",\"operator\":\"In\",\"values\":[\"system\"]}]}]}}}",
            "horizontal_pod_autoscaler_spec": "{\"minReplicas\": 1, \"maxReplicas\": 2, \"metrics\":[{\"resource\": {\"name\":\"cpu\", \"target\":{\"averageUtilization\":20, \"type\":\"Utilization\"}}, \"type\":\"Resource\"}, {\"resource\": {\"name\":\"memory\", \"target\":{\"averageUtilization\":50, \"type\":\"Utilization\"}}, \"type\":\"Resource\"}]}",
            "topology_spread_constraints": "[{\"maxSkew\":1,\"minDomains\":1,\"topologyKey\":\"kubernetes.io/hostname\",\"whenUnsatisfiable\":\"DoNotSchedule\", \"matchLabelKeys\": [\"pod-template-hash\"],\"labelSelector\":{\"matchLabels\": {\"rg1\":\"rg1\"}} }]"
        }
    ]
}
```

## Response Example

Status Code: 200

Indicates resource pool creation success.

```json
{
    "code": 0,
    "message": "",
    "result": {
        "failed_pools": null
    }
}
```

## Error Codes

Please refer to [Error Codes](error-code-rest-api).
