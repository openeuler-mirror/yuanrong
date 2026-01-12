.. _getSecretKey:

yr.Context.getSecretKey
------------------------

.. py:method:: Context.getSecretKey()

    获取用户的 SecretKey（有效期为 24 小时）；要使用此方法，您需要为函数配置委托权限。当前函数工作流已在 Runtime SDK 中停止维护 getSecretKey 接口，
    您将无法再通过该接口获取临时 SecretKey。

    返回：
        用户的委托 SecretKey。


