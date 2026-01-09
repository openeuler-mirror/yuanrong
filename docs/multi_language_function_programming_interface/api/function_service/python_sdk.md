# Python 服务开发 SDK

函数实例处理请求时，openYuanrong 运行时会将上下文对象（context）作为参数传递到 Handler 等方法中。该对象包含有关函数、执行环境等信息。

Python 运行时提供的上下文对象方法如下。

| 方法名         | 方法说明                |
|---------------|-------------------------|
| getRequestID()                   | 获取请求 ID。 |
| getRemainingTimeInMilliSeconds() | 获取函数剩余运行时间。|
| getSecurityAccessKey()           | 获取用户委托的 SecurityAccessKey（有效期 24 小时），使用该方法需要给函数配置委托。 |
| getSecuritySecretKey()           | 获取用户委托的 SecuritySecretKey（有效期 24 小时），使用该方法需要给函数配置委托。 |
| getSecurityToken()               | 获取用户委托的 SecurityToken（有效期 24 小时），使用该方法需要给函数配置委托。 |
| getUserData(string key)          | 通过 key 获取用户通过环境变量传入的值。|
| getFunctionName()                | 获取函数名称。|
| getRunningTimeInSeconds()        | 获取函数超时时间。 |
| getVersion()                     | 获取函数的版本。|
| getMemorySize()                  | 获取函数占用的内存资源。 |
| getCPUNumber()                   | 获取函数占用的 CPU 资源。 |
| getPackage()                     | 获取函数包名称。|
| getAuthToken()                   | 获取用户委托的 token（有效期 24 小时），使用该方法需要给函数配置委托。|
| getLogger()                      | 获取 context 提供的 logger 方法，返回一个日志输出类，通过使用其 info 方法按“时间-请求 ID-输出内容”的格式输出日志。 |
| getAlias()                       | 获取函数的别名。|
