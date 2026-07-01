#!/bin/bash
# 打包 amd64 镜像为 tar.gz
# 用法: ./scripts/package-amd64-images.sh
#
# 将所有 amd64 镜像打包成 tar.gz，生成校验文件和加载脚本

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUTPUT_DIR="$PROJECT_ROOT/docker/images"
ARCH_SUFFIX="amd64"

cd "$PROJECT_ROOT"

print_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
print_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
print_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

mkdir -p "$OUTPUT_DIR"

# ========== 要打包的镜像列表 ==========
# 基础镜像（带 -amd64 后缀）
BASE_IMAGES=(
    "paradedb/paradedb:v0.22.2-pg17-$ARCH_SUFFIX"
    "redis:7.0-alpine-$ARCH_SUFFIX"
    "nginx:stable-alpine-$ARCH_SUFFIX"
    "golang:1.26-bookworm-$ARCH_SUFFIX"
    "python:3.10.18-bookworm-$ARCH_SUFFIX"
    "python:3.11-slim-$ARCH_SUFFIX"
    "node:20-slim-$ARCH_SUFFIX"
    "debian:12.12-slim-$ARCH_SUFFIX"
)

# 自定义镜像（带 -amd64 后缀）
CUSTOM_IMAGES=(
    "xinwiki/xinwiki-ui:latest-$ARCH_SUFFIX"
    "xinwiki/xinwiki-app:latest-$ARCH_SUFFIX"
    "xinwiki/xinwiki-docreader:latest-$ARCH_SUFFIX"
    "xinwiki/xinwiki-sandbox:latest-$ARCH_SUFFIX"
)

ALL_IMAGES=("${BASE_IMAGES[@]}" "${CUSTOM_IMAGES[@]}")

# ========== 1. 检查本地存在的镜像 ==========
print_info "========== 1/3 检查本地镜像 =========="

EXISTING_IMAGES=()
MISSING_IMAGES=()

for img in "${ALL_IMAGES[@]}"; do
    if docker image inspect "$img" &>/dev/null; then
        # 用 docker run 验证架构（Docker Desktop 上 inspect 的 Architecture 可能为空）
        arch=$(docker run --rm --platform linux/amd64 "$img" uname -m 2>/dev/null || echo "unknown")
        size=$(docker image inspect "$img" --format='{{.Size}}' 2>/dev/null || echo "0")
        size_mb=$((size / 1024 / 1024))
        print_info "✓ $img ($arch, ${size_mb}MB)"
        EXISTING_IMAGES+=("$img")
    else
        print_warn "✗ $img (不存在)"
        MISSING_IMAGES+=("$img")
    fi
done

