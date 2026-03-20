# YuanRong Development Guide

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
