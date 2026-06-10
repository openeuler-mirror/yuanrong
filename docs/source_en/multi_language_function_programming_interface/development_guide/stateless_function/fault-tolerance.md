# Fault Tolerance

openYuanrong provides an automatic fault tolerance mechanism for stateless function execution:

- When a function instance is evicted due to priority or other reasons, the caller will automatically initiate rescheduling of the stateless function.
- When a function instance experiences network failures, the caller will automatically retry.

Additionally, openYuanrong supports configuring retries when stateless function instances fail due to other reasons such as core dump.

## Configuring Retries

::::{tab-set}

:::{tab-item} Python

```python
import yr

yr.init()

@yr.invoke
def add(n):
    print("run add")
    raise Exception('my test')

# Set retry count to 3
opt = yr.InvokeOptions(retry_times=3)
result = add.options(opt).invoke(1)
try:
    print(yr.get(result))
except Exception:
    pass

yr.finalize()
```

Running the program, in the function's standard output [log](../../../observability/logs.md) file, you will see four identical lines of output `run add`, indicating that the function executed once and then retried `3` times after failure.

:::

:::{tab-item} C++

For C++ language, we provide two configurations: retry count and retry judgment callback function `retryChecker`, supporting user-defined retries. You need to configure the `retryTimes` parameter greater than `0` for the retry judgment callback function to take effect.

The callback function prototype is as follows:

```c++
bool (*retryChecker)(const Exception &e) noexcept = nullptr;
```

```c++
#include <iostream>
#include <yr/yr.h>

int Add(int n)
{
    std::cout << "run Add" << std::endl;
    throw std::runtime_error("connection failed");
}
YR_INVOKE(Add)

bool RetryForConnection(const YR::Exception &e) noexcept
{
    // Trigger retry when error code is 2002 (code has core dump or exception) and error message contains "connection failed"
    if (e.Code() == YR::ErrorCode::ERR_USER_FUNCTION_EXCEPTION) {
        std::string msg = e.what();
        if (msg.find("connection failed") != std::string::npos) {
            return true;
        }
    }
    return false;
}

int main(int argc, char *argv[])
{
    YR::Init(YR::Config{}, argc, argv);

    // Set retry count to 3 and configure callback function
    YR::InvokeOptions opt;
    opt.retryTimes = 3;
    opt.retryChecker = RetryForConnection;

    try {
        auto ref = YR::Function(Add).Options(opt).Invoke(1);
        std::cout << *YR::Get(ref) << std::endl;
    } catch (const YR::Exception &e) {
        std::cout << e.what() << std::endl;
    }

    YR::Finalize();
    return 0;
}
```

Running the program, in the function's standard output log file, you will see four identical lines of output `run Add`, indicating that the function executed once and then retried `3` times after failure.

:::

:::{tab-item} Java

This feature is not currently supported.

:::
::::
