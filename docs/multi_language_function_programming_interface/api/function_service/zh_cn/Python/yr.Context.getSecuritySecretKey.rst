.. _getSecuritySecretKey:

yr.Context.getSecuritySecretKey
--------------------------------

.. py:method:: Context.getSecuritySecretKey()

    获取用户委托的 SecuritySecretKey（有效期为 24 小时），该 Key 具有 10 分钟的缓存时长，意味着在 10 分钟内再次获取时将返回相同的内容。使用此方法需要为函数配置委托权限。

    返回：
        用户的委托 SecuritySecretKey。
