# YuanRong Development Guide

请使用第一性原理思考。你不能总是假设我非常清楚自己想要什么和该怎么得到。请保持审慎，从原始需求和问题出发，如果动机和目标不清晰，停下来和我讨论。如果目标清晰但是路径不是最短，告诉我，并且建议更好的办法



## 角色分工

- Claude 是**产品经理 (PM)**：负责定义需求、规划优先级、确保目标符合用户价值，跟踪进度，协调沟通，总结输出。
- Gemini 是**技术架构师 (Arch)**：设计系统架构，评估技术可行性，做出关键技术决策。
- Codex 是**开发工程师 (Dev)**：编写代码，实现功能，修复 bug。
- OpenCode 是**测试工程师 (QA)**：设计测试用例，执行测试，确保质量。

## Project Structure

- `functionsystem/` - Core function system (IAM, runtime, etc.)
- `api/` - API definitions (C++, Go, Python)
- `datasystem/` - Data storage system
- `frontend/` - Frontend components
- `example/` - Example configurations and scripts

## Build Workflow

### After Modifying `functionsystem/` Code

```bash
# 1. Build functionsystem (generates wheel package)
make functionsystem

# 2. Build yuanrong runtime (depends on functionsystem output)
make yuanrong
```

### After Modifying `frontend/` Code

```bash
# 1. Build functionsystem (generates wheel package)
make frontend

# 2. Build yuanrong runtime (depends on functionsystem output)
make yuanrong
```


### Quick Restart

Use the integrated restart script:

```bash
./example/restart.sh token
```

This script:
1. Stops existing runtime
2. Reinstalls Python packages
3. Starts runtime with IAM server enabled


### Testing

```bash
# Run all tests
bash build.sh -t

# Run tests with specific filter
bash build.sh -t -T "TestName*"

# Generate coverage report
bash build.sh -c

# Run with AddressSanitizer
bash build.sh -S address

# Run with ThreadSanitizer
bash build.sh -S thread

# System tests (requires deployment)
cd test/st
bash test.sh -l all  # Run all language tests
bash test.sh -l cpp  # C++ tests only
bash test.sh -l python  # Python tests only
bash test.sh -l java  # Java tests only
bash test.sh -l go  # Go tests only
```

### Direct Bazel Commands

```bash
# Build specific targets
bazel build //api/python:yr_python_pkg
bazel build //api/java:yr_java_pkg
bazel build //api/cpp:yr_cpp_pkg
bazel build //api/go:yr_go_pkg

# Run specific tests
bazel test //test/...
bazel test //api/python/yr/tests/...
bazel test //api/java:java_tests

```
