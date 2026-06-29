#!/bin/bash
# XinWiki Docker Compose 快速启动脚本
# 用法: ./scripts/quick-start.sh [命令]
#
# 命令:
#   start     - 启动核心服务（默认）
#   stop      - 停止所有服务
#   restart   - 重启所有服务
#   status    - 查看服务状态
#   logs      - 查看实时日志
#   full      - 启动全部服务（含 MinIO、Neo4j、Langfuse）
#   clean     - 停止服务并删除数据卷（⚠️ 危险操作）
#   build     - 重新构建镜像并启动
#   pull      - 拉取最新镜像并启动

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目根目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

# 打印函数
print_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
print_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
print_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 检查 Docker 是否安装
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_error "Docker 未安装，请先安装 Docker"
        print_info "安装指南: https://docs.docker.com/get-docker/"
        exit 1
    fi
    if ! docker compose version &> /dev/null; then
        print_error "Docker Compose 未安装，请先安装 Docker Compose v2+"
        print_info "安装指南: https://docs.docker.com/compose/install/"
        exit 1
    fi
    print_ok "Docker 环境检查通过"
}

# 检查并创建 .env 文件
check_env() {
    if [ ! -f ".env" ]; then
        if [ -f ".env.example" ]; then
            print_warn ".env 文件不存在，从 .env.example 复制"
            cp .env.example .env
            print_warn "请编辑 .env 文件配置 LLM API Key 后重新运行"
            print_info "至少需要配置以下任一 LLM 提供商:"
            print_info "  LLM_API_KEY + LLM_BASE_URL + LLM_MODEL_NAME"
            print_info "  或参考 docs/DEPLOYMENT.md 获取详细配置说明"
            exit 1
        else
            print_error ".env 和 .env.example 都不存在"
            exit 1
        fi
    fi
    print_ok ".env 配置文件已存在"
}

# 检查关键配置
check_config() {
    local has_llm=false

    # 检查是否配置了 LLM API Key（支持多种变量名）
    if grep -qE '^\s*(LLM_API_KEY|EMBEDDING_API_KEY|OPENAI_API_KEY)=.+' .env 2>/dev/null; then
        has_llm=true
    fi

    # 检查 builtin_models.yaml
    if [ -f "config/builtin_models.yaml" ]; then
        has_llm=true
    fi

    if [ "$has_llm" = false ]; then
        print_warn "未检测到 LLM API Key 配置，启动后请在「设置 → 模型管理」中配置"
        print_warn "或在 .env 文件中设置 LLM_API_KEY 等变量"
    fi
}

# 启动核心服务
start_core() {
    print_info "启动核心服务（前端 + 后端 + PostgreSQL + Redis）..."
    docker compose up -d
    wait_for_services
    print_ok "核心服务已启动"
    show_urls
}

# 启动全部服务
start_full() {
    print_info "启动全部服务（含 MinIO、Neo4j、Langfuse）..."
    docker compose --profile full up -d
    wait_for_services
    print_ok "全部服务已启动"
    show_urls
}

# 等待服务就绪
wait_for_services() {
    print_info "等待服务就绪（约 30-60 秒）..."

    # 等待后端健康检查
    local max_wait=120
    local waited=0
    while [ $waited -lt $max_wait ]; do
        if curl -sf http://localhost:8080/health &> /dev/null; then
            print_ok "后端服务已就绪"
            return 0
        fi
        sleep 3
        waited=$((waited + 3))
        printf "."
    done
    echo ""
    print_warn "后端服务未在 ${max_wait}s 内就绪，请检查日志: docker compose logs app"
}

