yr.exception.YRequestError
==============================

.. py:exception:: exception yr.exception.YRequestError(code: int = 0, message: str = '', request_id='')

    请求失败错误。

    ``YRequestError`` 继承自 :doc:`yr.exception.YRRuntimeError`，因此仍可通过 ``except RuntimeError`` 捕获，同时可以通过 ``code``、``module_code``、``message`` 等属性读取结构化错误信息。
