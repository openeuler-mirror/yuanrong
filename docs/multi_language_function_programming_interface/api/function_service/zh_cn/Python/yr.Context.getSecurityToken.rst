.. _getSecurityToken:

yr.Context.getSecurityToken
----------------------------

.. py:method:: Context.getSecurityToken()

    获取用户委托的 SecurityToken（有效期为 24 小时），该 Token 具有 10 分钟的缓存时长，意味着在 10 分钟内再次获取时将返回相同的内容。使用此方法需要为函数配置委托权限。

    返回：
        用户的委托 SecurityToken。
