#!/bin/bash
# 在 Linux amd64 上拉取并打包 XinWiki 离线镜像
# 用法: ./scripts/package-offline-amd64.sh
#
# 注意：此脚本设计为在 Linux amd64 机器上直接运行，不需要 --platform
# 如果在 macOS arm64 上运行，请使用 pull-amd64-images.sh + build-amd64-custom-images.sh

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUTPUT_DIR="$PROJECT_ROOT/docker/images"
ARCH="amd64"

cd "$PROJECT_ROOT"

print_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
print_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
print_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

mkdir -p "$OUTPUT_DIR"

# ========== 镜像列表 ==========

# 基础设施镜像（运行时需要）
INFRA_IMAGES=(
    "paradedb/paradedb:v0.22.2-pg17"
    "redis:7.0-alpine"
)

# 构建基础镜像（构建自定义镜像时需要，运行时不需要）
BUILD_BASE_IMAGES=(
    "golang:1.26-bookworm"
    "python:3.10.18-bookworm"
    "python:3.11-slim"
    "node:20-slim"
    "debian:12.12-slim"
    "nginx:stable-alpine"
    "ghcr.io/astral-sh/uv:0.6.6"
)

ALL_BASE_IMAGES=("${INFRA_IMAGES[@]}" "${BUILD_BASE_IMAGES[@]}")

