#!/bin/bash
# Copyright (c) Huawei Technologies Co., Ltd. 2025. All rights reserved.
#
# Buildkite Agent 快速设置脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 颜色
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

print_usage() {
    cat << EOF
使用方法: bash $0 [command]

命令:
  setup   - 初始化 Buildkite Agent 配置
  start   - 启动 Buildkite Agent
  stop    - 停止 Buildkite Agent
  restart - 重启 Buildkite Agent
  logs    - 查看日志
  status  - 查看状态
  test    - 测试连接

环境变量:
  BUILDKITE_AGENT_TOKEN  - Agent Token (必需)
  BUILDKITE_AGENT_NAME   - Agent 名称 (默认: macos-arm64-builder)

获取 Agent Token:
  1. 访问 https://buildkite.com/organizations/<org>/agents
  2. 点击 "New Agent"
  3. 复制 Agent Token

示例:
  export BUILDKITE_AGENT_TOKEN=xxx
  $0 setup
  $0 start
EOF
}

# 检查 Docker
check_docker() {
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装"
        exit 1
    fi
    docker info &> /dev/null || { log_error "Docker 未运行"; exit 1; }
}

# 初始化配置
cmd_setup() {
    log_info "=== Buildkite Agent 初始化 ==="

    if [ -z "$BUILDKITE_AGENT_TOKEN" ]; then
        log_error "BUILDKITE_AGENT_TOKEN 未设置"
        echo ""
        echo "请先设置环境变量:"
        echo "  export BUILDKITE_AGENT_TOKEN=your-token-here"
        echo ""
        echo "获取 Token: https://buildkite.com/organizations/<org>/agents"
        exit 1
    fi

    check_docker

    # 创建 .env 文件
    cat > "${SCRIPT_DIR}/.buildkite-agent.env" << EOF
# Buildkite Agent 配置
BUILDKITE_AGENT_TOKEN=${BUILDKITE_AGENT_TOKEN}
BUILDKITE_AGENT_NAME=${BUILDKITE_AGENT_NAME:-macos-arm64-builder}
EOF

    log_info "配置文件已创建: ${SCRIPT_DIR}/.buildkite-agent.env"
    log_info "下一步: $0 start"
}

# 启动 Agent
cmd_start() {
    log_info "启动 Buildkite Agent..."

    check_docker

    # 加载环境变量
    if [ -f "${SCRIPT_DIR}/.buildkite-agent.env" ]; then
        set -a
        source "${SCRIPT_DIR}/.buildkite-agent.env"
        set +a
    fi

    if [ -z "$BUILDKITE_AGENT_TOKEN" ]; then
        log_error "BUILDKITE_AGENT_TOKEN 未设置，请先运行: $0 setup"
        exit 1
    fi

    cd "${SCRIPT_DIR}"
    docker-compose -f docker-compose.buildkite-agent.yml up -d

    log_info "Buildkite Agent 已启动"
    sleep 3
    cmd_status
}

# 停止 Agent
cmd_stop() {
    log_info "停止 Buildkite Agent..."

    cd "${SCRIPT_DIR}"
    docker-compose -f docker-compose.buildkite-agent.yml down

    log_info "Buildkite Agent 已停止"
}

# 重启 Agent
cmd_restart() {
    cmd_stop
    sleep 2
    cmd_start
}

# 查看日志
cmd_logs() {
    cd "${SCRIPT_DIR}"
    docker-compose -f docker-compose.buildkite-agent.yml logs -f
}

# 查看状态
cmd_status() {
    cd "${SCRIPT_DIR}"
    docker-compose -f docker-compose.buildkite-agent.yml ps
}

# 测试连接
cmd_test() {
    log_info "测试 Buildkite Agent 连接..."

    check_docker

    cd "${SCRIPT_DIR}"
    docker-compose -f docker-compose.buildkite-agent.yml exec buildkite-agent \
        buildkite-agent heartbeat

    log_info "连接测试完成"
}

# 主程序
main() {
    local command="${1:-setup}"

    case "${command}" in
        setup)   cmd_setup ;;
        start)   cmd_start ;;
        stop)    cmd_stop ;;
        restart) cmd_restart ;;
        logs)    cmd_logs ;;
        status)  cmd_status ;;
        test)    cmd_test ;;
        help|-h|--help)
            print_usage
            ;;
        *)
            log_error "未知命令: ${command}"
            echo ""
            print_usage
            exit 1
            ;;
    esac
}

main "$@"
