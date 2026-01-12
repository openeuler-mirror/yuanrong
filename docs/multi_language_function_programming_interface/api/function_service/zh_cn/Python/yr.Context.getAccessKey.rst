.. _getAccessKey:

yr.Context.getAccessKey
---------------------------------------------------

.. py:method:: Context.getAccessKey()

    获取用户的 AccessKey（有效期为 24 小时）；要使用此方法，您需要为函数配置委托权限。当前函数工作流已停止维护 Runtime SDK 中的 getAccessKey 接口，
    您将无法再通过该接口获取临时 AccessKey。

    返回：
        用户的委托 AccessKey。