# ========== 1. 拉取镜像 ==========
pull_images() {
    print_info "========== 1/4 拉取基础镜像 =========="
    
    local failed=()
    local success=()
    
    for img in "${ALL_BASE_IMAGES[@]}"; do
        print_info "拉取 $img ..."
        if docker pull "$img"; then
            print_ok "✓ $img"
            success+=("$img")
        else
            print_error "✗ $img 拉取失败"
            failed+=("$img")
        fi
        echo ""
    done
    
    print_info "拉取完成: ${#success[@]}/${#ALL_BASE_IMAGES[@]}"
    if [ ${#failed[@]} -gt 0 ]; then
        print_warn "失败: ${failed[*]}"
        return 1
    fi
    return 0
}

# ========== 2. 验证镜像完整性 ==========
verify_images() {
    print_info "========== 2/4 验证镜像完整性 =========="
    
    local failed=()
    
    for img in "${ALL_BASE_IMAGES[@]}"; do
        echo -n "  验证 $img ... "
        if docker save "$img" -o /dev/null 2>/dev/null; then
            echo "OK"
        else
            echo "FAILED"
            failed+=("$img")
        fi
    done
    
    if [ ${#failed[@]} -gt 0 ]; then
        print_error "以下镜像损坏，将尝试重新拉取: ${failed[*]}"
        print_info "删除损坏的镜像..."
        for img in "${failed[@]}"; do
            docker rmi -f "$img" 2>/dev/null || true
        done
        print_info "重新拉取..."
        for img in "${failed[@]}"; do
            docker pull "$img" || {
                print_error "$img 重新拉取仍然失败"
                return 1
            }
        done
        print_ok "重新拉取完成，再次验证..."
        # 再次验证
        for img in "${failed[@]}"; do
            if ! docker save "$img" -o /dev/null 2>/dev/null; then
                print_error "$img 仍然损坏"
                return 1
            fi
        done
    fi
    
    print_ok "所有镜像验证通过"
    return 0
}

# ========== 3. 打包镜像 ==========
package_images() {
    print_info "========== 3/4 打包镜像 =========="
    
    # 打包基础设施镜像
    local infra_tar="$OUTPUT_DIR/xinwiki-infra-$ARCH.tar"
    print_info "打包基础设施镜像 (${#INFRA_IMAGES[@]} 个) -> $infra_tar"
    docker save -o "$infra_tar" "${INFRA_IMAGES[@]}"
    print_ok "完成 ($(du -h "$infra_tar" | cut -f1))"
    
    # 打包构建基础镜像
    local build_tar="$OUTPUT_DIR/xinwiki-build-base-$ARCH.tar"
    print_info "打包构建基础镜像 (${#BUILD_BASE_IMAGES[@]} 个) -> $build_tar"
    docker save -o "$build_tar" "${BUILD_BASE_IMAGES[@]}"
    print_ok "完成 ($(du -h "$build_tar" | cut -f1))"
    
    # 压缩
    print_info "压缩镜像包..."
    gzip -f "$infra_tar"
    gzip -f "$build_tar"
    
    # 生成 SHA256
    sha256sum "$infra_tar.gz" > "$infra_tar.gz.sha256"
    sha256sum "$build_tar.gz" > "$build_tar.gz.sha256"
    
    print_ok "压缩完成"
    return 0
}

# ========== 4. 生成加载脚本和清单 ==========
generate_artifacts() {
    print_info "========== 4/4 生成加载脚本和清单 =========="
    
    # 生成加载脚本
    local load_script="$OUTPUT_DIR/load-images-$ARCH.sh"
    cat > "$load_script" << 'LOADEOF'
#!/bin/bash
# XinWiki 离线镜像加载脚本 (amd64 版本)
# 用法: ./load-images-amd64.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "========================================="
echo "  XinWiki 离线镜像加载 (amd64)"
echo "========================================="
echo ""

# 加载基础设施镜像
INFRA_FILE="$SCRIPT_DIR/xinwiki-infra-amd64.tar.gz"
if [ -f "$INFRA_FILE" ]; then
    echo "加载基础设施镜像..."
    if [ -f "$INFRA_FILE.sha256" ]; then
        (cd "$SCRIPT_DIR" && sha256sum -c xinwiki-infra-amd64.tar.gz.sha256) || {
            echo "警告: 基础设施镜像校验失败"
            read -p "继续加载？(y/N): " c
            [ "$c" != "y" ] && exit 1
        }
    fi
    gunzip -k -f "$INFRA_FILE"
    docker load -i "${INFRA_FILE%.gz}"
    rm -f "${INFRA_FILE%.gz}"
    echo "基础设施镜像加载完成"
else
    echo "跳过: 基础设施镜像文件不存在 ($INFRA_FILE)"
fi

echo ""

# 加载构建基础镜像
BUILD_FILE="$SCRIPT_DIR/xinwiki-build-base-amd64.tar.gz"
if [ -f "$BUILD_FILE" ]; then
    echo "加载构建基础镜像..."
    if [ -f "$BUILD_FILE.sha256" ]; then
        (cd "$SCRIPT_DIR" && sha256sum -c xinwiki-build-base-amd64.tar.gz.sha256) || {
            echo "警告: 构建基础镜像校验失败"
            read -p "继续加载？(y/N): " c
            [ "$c" != "y" ] && exit 1
        }
    fi
    gunzip -k -f "$BUILD_FILE"
    docker load -i "${BUILD_FILE%.gz}"
    rm -f "${BUILD_FILE%.gz}"
    echo "构建基础镜像加载完成"
else
    echo "跳过: 构建基础镜像文件不存在 ($BUILD_FILE)"
fi

echo ""
echo "========================================="
echo "  镜像加载完成！"
echo "========================================="
echo ""
echo "已加载的镜像:"
docker images --format "table {{.Repository}}:{{.Tag}}\t{{.Size}}" | grep -E "(paradedb|redis|golang|python|node|debian|nginx|uv)"
echo ""
echo "下一步:"
echo "  1. 回到项目根目录: cd ../.."
echo "  2. 复制配置: cp .env.example .env"
echo "  3. 修改配置: vim .env"
echo "  4. 启动服务: docker compose up -d"
echo ""
LOADEOF
    
    chmod +x "$load_script"
    print_ok "加载脚本: $load_script"
    
    # 生成清单
    local manifest="$OUTPUT_DIR/image-manifest-$ARCH.md"
    cat > "$manifest" << MANIFESTEOF
# XinWiki 离线部署镜像清单 ($ARCH)

生成时间: $(date '+%Y-%m-%d %H:%M:%S')
目标平台: linux/$ARCH

## 镜像包列表

| 包名 | 内容 | 大小 |
|------|------|------|
| xinwiki-infra-amd64.tar.gz | 运行时基础设施镜像 | $(du -h "$OUTPUT_DIR/xinwiki-infra-$ARCH.tar.gz" 2>/dev/null | cut -f1 || echo "N/A") |
| xinwiki-build-base-amd64.tar.gz | 构建依赖基础镜像 | $(du -h "$OUTPUT_DIR/xinwiki-build-base-$ARCH.tar.gz" 2>/dev/null | cut -f1 || echo "N/A") |

## 基础设施镜像（运行时必需）

$(for img in "${INFRA_IMAGES[@]}"; do
    size=$(docker image inspect "$img" --format='{{.Size}}' 2>/dev/null || echo 0)
    size_mb=$((size / 1024 / 1024))
    echo "- $img (${size_mb}MB)"
done)

## 构建基础镜像（构建自定义镜像时需要）

$(for img in "${BUILD_BASE_IMAGES[@]}"; do
    size=$(docker image inspect "$img" --format='{{.Size}}' 2>/dev/null || echo 0)
    size_mb=$((size / 1024 / 1024))
    echo "- $img (${size_mb}MB)"
done)

## 使用方法

```bash
# 1. 上传以下文件到目标服务器的 docker/images/ 目录
#    - xinwiki-infra-amd64.tar.gz + .sha256
#    - xinwiki-build-base-amd64.tar.gz + .sha256
#    - load-images-amd64.sh

# 2. 加载镜像
cd docker/images
chmod +x load-images-amd64.sh
./load-images-amd64.sh

# 3. 回到项目根目录，配置并启动
cd ../..
cp .env.example .env
vim .env
docker compose up -d
```
MANIFESTEOF
    
    print_ok "清单文件: $manifest"
    return 0
}

# ========== 主流程 ==========
main() {
    echo ""
    echo "========================================="
    echo "  XinWiki 离线镜像打包 ($ARCH)"
    echo "========================================="
    echo ""
    
    pull_images || true
    verify_images || {
        print_error "镜像验证失败，请检查网络或 Docker 状态"
        exit 1
    }
    package_images
    generate_artifacts
    
    echo ""
    print_ok "========== 全部完成 =========="
    echo ""
    echo "输出目录: $OUTPUT_DIR"
    ls -lh "$OUTPUT_DIR/" | grep "$ARCH"
    echo ""
    print_info "需要上传到服务器的文件:"
    echo "  - xinwiki-infra-$ARCH.tar.gz         基础设施镜像"
    echo "  - xinwiki-infra-$ARCH.tar.gz.sha256  校验文件"
    echo "  - xinwiki-build-base-$ARCH.tar.gz    构建基础镜像"
    echo "  - xinwiki-build-base-$ARCH.tar.gz.sha256 校验文件"
    echo "  - load-images-$ARCH.sh               加载脚本"
    echo "  - image-manifest-$ARCH.md            镜像清单"
    echo ""
}

main "$@"
