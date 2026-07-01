#!/bin/bash
# XinWiki 离线部署镜像打包脚本 (linux/amd64 版本)
# 用法: ./scripts/build-offline-images-amd64.sh
#
# 构建所有自定义镜像 + 拉取所有第三方镜像（amd64 架构），打包为 tar.gz 用于离线部署
# 在 macOS (arm64) 上运行需要 Docker Desktop 启用 buildx

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

# 检查 Docker buildx 是否可用
check_buildx() {
    if docker buildx version &>/dev/null; then
        print_ok "Docker buildx 可用"
        return 0
    else
        print_error "Docker buildx 不可用，无法构建跨平台镜像"
        print_info "请安装 Docker Desktop 或启用 buildx 插件"
        return 1
    fi
}

# 检查 buildx builder 是否已创建
ensure_builder() {
    if docker buildx inspect xinwiki-builder &>/dev/null 2>&1; then
        print_info "使用已有的 buildx builder: xinwiki-builder"
    else
        print_info "创建 buildx builder: xinwiki-builder ..."
        docker buildx create --name xinwiki-builder --driver docker-container --use
        docker buildx inspect --bootstrap
        print_ok "buildx builder 创建成功"
    fi
    docker buildx use xinwiki-builder
}

# ========== 1. 构建自定义镜像 (amd64) ==========
print_info "========== 1/5 构建自定义镜像 ($TARGET_PLATFORM) =========="

CUSTOM_IMAGES=(
    "xinwiki/xinwiki-ui:latest"
    "xinwiki/xinwiki-app:latest"
    "xinwiki/xinwiki-docreader:latest"
    "xinwiki/xinwiki-sandbox:latest"
)

ensure_builder

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

get_version_info

print_info "构建 frontend 镜像 ($TARGET_PLATFORM)..."
if [ -d "frontend/dist" ]; then
    if docker buildx build --platform "$TARGET_PLATFORM" --load \
        -t xinwiki/xinwiki-ui:latest-$ARCH_SUFFIX \
        -f frontend/Dockerfile frontend/ 2>&1 | tail -5; then
        print_ok "frontend 镜像构建完成"
        # 重新打标签，去掉后缀
        docker tag xinwiki/xinwiki-ui:latest-$ARCH_SUFFIX xinwiki/xinwiki-ui:latest
    else
        print_warn "frontend 镜像构建失败"
    fi
else
    print_warn "frontend/dist 不存在，跳过前端镜像构建（请先运行 ./scripts/build_frontend_dist.sh）"
fi

print_info "构建 app 镜像 ($TARGET_PLATFORM)..."
if docker buildx build --platform "$TARGET_PLATFORM" --load \
    --build-arg GOPRIVATE_ARG="${GOPRIVATE:-}" \
    --build-arg GOPROXY_ARG="${GOPROXY:-https://goproxy.cn,direct}" \
    --build-arg GOSUMDB_ARG="${GOSUMDB:-off}" \
    --build-arg VERSION_ARG="$VERSION" \
    --build-arg COMMIT_ID_ARG="$COMMIT_ID" \
    --build-arg BUILD_TIME_ARG="$BUILD_TIME" \
    --build-arg GO_VERSION_ARG="$GO_VERSION" \
    -t xinwiki/xinwiki-app:latest-$ARCH_SUFFIX \
    -f docker/Dockerfile.app . 2>&1 | tail -5; then
    print_ok "app 镜像构建完成"
    docker tag xinwiki/xinwiki-app:latest-$ARCH_SUFFIX xinwiki/xinwiki-app:latest
else
    print_warn "app 镜像构建失败"
fi

print_info "构建 docreader 镜像 ($TARGET_PLATFORM)..."
if docker buildx build --platform "$TARGET_PLATFORM" --load \
    --build-arg APT_MIRROR="${APT_MIRROR:-}" \
    -t xinwiki/xinwiki-docreader:latest-$ARCH_SUFFIX \
    -f docker/Dockerfile.docreader docreader/ 2>&1 | tail -5; then
    print_ok "docreader 镜像构建完成"
    docker tag xinwiki/xinwiki-docreader:latest-$ARCH_SUFFIX xinwiki/xinwiki-docreader:latest
else
    print_warn "docreader 镜像构建失败"
fi

print_info "构建 sandbox 镜像 ($TARGET_PLATFORM)..."
if [ -f docker/Dockerfile.sandbox ]; then
    if docker buildx build --platform "$TARGET_PLATFORM" --load \
        -t xinwiki/xinwiki-sandbox:latest-$ARCH_SUFFIX \
        -f docker/Dockerfile.sandbox . 2>&1 | tail -5; then
        print_ok "sandbox 镜像构建完成"
        docker tag xinwiki/xinwiki-sandbox:latest-$ARCH_SUFFIX xinwiki/xinwiki-sandbox:latest
    else
        print_warn "sandbox 镜像构建失败"
    fi
