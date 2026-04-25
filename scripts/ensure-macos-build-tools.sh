#!/bin/bash
# Copyright (c) Huawei Technologies Co., Ltd. 2025. All rights reserved.
#
# Install only the missing software prerequisites needed to build the macOS SDK
# with:
#   bash build.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_step() { echo -e "${BLUE}[STEP]${NC} $*"; }

check_macos() {
    if [[ "$(uname)" != "Darwin" ]]; then
        log_error "This script only supports macOS"
        exit 1
    fi
}

check_xcode_cli() {
    if ! xcode-select -p >/dev/null 2>&1; then
        log_error "Xcode Command Line Tools are required"
        echo "Run: xcode-select --install"
        exit 1
    fi
}

check_brew() {
    if ! command -v brew >/dev/null 2>&1; then
        log_error "Homebrew is required"
        echo 'Run: /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"'
        exit 1
    fi
}

brew_install_if_missing() {
    local formula="$1"
    local check_cmd="$2"
    if command -v "${check_cmd}" >/dev/null 2>&1; then
        log_info "${check_cmd} already available"
        return
    fi
    brew_install "${formula}"
}

brew_install() {
    local formula="$1"
    if brew list --versions "$formula" >/dev/null 2>&1; then
        log_info "$formula already installed"
        return
    fi
    brew install "$formula"
}

python_formula_bin() {
    local formula="$1"
    if ! brew list --versions "$formula" >/dev/null 2>&1; then
        return 1
    fi
    local prefix
    prefix="$(brew --prefix "$formula")"
    case "$formula" in
        python@3.11) echo "${prefix}/bin/python3.11" ;;
        python@3.10) echo "${prefix}/bin/python3.10" ;;
        python@3.9) echo "${prefix}/bin/python3.9" ;;
        *) return 1 ;;
    esac
}

ensure_bazel() {
    if command -v bazel >/dev/null 2>&1; then
        log_info "bazel already available"
        return
    fi
    if command -v bazelisk >/dev/null 2>&1; then
        local brew_bin
        brew_bin="$(brew --prefix)/bin"
        ln -sf "$(command -v bazelisk)" "${brew_bin}/bazel"
        log_info "Linked bazel -> bazelisk"
        return
    fi
    brew_install bazelisk
    ensure_bazel
}

pick_python() {
    local py
    local candidates=()

    for py in python3.11 python3.10 python3.9 python3; do
        if command -v "${py}" >/dev/null 2>&1; then
            candidates+=("$(command -v "${py}")")
        fi
    done

    for py in python@3.11 python@3.10 python@3.9; do
        if py_path="$(python_formula_bin "${py}" 2>/dev/null)"; then
            candidates+=("${py_path}")
        fi
    done

    for py in "${candidates[@]}"; do
        if [[ -x "${py}" ]] && "${py}" - <<'PY' >/dev/null 2>&1
import sys
raise SystemExit(0 if sys.version_info >= (3, 9) else 1)
PY
        then
            echo "${py}"
            return 0
        fi
    done

    return 1
}

pip_flags_for_python() {
    local python_bin="$1"
    if "${python_bin}" -m pip install --help 2>/dev/null | grep -q -- '--break-system-packages'; then
        echo "--break-system-packages"
    fi
}

ensure_python() {
    if pick_python >/dev/null 2>&1; then
        log_info "python3.9+ already available"
        return
    fi
    brew_install python@3.11
    brew link --overwrite --force python@3.11 >/dev/null 2>&1 || true

    if ! pick_python >/dev/null 2>&1; then
        log_error "python3.9+ is still unavailable after installing python@3.11"
        exit 1
    fi
}

ensure_python_packages() {
    local python_bin
    python_bin="$(pick_python)"
    local pip_flag
    pip_flag="$(pip_flags_for_python "${python_bin}")"
    local missing
    missing="$("${python_bin}" - <<'PY'
import importlib.util
required = ["packaging", "wheel"]
missing = [name for name in required if importlib.util.find_spec(name) is None]
print(" ".join(missing))
PY
)"
    if [[ -z "${missing}" ]]; then
        log_info "python packages already available: packaging wheel"
        return
    fi
    "${python_bin}" -m pip install ${pip_flag:+$pip_flag} --upgrade ${missing}
}

main() {
    check_macos
    check_xcode_cli
    check_brew

    log_step "Checking minimal macOS SDK build prerequisites"
    if [[ "${SKIP_BREW_UPDATE:-0}" != "1" ]]; then
        brew update
    fi

    brew_install_if_missing wget wget
    ensure_python
    brew_install_if_missing go go
    ensure_bazel

    ensure_python_packages

    log_info "macOS SDK build prerequisites are ready"
}

main "$@"
