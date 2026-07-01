#!/bin/bash
# XinWiki amd64 基础镜像拉取与打包脚本
# 用法: ./scripts/pull-and-package-amd64-images.sh
#
# 拉取 linux/amd64 架构的基础镜像并打包为 tar.gz

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUTPUT_DIR="$PROJECT_ROOT/docker/images"
TARGET_PLATFORM="linux/amd64"
ARCH_SUFFIX="amd64"

cd "$PROJECT_ROOT"

print_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
print_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
print_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

mkdir -p "$OUTPUT_DIR"

# ========== 1. 拉取 amd64 架构的基础镜像 ==========
print_info "========== 1/3 拉取 $TARGET_PLATFORM 基础镜像 =========="

BASE_IMAGES=(
    "paradedb/paradedb:v0.22.2-pg17"
    "redis:7.0-alpine"
    "golang:1.26-bookworm"
    "python:3.10.18-bookworm"
    "python:3.11-slim"
    "node:20-slim"
    "debian:12.12-slim"
    "nginx:stable-alpine"
)

SUCCESS_IMAGES=()
FAILED_IMAGES=()

for img in "${BASE_IMAGES[@]}"; do
    print_info "拉取 $img ..."
    if docker pull --platform "$TARGET_PLATFORM" "$img" 2>&1 | tail -5; then
        arch=$(docker image inspect "$img" --format='{{.Architecture}}' 2>/dev/null || echo "unknown")
        print_ok "已拉取 $img (架构: $arch)"
        SUCCESS_IMAGES+=("$img")
    else
        print_warn "拉取失败: $img"
        FAILED_IMAGES+=("$img")
    fi
done

print_info "拉取完成: ${#SUCCESS_IMAGES[@]}/${#BASE_IMAGES[@]} 成功"

if [ ${#FAILED_IMAGES[@]} -gt 0 ]; then
    print_warn "失败的镜像: ${FAILED_IMAGES[*]}"
fi

if [ ${#SUCCESS_IMAGES[@]} -eq 0 ]; then
    print_error "没有成功拉取任何镜像"
    exit 1
fi

# ========== 2. 打包镜像 ==========
print_info "========== 2/3 打包镜像 =========="

TAR_FILE="$OUTPUT_DIR/xinwiki-base-images-$ARCH_SUFFIX.tar"
print_info "保存 ${#SUCCESS_IMAGES[@]} 个镜像到 $TAR_FILE ..."
docker save -o "$TAR_FILE" "${SUCCESS_IMAGES[@]}"
print_ok "tar 文件已生成: $TAR_FILE ($(du -h "$TAR_FILE" | cut -f1))"

print_info "压缩为 tar.gz ..."
gzip -f "$TAR_FILE"
GZ_FILE="$TAR_FILE.gz"
print_ok "压缩完成: $GZ_FILE ($(du -h "$GZ_FILE" | cut -f1))"

# 生成 SHA256 校验
sha256sum "$GZ_FILE" > "$GZ_FILE.sha256"
print_ok "校验文件: $GZ_FILE.sha256"

# ========== 3. 生成镜像清单 ==========
print_info "========== 3/3 生成镜像清单 =========="

MANIFEST="$OUTPUT_DIR/base-images-manifest-$ARCH_SUFFIX.md"
cat > "$MANIFEST" << MANIFESTEOF
# XinWiki 基础镜像清单 ($ARCH_SUFFIX)

生成时间: $(date '+%Y-%m-%d %H:%M:%S')
目标平台: $TARGET_PLATFORM
镜像包大小: $(du -h "$GZ_FILE" | cut -f1) (压缩后)

## 包含镜像

| 镜像 | 架构 | 大小 | 用途 |
|------|------|------|------|
MANIFESTEOF

declare -A IMAGE_PURPOSE=(
    ["paradedb/paradedb:v0.22.2-pg17"]="PostgreSQL + pgvector + ParadeDB (数据库)"
    ["redis:7.0-alpine"]="Redis 缓存服务"
    ["golang:1.26-bookworm"]="Go 编译环境 (构建后端镜像)"
    ["python:3.10.18-bookworm"]="Python 3.10 (构建 docreader 镜像)"
    ["python:3.11-slim"]="Python 3.11 (构建 sandbox 镜像)"
    ["node:20-slim"]="Node.js 20 (构建前端镜像)"
    ["debian:12.12-slim"]="Debian 基础镜像 (运行时)"
    ["nginx:stable-alpine"]="Nginx Web 服务器 (前端镜像基础)"
)

for img in "${SUCCESS_IMAGES[@]}"; do
    arch=$(docker image inspect "$img" --format='{{.Architecture}}' 2>/dev/null || echo "unknown")
    size=$(docker image inspect "$img" --format='{{.Size}}' 2>/dev/null || echo "0")
    size_mb=$((size / 1024 / 1024))
    purpose="${IMAGE_PURPOSE[$img]:-基础镜像}"
    echo "| $img | $arch | ${size_mb}MB | $purpose |" >> "$MANIFEST"
done

cat >> "$MANIFEST" << 'MANIFESTEOF'

## 使用方法

```bash
# 1. 加载镜像
cd docker/images
docker load -i xinwiki-base-images-amd64.tar.gz

# 或者先解压再加载:
# gunzip -k xinwiki-base-images-amd64.tar.gz
# docker load -i xinwiki-base-images-amd64.tar

# 2. 验证
docker images --format "table {{.Repository}}:{{.Tag}}\t{{.Architecture}}\t{{.Size}}"

# 3. 构建 XinWiki 自定义镜像（使用本地已有的 amd64 基础镜像）
# 在 Linux amd64 机器上直接构建即可
```
MANIFESTEOF

print_ok "清单文件: $MANIFEST"

# 清理 tar 文件
rm -f "$TAR_FILE"

print_ok "========== 完成 =========="
echo ""
echo "输出目录: $OUTPUT_DIR"
echo ""
ls -lh "$OUTPUT_DIR/"
echo ""
print_info "将以下文件拷贝到离线 Linux amd64 服务器:"
echo "  - xinwiki-base-images-$ARCH_SUFFIX.tar.gz"
echo "  - xinwiki-base-images-$ARCH_SUFFIX.tar.gz.sha256"
echo "  - base-images-manifest-$ARCH_SUFFIX.md"
