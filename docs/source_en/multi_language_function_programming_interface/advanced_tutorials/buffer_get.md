# Zero Serialization/Deserialization for APIs

openYuanrong supports directly passing `YR::Buffer` type parameters when using `Put` and `invoke` interfaces, which avoids serialization. When using the `Get` interface, it supports directly returning `YR::Buffer` type, which avoids deserialization. This approach can effectively improve API performance.

:::{Note}

The `YR::Buffer` parameter passed to the `Put` interface cannot be a null pointer or have size `0`, otherwise exception 1001 will be thrown.

:::

## Use Cases

- Pass `YR::Buffer` type parameters to the `Put` interface, and use the `Get` interface to retrieve `YR::Buffer` type return values.
- When a Cpp program cross-language calls a Python function, pass `YR::Buffer` type parameters in the `invoke` interface, and use the `Get` interface to retrieve `YR::Buffer` type return values.

## Usage Examples

### Using Put/Get Interfaces

The example demonstrates passing `YR::Buffer` type parameters to the `Put` interface, then calling the `Get` interface to retrieve `YR::Buffer` type return values.

Example code:

```c++
#include <string>

#include "yr/yr.h"

int main(int argc, char *argv[]) {
    YR::Init(YR::Config{}, argc, argv);

    YR::CreateParam param;
    param.writeMode = YR::WriteMode::NONE_L2_CACHE_EVICT;
    param.consistencyType = YR::ConsistencyType::PRAM;

    std::string str = "success";
    YR::Buffer yrBuf(str.data(), str.length());

    auto resRef = YR::Put(yrBuf, param);
    auto value = YR::Get(resRef);
    std::string result = std::string(static_cast<const char*>(value->ImmutableData()), value->GetSize());
    std::cout << result << std::endl;

    YR::Finalize();
    return 0;
}
```

### Calling Python Functions from Cpp Programs

This is an example of a Cpp program cross-language calling a Python function, passing `YR::Buffer` type parameters in the `invoke` interface, then using the `Get` interface to retrieve `YR::Buffer` type return values.

Example code:

```python
import yr

def echo(str):
    return str
```

```c++
#include <string>

#include "yr/yr.h"

int main(int argc, char *argv[]) {
    YR::Init(YR::Config{}, argc, argv);

    YR::CreateParam param;
    param.writeMode = YR::WriteMode::NONE_L2_CACHE_EVICT;
    param.consistencyType = YR::ConsistencyType::PRAM;

    std::string str = "success";
    YR::Buffer yrBuf(str.data(), str.length());

    auto ret = YR::PyFunction<YR::Buffer>("common", "echo")
                   .SetUrn("sn:cn:yrk:default:function:0-yr-stpython:$latest")
                   .Invoke(yrBuf);
    auto value = YR::Get(ret);
    std::string result = std::string(static_cast<const char*>(value->ImmutableData()), value->GetSize());
    std::cout << result << std::endl;

    YR::Finalize();
    return 0;
}
```