echo ""
if [ ${#EXISTING_IMAGES[@]} -eq 0 ]; then
    print_error "没有可打包的镜像"
    print_info "请先运行: ./scripts/pull-amd64-images.sh 拉取基础镜像"
    print_info "然后运行: ./scripts/build-amd64-custom-images.sh 构建自定义镜像"
    exit 1
fi

if [ ${#MISSING_IMAGES[@]} -gt 0 ]; then
    print_warn "有 ${#MISSING_IMAGES[@]} 个镜像缺失，将只打包已存在的 ${#EXISTING_IMAGES[@]} 个"
fi

# ========== 2. 打包镜像 ==========
print_info "========== 2/3 打包镜像 =========="

TAR_FILE="$OUTPUT_DIR/xinwiki-all-images-$ARCH_SUFFIX.tar"
print_info "保存 ${#EXISTING_IMAGES[@]} 个镜像到 $TAR_FILE ..."
print_info "这可能需要几分钟，请耐心等待..."

docker save -o "$TAR_FILE" "${EXISTING_IMAGES[@]}"
print_ok "tar 文件已生成: $TAR_FILE ($(du -h "$TAR_FILE" | cut -f1))"

print_info "压缩为 tar.gz ..."
gzip -f "$TAR_FILE"
GZ_FILE="$TAR_FILE.gz"
print_ok "压缩完成: $GZ_FILE ($(du -h "$GZ_FILE" | cut -f1))"

# 生成 SHA256 校验
sha256sum "$GZ_FILE" > "$GZ_FILE.sha256"
print_ok "校验文件: $GZ_FILE.sha256"

# ========== 3. 生成加载脚本和清单 ==========
print_info "========== 3/3 生成加载脚本和清单 =========="

# 生成加载脚本
LOAD_SCRIPT="$OUTPUT_DIR/load-images-$ARCH_SUFFIX.sh"
cat > "$LOAD_SCRIPT" << LOADEOF
#!/bin/bash
# XinWiki 离线镜像加载脚本 ($ARCH_SUFFIX 版本)
# 用法: ./load-images-$ARCH_SUFFIX.sh
#
# 加载后，镜像会带有 -$ARCH_SUFFIX 后缀。
# 如需使用原始 tag 名称，请手动 retag。

set -e

SCRIPT_DIR="\$(cd "\$(dirname "\${BASH_SOURCE[0]}")" && pwd)"
GZ_FILE="\$SCRIPT_DIR/xinwiki-all-images-$ARCH_SUFFIX.tar.gz"
TAR_FILE="\$SCRIPT_DIR/xinwiki-all-images-$ARCH_SUFFIX.tar"

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

echo ""
echo "解压镜像文件..."
gunzip -k -f "\$GZ_FILE"

echo ""
echo "加载 Docker 镜像（这可能需要几分钟）..."
docker load -i "\$TAR_FILE"

echo ""
echo "========================================="
echo "  XinWiki 镜像加载完成 ($ARCH_SUFFIX)！"
echo "========================================="
echo ""
echo "已加载的镜像（带 -$ARCH_SUFFIX 后缀）:"
docker images --format "table {{.Repository}}:{{.Tag}}\t{{.Architecture}}\t{{.Size}}" | grep -- "-$ARCH_SUFFIX"
echo ""
echo "提示: 镜像带有 -$ARCH_SUFFIX 后缀。如需在 docker-compose 中使用，请:"
echo "  方式一: 修改 docker-compose.yml 中的镜像名为带后缀的版本"
echo "  方式二: 手动 retag，例如:"
echo "    docker tag paradedb/paradedb:v0.22.2-pg17-$ARCH_SUFFIX paradedb/paradedb:v0.22.2-pg17"
echo ""
echo "下一步:"
echo "  1. 复制项目代码到部署目录"
echo "  2. cp .env.example .env && vim .env (配置必要参数)"
echo "  3. docker compose up -d"
echo ""

# 清理 tar 文件（保留 tar.gz）
rm -f "\$TAR_FILE"
LOADEOF

chmod +x "$LOAD_SCRIPT"
print_ok "加载脚本: $LOAD_SCRIPT"

# 生成镜像清单
MANIFEST="$OUTPUT_DIR/image-manifest-$ARCH_SUFFIX.md"
cat > "$MANIFEST" << MANIFESTEOF
# XinWiki 离线部署镜像清单 ($ARCH_SUFFIX)

生成时间: $(date '+%Y-%m-%d %H:%M:%S')
目标平台: linux/$ARCH_SUFFIX
镜像包大小: $(du -h "$GZ_FILE" | cut -f1) (压缩后)
镜像数量: ${#EXISTING_IMAGES[@]}

## 包含镜像

| 镜像 (包内 tag) | 原始镜像名 | 架构 | 大小 | 用途 |
|----------------|-----------|------|------|------|
MANIFESTEOF

declare -A IMAGE_PURPOSE=(
    ["paradedb/paradedb:v0.22.2-pg17-$ARCH_SUFFIX"]="PostgreSQL + pgvector + ParadeDB (数据库)|paradedb/paradedb:v0.22.2-pg17"
    ["redis:7.0-alpine-$ARCH_SUFFIX"]="Redis 缓存服务|redis:7.0-alpine"
    ["nginx:stable-alpine-$ARCH_SUFFIX"]="Nginx Web 服务器|nginx:stable-alpine"
    ["golang:1.26-bookworm-$ARCH_SUFFIX"]="Go 编译环境（构建后端镜像）|golang:1.26-bookworm"
    ["python:3.10.18-bookworm-$ARCH_SUFFIX"]="Python 3.10（构建 docreader 镜像）|python:3.10.18-bookworm"
    ["python:3.11-slim-$ARCH_SUFFIX"]="Python 3.11（构建 sandbox 镜像）|python:3.11-slim"
    ["node:20-slim-$ARCH_SUFFIX"]="Node.js 20（构建前端镜像）|node:20-slim"
    ["debian:12.12-slim-$ARCH_SUFFIX"]="Debian 基础镜像（运行时）|debian:12.12-slim"
    ["xinwiki/xinwiki-ui:latest-$ARCH_SUFFIX"]="前端 Web UI|xinwiki/xinwiki-ui:latest"
    ["xinwiki/xinwiki-app:latest-$ARCH_SUFFIX"]="后端 API 服务|xinwiki/xinwiki-app:latest"
    ["xinwiki/xinwiki-docreader:latest-$ARCH_SUFFIX"]="文档解析服务|xinwiki/xinwiki-docreader:latest"
    ["xinwiki/xinwiki-sandbox:latest-$ARCH_SUFFIX"]="Agent Skill 沙箱|xinwiki/xinwiki-sandbox:latest"
)

for img in "${EXISTING_IMAGES[@]}"; do
    # 用 docker run 验证架构
    arch=$(docker run --rm --platform linux/amd64 "$img" uname -m 2>/dev/null || echo "unknown")
    size=$(docker image inspect "$img" --format='{{.Size}}' 2>/dev/null || echo "0")
    size_mb=$((size / 1024 / 1024))
    purpose_info="${IMAGE_PURPOSE[$img]:-基础镜像|unknown}"
    purpose="${purpose_info%%|*}"
    original="${purpose_info##*|}"
    echo "| $img | $original | $arch | ${size_mb}MB | $purpose |" >> "$MANIFEST"
done

cat >> "$MANIFEST" << 'MANIFESTEOF'

## 使用方法

```bash
# 1. 上传以下文件到 Linux amd64 服务器的 docker/images/ 目录
#    - xinwiki-all-images-amd64.tar.gz
#    - xinwiki-all-images-amd64.tar.gz.sha256
#    - load-images-amd64.sh

# 2. 加载镜像
cd docker/images
chmod +x load-images-amd64.sh
./load-images-amd64.sh

# 3. 回到项目根目录，配置环境
cd ../..
cp .env.example .env
vim .env

# 4. 启动服务
docker compose up -d
```

## 镜像重命名（可选）

加载后镜像带有 `-amd64` 后缀。如果需要使用原始 tag 名（docker-compose 默认识别的），可以执行：

```bash
# 重命名基础镜像
docker tag paradedb/paradedb:v0.22.2-pg17-amd64 paradedb/paradedb:v0.22.2-pg17
docker tag redis:7.0-alpine-amd64 redis:7.0-alpine
docker tag nginx:stable-alpine-amd64 nginx:stable-alpine

# 重命名自定义镜像
docker tag xinwiki/xinwiki-ui:latest-amd64 xinwiki/xinwiki-ui:latest
docker tag xinwiki/xinwiki-app:latest-amd64 xinwiki/xinwiki-app:latest
docker tag xinwiki/xinwiki-docreader:latest-amd64 xinwiki/xinwiki-docreader:latest
docker tag xinwiki/xinwiki-sandbox:latest-amd64 xinwiki/xinwiki-sandbox:latest
```
MANIFESTEOF

print_ok "清单文件: $MANIFEST"

# 清理
rm -f "$TAR_FILE"

print_ok "========== 全部完成 =========="
echo ""
echo "输出目录: $OUTPUT_DIR"
echo ""
ls -lh "$OUTPUT_DIR/" | grep "$ARCH_SUFFIX"
echo ""
print_info "需要上传到 Linux amd64 服务器的文件:"
echo "  1. xinwiki-all-images-$ARCH_SUFFIX.tar.gz    - 镜像压缩包"
echo "  2. xinwiki-all-images-$ARCH_SUFFIX.tar.gz.sha256 - 校验文件"
echo "  3. load-images-$ARCH_SUFFIX.sh              - 加载脚本"
echo "  4. image-manifest-$ARCH_SUFFIX.md           - 镜像清单"
echo ""
print_info "在服务器上执行: ./load-images-$ARCH_SUFFIX.sh"