# 显示服务地址
show_urls() {
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  XinWiki 已启动！${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo -e "  ${BLUE}Web UI${NC}:        http://localhost"
    echo -e "  ${BLUE}API${NC}:           http://localhost:8080"
    echo -e "  ${BLUE}API 文档${NC}:      http://localhost:8080/swagger/index.html"
    echo ""
    echo -e "  ${YELLOW}默认账号${NC}: a****@*********** / admin123"
    echo -e "  ${YELLOW}⚠️ 首次登录后请立即修改密码${NC}"
    echo ""
    echo -e "  常用命令:"
    echo -e "    查看日志:   ./scripts/quick-start.sh logs"
    echo -e "    查看状态:   ./scripts/quick-start.sh status"
    echo -e "    停止服务:   ./scripts/quick-start.sh stop"
    echo -e "    重启服务:   ./scripts/quick-start.sh restart"
    echo ""
}

# 停止服务
stop_services() {
    print_info "停止所有服务..."
    docker compose down
    print_ok "所有服务已停止"
}

# 重启服务
restart_services() {
    print_info "重启所有服务..."
    docker compose restart
    print_ok "所有服务已重启"
}

# 查看状态
show_status() {
    echo -e "${BLUE}=== XinWiki 服务状态 ===${NC}"
    docker compose ps
    echo ""
    echo -e "${BLUE}=== 资源使用 ===${NC}"
    docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" \
        $(docker compose ps -q) 2>/dev/null || echo "（无运行中的容器）"
}

# 查看日志
show_logs() {
    local service="${2:-}"
    if [ -n "$service" ]; then
        print_info "查看 $service 日志（Ctrl+C 退出）..."
        docker compose logs -f "$service"
    else
        print_info "查看所有服务日志（Ctrl+C 退出）..."
        docker compose logs -f
    fi
}

# 构建并启动
build_and_start() {
    print_info "重新构建镜像并启动..."
    docker compose up -d --build
    wait_for_services
    print_ok "构建并启动完成"
    show_urls
}

# 拉取镜像并启动
pull_and_start() {
    print_info "拉取最新镜像..."
    docker compose pull
    print_info "启动服务..."
    docker compose up -d
    wait_for_services
    print_ok "拉取并启动完成"
    show_urls
}

# 清理（危险操作）
clean_all() {
    echo -e "${RED}========================================${NC}"
    echo -e "${RED}  ⚠️  危险操作：将删除所有数据！${NC}"
    echo -e "${RED}========================================${NC}"
    echo ""
    read -p "确定要删除所有数据并停止服务吗？(输入 YES 确认): " confirm
    if [ "$confirm" = "YES" ]; then
        print_warn "停止服务并删除数据卷..."
        docker compose down -v
        print_ok "所有服务和数据已清除"
    else
        print_info "已取消"
    fi
}

# 主逻辑
main() {
    local cmd="${1:-start}"

    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}  XinWiki 快速启动脚本${NC}"
    echo -e "${BLUE}========================================${NC}"

    check_docker

    case "$cmd" in
        start)
            check_env
            check_config
            start_core
            ;;
        full)
            check_env
            check_config
            start_full
            ;;
        stop)
            stop_services
            ;;
        restart)
            restart_services
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs "$@"
            ;;
        build)
            check_env
            build_and_start
            ;;
        pull)
            check_env
            pull_and_start
            ;;
        clean)
            clean_all
            ;;
        *)
            echo "用法: $0 {start|stop|restart|status|logs|full|build|pull|clean}"
            echo ""
            echo "命令说明:"
            echo "  start    - 启动核心服务（默认）"
            echo "  full     - 启动全部服务（含 MinIO、Neo4j、Langfuse）"
            echo "  stop     - 停止所有服务"
            echo "  restart  - 重启所有服务"
            echo "  status   - 查看服务状态"
            echo "  logs     - 查看实时日志（可指定服务名: logs app）"
            echo "  build    - 重新构建镜像并启动"
            echo "  pull     - 拉取最新镜像并启动"
            echo "  clean    - 停止服务并删除所有数据（⚠️ 危险）"
            exit 1
            ;;
    esac
}

main "$@"
