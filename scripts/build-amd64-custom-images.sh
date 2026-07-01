#!/bin/bash
# 构建自定义 amd64 镜像 (使用 docker buildx)
# 用法: ./scripts/build-amd64-custom-images.sh
#
# 在 macOS arm64 上使用 buildx 构建 linux/amd64 架构的自定义镜像
# 需要先拉取基础镜像（运行 pull-amd64-images.sh）

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TARGET_PLATFORM="linux/amd64"
ARCH_SUFFIX="amd64"

cd "$PROJECT_ROOT"

print_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
print_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
print_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 检查 buildx
check_buildx() {
    if ! docker buildx version &>/dev/null; then
        print_error "Docker buildx 不可用"
        print_info "请安装 Docker Desktop 或启用 buildx 插件"
        return 1
    fi
    print_ok "Docker buildx 可用"
    return 0
}

# 确保 builder 存在
ensure_builder() {
    if docker buildx inspect xinwiki-builder &>/dev/null 2>&1; then
        print_info "使用已有的 buildx builder: xinwiki-builder"
    else
        print_info "创建 buildx builder: xinwiki-builder ..."
        docker buildx create --name xinwiki-builder --driver docker-container --use 2>&1
        docker buildx inspect --bootstrap 2>&1 | tail -3
        print_ok "buildx builder 创建成功"
    fi
    docker buildx use xinwiki-builder
}

# 获取版本信息
get_version_info() {
    if [ -f "VERSION" ]; then
        VERSION=$(cat VERSION | tr -d '\n\r')
    else
        VERSION="unknown"
    fi
    if command -v git >/dev/null 2>&1; then
        COMMIT_ID=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    else
        COMMIT_ID="unknown"
    fi
    BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M:%S UTC')
    if command -v go >/dev/null 2>&1; then
        GO_VERSION=$(go version 2>/dev/null || echo "unknown")
    else
        GO_VERSION="unknown"
    fi
}

# ========== 开始构建 ==========
print_info "========== 构建自定义 $TARGET_PLATFORM 镜像 =========="

check_buildx
ensure_builder
get_version_info

SUCCESS_IMAGES=()
FAILED_IMAGES=()

# 1. 构建前端镜像
print_info ""
print_info "[1/4] 构建 frontend 镜像 ($TARGET_PLATFORM)..."
if [ -d "frontend/dist" ]; then
    if docker buildx build --platform "$TARGET_PLATFORM" --load \
        -t "xinwiki/xinwiki-ui:latest-$ARCH_SUFFIX" \
        -f frontend/Dockerfile frontend/ 2>&1 | tail -10; then
        print_ok "✓ xinwiki/xinwiki-ui:latest-$ARCH_SUFFIX 构建成功"
        SUCCESS_IMAGES+=("xinwiki/xinwiki-ui:latest-$ARCH_SUFFIX")
    else
        print_error "✗ frontend 镜像构建失败"
        FAILED_IMAGES+=("xinwiki/xinwiki-ui")
    fi
else
    print_warn "  frontend/dist 不存在，跳过前端镜像构建"
    print_info "  请先运行: ./scripts/build_frontend_dist.sh"
fi

# 2. 构建 app 镜像
print_info ""
print_info "[2/4] 构建 app 镜像 ($TARGET_PLATFORM)..."
if docker buildx build --platform "$TARGET_PLATFORM" --load \
    --build-arg GOPRIVATE_ARG="${GOPRIVATE:-}" \
    --build-arg GOPROXY_ARG="${GOPROXY:-https://goproxy.cn,direct}" \
    --build-arg GOSUMDB_ARG="${GOSUMDB:-off}" \
    --build-arg VERSION_ARG="$VERSION" \
    --build-arg COMMIT_ID_ARG="$COMMIT_ID" \
    --build-arg BUILD_TIME_ARG="$BUILD_TIME" \
    --build-arg GO_VERSION_ARG="$GO_VERSION" \
    -t "xinwiki/xinwiki-app:latest-$ARCH_SUFFIX" \
    -f docker/Dockerfile.app . 2>&1 | tail -10; then
    print_ok "✓ xinwiki/xinwiki-app:latest-$ARCH_SUFFIX 构建成功"
    SUCCESS_IMAGES+=("xinwiki/xinwiki-app:latest-$ARCH_SUFFIX")
else
    print_error "✗ app 镜像构建失败"
    FAILED_IMAGES+=("xinwiki/xinwiki-app")
fi

# 3. 构建 docreader 镜像
print_info ""
print_info "[3/4] 构建 docreader 镜像 ($TARGET_PLATFORM)..."
if docker buildx build --platform "$TARGET_PLATFORM" --load \
    --build-arg APT_MIRROR="${APT_MIRROR:-}" \
    -t "xinwiki/xinwiki-docreader:latest-$ARCH_SUFFIX" \
    -f docker/Dockerfile.docreader docreader/ 2>&1 | tail -10; then
    print_ok "✓ xinwiki/xinwiki-docreader:latest-$ARCH_SUFFIX 构建成功"
    SUCCESS_IMAGES+=("xinwiki/xinwiki-docreader:latest-$ARCH_SUFFIX")
else
    print_error "✗ docreader 镜像构建失败"
    FAILED_IMAGES+=("xinwiki/xinwiki-docreader")
fi

# 4. 构建 sandbox 镜像
print_info ""
print_info "[4/4] 构建 sandbox 镜像 ($TARGET_PLATFORM)..."
if [ -f docker/Dockerfile.sandbox ]; then
    if docker buildx build --platform "$TARGET_PLATFORM" --load \
        -t "xinwiki/xinwiki-sandbox:latest-$ARCH_SUFFIX" \
        -f docker/Dockerfile.sandbox . 2>&1 | tail -10; then
        print_ok "✓ xinwiki/xinwiki-sandbox:latest-$ARCH_SUFFIX 构建成功"
        SUCCESS_IMAGES+=("xinwiki/xinwiki-sandbox:latest-$ARCH_SUFFIX")
    else
        print_error "✗ sandbox 镜像构建失败"
        FAILED_IMAGES+=("xinwiki/xinwiki-sandbox")
    fi
else
    print_warn "  Dockerfile.sandbox 不存在，跳过"
fi

# ========== 结果汇总 ==========
print_info ""
print_info "==================== 构建结果 ===================="
print_ok "成功: ${#SUCCESS_IMAGES[@]}/4"

if [ ${#FAILED_IMAGES[@]} -gt 0 ]; then
    print_warn "失败: ${FAILED_IMAGES[*]}"
fi

echo ""
print_info "已成功构建的镜像:"
for img in "${SUCCESS_IMAGES[@]}"; do
    # 用 docker run 验证架构
    arch=$(docker run --rm --platform "$TARGET_PLATFORM" "$img" uname -m 2>/dev/null || echo "unknown")
    size=$(docker image inspect "$img" --format='{{.Size}}' 2>/dev/null || echo "0")
    size_mb=$((size / 1024 / 1024))
    echo "  - $img ($arch, ${size_mb}MB)"
done

echo ""
if [ ${#SUCCESS_IMAGES[@]} -gt 0 ]; then
    print_ok "自定义镜像构建完成！"
    print_info "下一步: 运行 ./scripts/package-amd64-images.sh 打包所有镜像"
else
    print_error "没有成功构建任何镜像"
fi
