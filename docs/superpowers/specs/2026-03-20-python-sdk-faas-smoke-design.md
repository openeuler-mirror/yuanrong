# Python SDK And FaaS Smoke Design

**Goal**

Provide a minimal, reproducible single-node validation flow that runs inside the `dev` container and answers one question quickly: after changes land, do the basic Python SDK and Python FaaS paths still work?

**Scope**

- Single-node development environment only
- Run entirely inside the `dev` container
- Do not depend on Traefik, Casdoor, Keycloak, or frontend login
- Cover Python SDK basic invoke path and Python FaaS basic invoke path
- Emit clear PASS or FAIL output and enough diagnostics for first-pass debugging

**Out Of Scope**

- Auth flows
- Frontend UI
- Quota sync
- Multi-node deployment
- Performance or pressure testing
- Long-running resilience tests
- Cross-language SDK coverage

## Context

The repository already contains:

- Python SDK unit tests under `api/python/yr/tests/`
- Runtime bootstrap scripts such as `example/restart.sh`
- Minimal service configuration in `example/webterminal/services.yaml`
- Python function examples under `example/others/` and `example/webterminal/`

Those assets are useful references, but they are not currently packaged as a single minimal smoke flow for regression checking in the `dev` container.

## Recommended Approach

Use a single entry script to orchestrate deployment and validation, but keep deployment logic and test logic split into separate files.

This gives one command for humans and CI later, while keeping the code readable:

- deployment shell handles environment bootstrap and teardown
- SDK smoke script validates in-process remote invocation through the Python SDK
- FaaS smoke script validates deployed Python FaaS invocation through the runtime entry

## Alternatives Considered

### 1. Pure shell smoke only

Fast to write, but too brittle and hard to extend. Rejected.

### 2. Pytest-only integration suite

Good long-term shape, but heavier to bootstrap for the first useful regression check. Deferred.

### 3. Unified shell entry plus focused Python helpers

Recommended. One command to run, but logic stays readable and debuggable.

## Test Surface

### Python SDK Minimal Coverage

The SDK smoke must cover:

1. Runtime initialization succeeds
2. Stateless Python invocation succeeds
3. Stateful Python instance creation succeeds
4. Stateful method invocation succeeds
5. Stateful readback succeeds
6. One negative case fails explicitly instead of silently succeeding

### Python FaaS Minimal Coverage

The FaaS smoke must cover:

1. Python function package is present and valid
2. Function registration or deployment succeeds
3. Invocation with a simple string payload succeeds
4. Invocation with a JSON payload succeeds
5. Response body contains expected business fields
6. Repeated invocation succeeds

## Proposed File Layout

- `example/minimal-python/README.md`
  Explains prerequisites, the single command to run, and expected output
- `example/minimal-python/run_smoke.sh`
  Main entry script executed inside `dev`
- `example/minimal-python/services.yaml`
  Minimal service config for Python-only runtime validation
- `example/minimal-python/sdk_smoke.py`
  Python SDK smoke cases
- `example/minimal-python/faas/handler.py`
  Minimal Python FaaS handler
- `example/minimal-python/faas/function.json`
  FaaS metadata
- `example/minimal-python/faas_smoke.sh`
  FaaS deploy and invoke checks

## Deployment Script Design

The main script should follow this sequence:

1. Validate it is running inside the `dev` container
2. Stop any existing Yuanrong runtime
3. Install or refresh the locally built wheel from `output/`
4. Start single-node runtime without auth dependencies
5. Wait for health or readiness with bounded retries
6. Run SDK smoke
7. Run FaaS smoke
8. Print summary
9. On failure, print diagnostics and preserve logs

## Runtime Mode

Use the smallest local startup mode that does not require external auth components.

Recommended defaults:

- no Traefik
- no Casdoor
- no Keycloak
- local or direct runtime address inside `dev`
- reuse existing `yr start --master --enable_faas_frontend=true --enable_meta_service true --enable_iam_server true` pattern only if required by FaaS path

If `enable_iam_server` turns out to be unnecessary for the selected FaaS invocation path, prefer disabling it to reduce moving parts.

## SDK Smoke Design

The SDK smoke should define tiny in-file examples, not reuse large app examples.

Recommended cases:

- `add_one(x)` stateless function
  input `1`, expect `2`
- `Counter(start)` stateful class
  create with `10`
  call `add(5)`, expect `15`
  call `get()`, expect `15`
- negative validation
  pass unsupported argument shape or invalid invoke path and assert an exception is raised

The smoke must fail fast with a non-zero exit code if any assertion fails.

## FaaS Smoke Design

The minimal FaaS handler should:

- accept either plain text or JSON input
- return stable JSON so shell assertions stay simple

Recommended response shape:

```json
{
  "ok": true,
  "echo": "...",
  "mode": "text|json"
}
```

Recommended checks:

- invoke with `"hello"`
  expect `"ok": true`, `"echo": "hello"`, `"mode": "text"`
- invoke with `{"name":"yuanrong"}`
  expect `"ok": true`, `"echo": "yuanrong"`, `"mode": "json"`
- invoke twice to ensure the second call still succeeds

## Diagnostics Requirements

When the smoke fails, output should include:

- failing stage name
- exact command that failed
- runtime startup log path if available
- relevant function invoke response
- current process or port snapshot if lightweight to collect

The main script should avoid silent retries and should show bounded progress output.

## Acceptance Criteria

The design is complete when:

1. One command inside `dev` runs the whole flow
2. No auth components are required
3. SDK smoke and FaaS smoke both run in the same environment
4. PASS or FAIL is obvious from exit code and summary output
5. Failure output is good enough for a developer to triage quickly

## Implementation Notes

- Prefer existing runtime bootstrapping patterns from `example/restart.sh`, but strip auth-specific behavior
- Prefer small, direct assertions over a large test framework for the first version
- Keep the handler and service config intentionally tiny so future breakage is easy to localize
- Design the entry script so it can later be called from CI without large changes