else
    print_warn "Dockerfile.sandbox 不存在，跳过"
fi

# ========== 2. 拉取第三方基础镜像 (amd64) ==========
print_info "========== 2/5 拉取第三方基础镜像 ($TARGET_PLATFORM) =========="

THIRD_PARTY_IMAGES=(
    # 数据库
    "paradedb/paradedb:v0.22.2-pg17"
    "redis:7.0-alpine"
    # 基础设施
    "busybox:1.36"
    "nginx:stable-alpine"
    # 构建基础镜像
    "golang:1.26-bookworm"
    "python:3.10.18-bookworm"
    "python:3.11-slim"
    "node:20-slim"
    "debian:12.12-slim"
    # 对象存储
    "minio/minio:RELEASE.2025-09-07T16-13-09Z"
    # 可选向量数据库
    "qdrant/qdrant:v1.16.2"
    # 知识图谱
    "neo4j:2025.10.1"
    # 网络搜索
    "searxng/searxng:latest"
    # Langfuse 可观测性
    "clickhouse/clickhouse-server:24.8"
    "langfuse/langfuse:3"
    "langfuse/langfuse-worker:3"
)

for img in "${THIRD_PARTY_IMAGES[@]}"; do
    print_info "拉取 $img ($TARGET_PLATFORM) ..."
    if docker pull --platform "$TARGET_PLATFORM" "$img" 2>&1 | tail -3; then
        print_ok "已拉取 $img"
    else
        print_warn "拉取失败: $img（离线部署时可能需要手动获取）"
    fi
done

# ========== 3. 验证镜像架构 ==========
print_info "========== 3/5 验证镜像架构 =========="

ALL_IMAGES=("${CUSTOM_IMAGES[@]}" "${THIRD_PARTY_IMAGES[@]}")

# 过滤出本地实际存在的镜像
EXISTING_IMAGES=()
for img in "${ALL_IMAGES[@]}"; do
    if docker image inspect "$img" &>/dev/null; then
        arch=$(docker image inspect "$img" --format='{{.Architecture}}' 2>/dev/null || echo "unknown")
        print_info "$img 架构: $arch"
        EXISTING_IMAGES+=("$img")
    else
        print_warn "镜像不存在，跳过: $img"
    fi
done

