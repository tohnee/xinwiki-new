#!/bin/bash
# XinWiki 离线部署镜像打包脚本
# 用法: ./scripts/build-offline-images.sh
#
# 构建所有自定义镜像 + 拉取所有第三方镜像，打包为 tar.gz 用于离线部署

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUTPUT_DIR="$PROJECT_ROOT/docker/images"

cd "$PROJECT_ROOT"

print_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
print_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
print_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

mkdir -p "$OUTPUT_DIR"

# ========== 1. 构建自定义镜像 ==========
print_info "========== 1/4 构建自定义镜像 =========="

CUSTOM_IMAGES=(
    "xinwiki/xinwiki-ui:latest"
    "xinwiki/xinwiki-app:latest"
    "xinwiki/xinwiki-docreader:latest"
    "xinwiki/xinwiki-sandbox:latest"
)

print_info "构建 frontend 镜像..."
if docker build -t xinwiki/xinwiki-ui:latest -f frontend/Dockerfile frontend/ 2>&1 | tail -5; then
    print_ok "frontend 镜像构建完成"
else
    print_warn "frontend 镜像构建失败（需要先运行 ./scripts/build_frontend_dist.sh）"
fi

print_info "构建 app 镜像..."
if docker build -t xinwiki/xinwiki-app:latest -f docker/Dockerfile.app . 2>&1 | tail -5; then
    print_ok "app 镜像构建完成"
else
    print_warn "app 镜像构建失败"
fi

print_info "构建 docreader 镜像..."
if docker build -t xinwiki/xinwiki-docreader:latest -f docker/Dockerfile.docreader docreader/ 2>&1 | tail -5; then
    print_ok "docreader 镜像构建完成"
else
    print_warn "docreader 镜像构建失败"
fi

print_info "构建 sandbox 镜像..."
if [ -f docker/Dockerfile.sandbox ]; then
    if docker build -t xinwiki/xinwiki-sandbox:latest -f docker/Dockerfile.sandbox . 2>&1 | tail -5; then
        print_ok "sandbox 镜像构建完成"
    else
        print_warn "sandbox 镜像构建失败"
    fi
else
    print_warn "Dockerfile.sandbox 不存在，跳过"
fi

# ========== 2. 拉取第三方基础镜像 ==========
print_info "========== 2/4 拉取第三方基础镜像 =========="

THIRD_PARTY_IMAGES=(
    # 数据库
    "paradedb/paradedb:v0.22.2-pg17"
    "redis:7.0-alpine"
    # 基础设施
    "busybox:1.36"
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
    print_info "拉取 $img ..."
    if docker pull "$img" 2>&1 | tail -3; then
        print_ok "已拉取 $img"
    else
        print_warn "拉取失败: $img（离线部署时可能需要手动获取）"
    fi
done

# ========== 3. 打包所有镜像 ==========
print_info "========== 3/4 打包镜像 =========="

ALL_IMAGES=("${CUSTOM_IMAGES[@]}" "${THIRD_PARTY_IMAGES[@]}")

# 过滤出本地实际存在的镜像
EXISTING_IMAGES=()
for img in "${ALL_IMAGES[@]}"; do
    if docker image inspect "$img" &>/dev/null; then
        EXISTING_IMAGES+=("$img")
    else
        print_warn "镜像不存在，跳过: $img"
    fi
done

if [ ${#EXISTING_IMAGES[@]} -eq 0 ]; then
    print_error "没有可打包的镜像"
    exit 1
fi

print_info "打包 ${#EXISTING_IMAGES[@]} 个镜像到 $OUTPUT_DIR ..."

# 保存所有镜像到一个 tar 文件
TAR_FILE="$OUTPUT_DIR/xinwiki-all-images.tar"
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
MANIFEST="$OUTPUT_DIR/image-manifest.txt"
echo "# XinWiki 离线部署镜像清单" > "$MANIFEST"
echo "# 生成时间: $(date)" >> "$MANIFEST"
echo "# 镜像数量: ${#EXISTING_IMAGES[@]}" >> "$MANIFEST"
echo "" >> "$MANIFEST"
for img in "${EXISTING_IMAGES[@]}"; do
    size=$(docker image inspect "$img" --format='{{.Size}}' 2>/dev/null || echo "0")
    size_mb=$((size / 1024 / 1024))
    echo "$img  (${size_mb}MB)" >> "$MANIFEST"
done
print_ok "清单文件: $MANIFEST"

# ========== 4. 生成离线加载脚本 ==========
print_info "========== 4/4 生成离线加载脚本 =========="

LOAD_SCRIPT="$OUTPUT_DIR/load-images.sh"
cat > "$LOAD_SCRIPT" << 'LOADEOF'
#!/bin/bash
# XinWiki 离线镜像加载脚本
# 用法: ./load-images.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GZ_FILE="$SCRIPT_DIR/xinwiki-all-images.tar.gz"

if [ ! -f "$GZ_FILE" ]; then
    echo "错误: 镜像文件不存在: $GZ_FILE"
    exit 1
fi

# 校验 SHA256
if [ -f "$GZ_FILE.sha256" ]; then
    echo "校验文件完整性..."
    (cd "$SCRIPT_DIR" && sha256sum -c xinwiki-all-images.tar.gz.sha256) || {
        echo "警告: 文件校验失败，文件可能已损坏"
        read -p "是否继续加载？(y/N): " confirm
        [ "$confirm" != "y" ] && exit 1
    }
    echo "校验通过"
fi

echo "解压镜像文件..."
gunzip -k -f "$GZ_FILE"

echo "加载 Docker 镜像（这可能需要几分钟）..."
docker load -i "$SCRIPT_DIR/xinwiki-all-images.tar"

echo ""
echo "========================================="
echo "  镜像加载完成！"
echo "========================================="
echo ""
echo "已加载的镜像:"
docker images --format "table {{.Repository}}:{{.Tag}}\t{{.Size}}" | grep -E "xinwiki|paradedb|redis|busybox|minio|qdrant|neo4j|searxng|clickhouse|langfuse|milvus|weaviate|doris"
echo ""
echo "下一步: 在项目根目录执行 docker compose up -d 启动服务"
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
print_info "将整个 docker/images/ 目录拷贝到离线服务器，执行 ./load-images.sh 即可加载"
