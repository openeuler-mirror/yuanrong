#!/usr/bin/env bash
set -euo pipefail

TEST_PYPI_URL="${TEST_PYPI_REPOSITORY_URL:-https://test.pypi.org/legacy/}"
PUBLISH_TO_TEST_PYPI="${PUBLISH_TEST_PYPI:-}"

is_enabled() {
    case "$1" in
        1|true|TRUE|yes|YES|on|ON) return 0 ;;
        *) return 1 ;;
    esac
}

if [ -z "${PUBLISH_TO_TEST_PYPI}" ] && [ -n "${BUILDKITE_TAG:-}" ]; then
    PUBLISH_TO_TEST_PYPI=1
fi

if ! is_enabled "${PUBLISH_TO_TEST_PYPI}"; then
    printf 'PUBLISH_TEST_PYPI not enabled, skipping TestPyPI upload.\n' >&2
    exit 0
fi

if [ -z "${TEST_PYPI_API_TOKEN:-}" ]; then
    printf 'TEST_PYPI_API_TOKEN is required when TestPyPI publishing is enabled.\n' >&2
    exit 1
fi

if [ "$#" -eq 0 ]; then
    printf 'No wheel paths were requested for TestPyPI upload.\n' >&2
    exit 1
fi

files=()
for path in "$@"; do
    if [ -d "${path}" ]; then
        while IFS= read -r -d '' file; do
            files+=("${file}")
        done < <(find "${path}" -type f -name 'openyuanrong_sdk*.whl' -print0)
        continue
    fi
    for file in ${path}; do
        [ -f "${file}" ] || continue
        case "$(basename "${file}")" in
            openyuanrong_sdk*.whl) files+=("${file}") ;;
        esac
    done
done

if [ "${#files[@]}" -eq 0 ]; then
    printf 'No openyuanrong_sdk wheel files matched for TestPyPI upload.\n' >&2
    exit 1
fi

for file in "${files[@]}"; do
    case "$(basename "${file}")" in
        *+*.whl)
            printf 'ERROR: %s uses a local version; PyPI/TestPyPI reject local versions.\n' "${file}" >&2
            exit 1
            ;;
    esac
done

if ! python3 -m pip show twine >/dev/null 2>&1; then
    pip_install_args=(
        install
        -q
        --retries 2
        --timeout 60
        --index-url "${PIP_INDEX_URL:-https://mirrors.huaweicloud.com/repository/pypi/simple}"
        --trusted-host "${PIP_TRUSTED_HOST:-mirrors.huaweicloud.com}"
        twine
    )
    if python3 -m pip install --help 2>/dev/null | grep -q -- "--break-system-packages"; then
        pip_install_args+=(--break-system-packages)
    fi
    PIP_BREAK_SYSTEM_PACKAGES=1 python3 -m pip "${pip_install_args[@]}"
fi

attempt=1
until TWINE_USERNAME=__token__ TWINE_PASSWORD="${TEST_PYPI_API_TOKEN}" \
    python3 -m twine upload \
        --repository-url "${TEST_PYPI_URL}" \
        --skip-existing \
        "${files[@]}"; do
    if [ "${attempt}" -ge 3 ]; then
        printf 'ERROR: TestPyPI upload failed after %s attempts.\n' "${attempt}" >&2
        exit 1
    fi
    attempt=$((attempt + 1))
    printf 'WARNING: TestPyPI upload failed; retrying attempt %s/3.\n' "${attempt}" >&2
    sleep 15
done