if [ ${#EXISTING_IMAGES[@]} -eq 0 ]; then
    print_error "没有可打包的镜像"
    exit 1
fi

# ========== 4. 打包所有镜像 ==========
print_info "========== 4/5 打包镜像 ($ARCH_SUFFIX) =========="

print_info "打包 ${#EXISTING_IMAGES[@]} 个镜像到 $OUTPUT_DIR ..."

# 保存所有镜像到一个 tar 文件（带 amd64 后缀）
TAR_FILE="$OUTPUT_DIR/xinwiki-all-images-$ARCH_SUFFIX.tar"
print_info "执行 docker save（这可能需要几分钟）..."
docker save -o "$TAR_FILE" "${EXISTING_IMAGES[@]}"
print_ok "tar 文件已生成: $TAR_FILE ($(du -h "$TAR_FILE" | cut -f1))"

# 压缩
print_info "压缩为 tar.gz ..."
gzip -f "$TAR_FILE"
GZ_FILE="$TAR_FILE.gz"
print_ok "压缩完成: $GZ_FILE ($(du -h "$GZ_FILE" | cut -f1))"

# 生成 SHA256 校验文件
sha256sum "$GZ_FILE" > "$GZ_FILE.sha256"
print_ok "校验文件: $GZ_FILE.sha256"

# 生成镜像清单
MANIFEST="$OUTPUT_DIR/image-manifest-$ARCH_SUFFIX.md"
cat > "$MANIFEST" << MANIFESTEOF
# XinWiki 离线部署镜像清单 ($ARCH_SUFFIX)

生成时间: $(date '+%Y-%m-%d %H:%M:%S')
目标平台: $TARGET_PLATFORM
镜像包大小: $(du -h "$GZ_FILE" | cut -f1) (压缩后)

## 包含镜像

| 镜像 | 架构 | 大小 | 必需 |
|------|------|------|------|
MANIFESTEOF

# 标记必需镜像
REQUIRED_IMAGES=(
    "xinwiki/xinwiki-ui:latest"
    "xinwiki/xinwiki-app:latest"
    "paradedb/paradedb:v0.22.2-pg17"
    "redis:7.0-alpine"
    "nginx:stable-alpine"
)

for img in "${EXISTING_IMAGES[@]}"; do
    arch=$(docker image inspect "$img" --format='{{.Architecture}}' 2>/dev/null || echo "unknown")
    size=$(docker image inspect "$img" --format='{{.Size}}' 2>/dev/null || echo "0")
    size_mb=$((size / 1024 / 1024))
    required=" "
    for req in "${REQUIRED_IMAGES[@]}"; do
        if [ "$img" = "$req" ]; then
            required="✅"
            break
        fi
    done
    echo "| $img | $arch | ${size_mb}MB | $required |" >> "$MANIFEST"
done

cat >> "$MANIFEST" << 'MANIFESTEOF'

## 未包含的可选镜像

以下镜像为可选功能，离线部署时按需自行拉取：

| 镜像 | 用途 | Profile |
|------|------|---------|
| minio/minio:RELEASE.2025-09-07T16-13-09Z | 对象存储 | --profile minio |
| neo4j:2025.10.1 | 知识图谱 | --profile neo4j |
| qdrant/qdrant:v1.16.2 | 向量数据库 | - |
| searxng/searxng:latest | 网络搜索 | --profile searxng |
| clickhouse/clickhouse-server:24.8 | Langfuse 分析 | --profile langfuse |
| langfuse/langfuse:3 | LLM 可观测性 | --profile langfuse |
| langfuse/langfuse-worker:3 | Langfuse Worker | --profile langfuse |

## 使用方法

```bash
# 1. 加载镜像
cd docker/images
chmod +x load-images-amd64.sh
./load-images-amd64.sh

# 2. 回到项目根目录，配置环境
cd ../..
cp .env.example .env
vim .env  # 配置数据库密码、JWT密钥、LLM API Key 等

# 3. 启动服务
docker compose up -d

# 4. 访问
# Web UI: http://localhost
# 默认账号: admin@xinwiki.com / admin123
```
MANIFESTEOF

print_ok "清单文件: $MANIFEST"

# ========== 5. 生成离线加载脚本 ==========
print_info "========== 5/5 生成离线加载脚本 =========="

LOAD_SCRIPT="$OUTPUT_DIR/load-images-$ARCH_SUFFIX.sh"
cat > "$LOAD_SCRIPT" << LOADEOF
#!/bin/bash
# XinWiki 离线镜像加载脚本 ($ARCH_SUFFIX 版本)
# 用法: ./load-images-$ARCH_SUFFIX.sh

set -e

SCRIPT_DIR="\$(cd "\$(dirname "\${BASH_SOURCE[0]}")" && pwd)"
GZ_FILE="\$SCRIPT_DIR/xinwiki-all-images-$ARCH_SUFFIX.tar.gz"

if [ ! -f "\$GZ_FILE" ]; then
    echo "错误: 镜像文件不存在: \$GZ_FILE"
    exit 1
fi

# 校验 SHA256
if [ -f "\$GZ_FILE.sha256" ]; then
    echo "校验文件完整性..."
    (cd "\$SCRIPT_DIR" && sha256sum -c xinwiki-all-images-$ARCH_SUFFIX.tar.gz.sha256) || {
        echo "警告: 文件校验失败，文件可能已损坏"
        read -p "是否继续加载？(y/N): " confirm
        [ "\$confirm" != "y" ] && exit 1
    }
    echo "校验通过"
fi

echo "解压镜像文件..."
gunzip -k -f "\$GZ_FILE"

echo "加载 Docker 镜像（这可能需要几分钟）..."
docker load -i "\$SCRIPT_DIR/xinwiki-all-images-$ARCH_SUFFIX.tar"

echo ""
echo "========================================="
echo "  XinWiki 镜像加载完成 ($ARCH_SUFFIX)！"
echo "========================================="
echo ""
echo "已加载的镜像:"
docker images --format "table {{.Repository}}:{{.Tag}}\t{{.Architecture}}\t{{.Size}}" | grep -E "xinwiki|paradedb|redis|nginx|busybox|minio|qdrant|neo4j|searxng|clickhouse|langfuse|milvus|weaviate|doris|golang|python|node|debian"
echo ""
echo "下一步:"
echo "  1. 复制项目代码到部署目录"
echo "  2. cp .env.example .env && vim .env (配置必要参数)"
echo "  3. docker compose up -d"
echo ""
LOADEOF

chmod +x "$LOAD_SCRIPT"
print_ok "离线加载脚本: $LOAD_SCRIPT"

# 清理 tar 文件（保留 tar.gz）
rm -f "$TAR_FILE"

print_ok "========== 全部完成 =========="
echo ""
echo "输出目录: $OUTPUT_DIR"
echo ""
ls -lh "$OUTPUT_DIR/"
echo ""
print_info "将 docker/images/ 目录下的以下文件拷贝到离线服务器："
echo "  - xinwiki-all-images-$ARCH_SUFFIX.tar.gz"
echo "  - xinwiki-all-images-$ARCH_SUFFIX.tar.gz.sha256"
echo "  - load-images-$ARCH_SUFFIX.sh"
echo "  - image-manifest-$ARCH_SUFFIX.md"
echo ""
print_info "在离线服务器上执行: ./load-images-$ARCH_SUFFIX.sh"
