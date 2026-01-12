Python SDK
================

yr.Context
---------------------

.. py:class:: yr.Context(options: dict)

    基类：``object``

    openYuanrong 运行时提供的上下文信息。

    **方法**：

    .. list-table::
       :header-rows: 0
       :widths: 30 70

       * - :ref:`__init__ <init_context>`
         -
       * - :ref:`getAccessKey <getAccessKey>`
         - 获取用户的 AccessKey（有效期为24小时）；此方法需为函数配置委托权限。
       * - :ref:`getAlias <getAlias>`
         - 获取函数别名。
       * - :ref:`getAuthToken <getAuthToken>`
         - 获取用户的委托令牌（有效期为 24 小时）；要使用此方法，您需要为该函数配置委托权限。
       * - :ref:`getCPUNumber <getCPUNumber>`
         - 获取分配给正在运行的函数的 CPU 数量（CPU 数量按千分之一核计算，1 个 CPU 核心等于 1000 千分之一核）。
       * - :ref:`getFunctionName <getFunctionName>`
         - 获取函数名。
       * - :ref:`getLogger <getLogger>`
         - 获取用于用户在标准输出中打印日志的记录器，SDK 中必须提供 Logger 接口。
       * - :ref:`getMemorySize <getMemorySize>`
         - 获取分配给正在运行的函数的内存大小。
       * - :ref:`getPackage <getPackage>`
         - 获取函数包。
       * - :ref:`getProjectID <getProjectID>`
         - 获取 Project ID。
       * - :ref:`getRemainingTimeInMilliSeconds <getRemainingTimeInMilliSeconds>`
         - 获取函数的剩余运行时间（单位：毫秒）。
       * - :ref:`getRequestID <getRequestID>`
         - 获取 Request ID。
       * - :ref:`getRunningTimeInSeconds <getRunningTimeInSeconds>`
         - 获取函数的超时时间（单位：秒）。
       * - :ref:`getSecretKey <getSecretKey>`
         - 获取用户的 SecretKey（有效期为 24 小时）；要使用此方法，您需要为该函数配置委托权限。
       * - :ref:`getSecurityAccessKey <getSecurityAccessKey>`
         - 获取用户的委托 SecurityAccessKey（有效期为 24 小时），缓存时长为 10 分钟，即如果在 10 分钟内再次获取，则返回相同的内容。
       * - :ref:`getSecuritySecretKey <getSecuritySecretKey>`
         - 获取用户的委托 SecuritySecretKey（有效期为 24 小时），缓存时长为 10 分钟，即如果在 10 分钟内再次获取，则返回相同的内容。
       * - :ref:`getSecurityToken <getSecurityToken>`
         - 获取用户的委托 SecurityToken（有效期为 24 小时），缓存时长为 10 分钟，即如果在 10 分钟内再次获取，则返回相同的内容。
       * - :ref:`getTenantID <getTenantID>`
         - 获取租户 ID。
       * - :ref:`getUserData <getUserData>`
         - 根据键获取用户通过环境变量传入的值。
       * - :ref:`getVersion <getVersion>`
         - 获取函数版本。

.. toctree::
    :maxdepth: 1
    :hidden:

    yr.Context.__init__
    yr.Context.getAccessKey
    yr.Context.getAlias
    yr.Context.getAuthToken
    yr.Context.getCPUNumber
    yr.Context.getFunctionName
    yr.Context.getLogger
    yr.Context.getMemorySize
    yr.Context.getPackage
    yr.Context.getProjectID
    yr.Context.getRemainingTimeInMilliSeconds
    yr.Context.getRequestID
    yr.Context.getRunningTimeInSeconds
    yr.Context.getSecretKey
    yr.Context.getSecurityAccessKey
    yr.Context.getSecuritySecretKey
    yr.Context.getSecurityToken
    yr.Context.getTenantID
    yr.Context.getUserData
    yr.Context.getVersion

yr.Function
------------------

.. py:class:: yr.Function(function_name: str, context_: Context | None = None)

    基类：``object``

    提供函数互调能力。

    **方法**：

    .. list-table::
       :header-rows: 0
       :widths: 30 70

       * - :ref:`__init__ <init_func>`
         -
       * - :ref:`invoke <invoke_func>`
         - 调用函数。
       * - :ref:`options <options_func>`
         - 设置用户调用选项。

.. toctree::
    :maxdepth: 1
    :hidden:

    yr.Function.__init__
    yr.Function.invoke
    yr.Function.options
    
