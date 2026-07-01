#!/bin/bash
# 拉取 amd64 架构的基础镜像（带自动重试）
# 用法: ./scripts/pull-amd64-images.sh
#
# 在 macOS arm64 上拉取 linux/amd64 镜像，使用 -amd64 后缀 tag 保存
# 注意：会先删除同名的本地镜像（不管架构），确保拉取到正确的 amd64 版本

set +e
# set -e 会导致 docker pull 失败时直接退出，我们自己处理重试逻辑

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TARGET_PLATFORM="linux/amd64"
ARCH_SUFFIX="amd64"
MAX_RETRIES=3

cd "$PROJECT_ROOT"

print_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
print_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
print_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 镜像列表（按从小到大排列，先拉小的）
IMAGE_ORDER=(
    "debian:12.12-slim"
    "redis:7.0-alpine"
    "nginx:stable-alpine"
    "python:3.11-slim"
    "node:20-slim"
    "python:3.10.18-bookworm"
    "golang:1.26-bookworm"
    "paradedb/paradedb:v0.22.2-pg17"
)

SUCCESS_IMAGES=()
FAILED_IMAGES=()

pull_amd64_image() {
    local original_img="$1"
    local target_tag="${original_img}-${ARCH_SUFFIX}"
    local attempt=1
    
    while [ $attempt -le $MAX_RETRIES ]; do
        print_info "[$attempt/$MAX_RETRIES] 拉取 $original_img ($TARGET_PLATFORM) ..."
        
        # 先删除本地同名镜像（避免 "up to date" 问题）
        docker rmi "$original_img" 2>/dev/null || true
        
        if docker pull --platform "$TARGET_PLATFORM" "$original_img"; then
            # 验证架构（docker image inspect 的 Architecture 字段在 Docker Desktop 上可能为空）
            # 用 docker run 实际运行一下来验证
            local arch
            arch=$(docker run --rm --platform "$TARGET_PLATFORM" "$original_img" uname -m 2>/dev/null || echo "unknown")
            
            # x86_64 = amd64
            if [ "$arch" != "x86_64" ] && [ "$arch" != "amd64" ]; then
                print_warn "  拉取后运行架构是 $arch，不是预期的 amd64/x86_64，重试..."
                sleep 5
                attempt=$((attempt + 1))
                continue
            fi
            
            # 重新打标签为带架构后缀的名字
            docker tag "$original_img" "$target_tag"
            
            local size
            size=$(docker image inspect "$target_tag" --format='{{.Size}}' 2>/dev/null || echo "0")
            local size_mb=$((size / 1024 / 1024))
            
            print_ok "✓ $original_img -> $target_tag ($arch, ${size_mb}MB)"
            return 0
        else
            print_warn "  第 $attempt 次拉取失败，等待 5 秒后重试..."
            sleep 5
            attempt=$((attempt + 1))
        fi
    done
    
    print_error "✗ $original_img 拉取失败（已重试 $MAX_RETRIES 次）"
    return 1
}

print_info "开始拉取 $TARGET_PLATFORM 基础镜像（共 ${#IMAGE_ORDER[@]} 个）"
print_info "使用 -$ARCH_SUFFIX 后缀 tag 保存，避免与本地 arm64 镜像冲突"
print_info "注意：会先删除同名的本地镜像，确保拉取到正确架构"
echo ""

for img in "${IMAGE_ORDER[@]}"; do
    if pull_amd64_image "$img"; then
        SUCCESS_IMAGES+=("${img}-${ARCH_SUFFIX}")
    else
        FAILED_IMAGES+=("$img")
    fi
    echo ""
done

print_info "==================== 结果 ===================="
print_ok "成功: ${#SUCCESS_IMAGES[@]}/${#IMAGE_ORDER[@]}"

if [ ${#FAILED_IMAGES[@]} -gt 0 ]; then
    print_warn "失败: ${FAILED_IMAGES[*]}"
fi

echo ""
print_info "已成功拉取的 amd64 镜像:"
for img in "${SUCCESS_IMAGES[@]}"; do
    # 用 docker run 验证架构
    arch=$(docker run --rm --platform "$TARGET_PLATFORM" "$img" uname -m 2>/dev/null || echo "unknown")
    size=$(docker image inspect "$img" --format='{{.Size}}' 2>/dev/null || echo "0")
    size_mb=$((size / 1024 / 1024))
    echo "  - $img ($arch, ${size_mb}MB)"
done

echo ""
if [ ${#SUCCESS_IMAGES[@]} -eq ${#IMAGE_ORDER[@]} ]; then
    print_ok "所有基础镜像拉取完成！"
    print_info "下一步（可选）: 运行 ./scripts/build-amd64-custom-images.sh 构建自定义镜像"
    print_info "下一步（必选）: 运行 ./scripts/package-amd64-images.sh 打包所有镜像"
else
    print_warn "部分镜像拉取失败，请检查网络后重新运行本脚本"
fi
