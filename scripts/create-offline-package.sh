#!/bin/bash
# 创建 XinWiki 完整离线部署包
# 用法: ./scripts/create-offline-package.sh
#
# 打包项目必要文件 + 已构建的镜像 tar + 构建脚本
# 用于在无互联网的 Linux amd64 服务器上部署

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
AMD64_DIR="$PROJECT_ROOT/docker/images/amd64"
PACKAGE_DIR="/tmp/xinwiki-offline-deploy"

cd "$PROJECT_ROOT"

print_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
print_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
print_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# ========== 1. 准备打包目录 ==========
print_info "========== 1/5 准备打包目录 =========="

rm -rf "$PACKAGE_DIR"
mkdir -p "$PACKAGE_DIR"/{docker/images,config,migrations,scripts,docker/searxng}

print_ok "打包目录: $PACKAGE_DIR"

# ========== 2. 复制项目文件 ==========
print_info "========== 2/5 复制项目文件 =========="

# 核心配置
cp docker-compose.yml "$PACKAGE_DIR/docker-compose.yml"
cp .env.example "$PACKAGE_DIR/.env.example"
cp VERSION "$PACKAGE_DIR/VERSION" 2>/dev/null || echo "unknown" > "$PACKAGE_DIR/VERSION"

# config 目录
cp -r config/* "$PACKAGE_DIR/config/" 2>/dev/null || print_warn "config/ 目录为空或不存在"

# migrations 目录
cp -r migrations/versioned/* "$PACKAGE_DIR/migrations/" 2>/dev/null || print_warn "migrations/versioned/ 不存在"

# docker/searxng 配置
cp docker/searxng/settings.yml "$PACKAGE_DIR/docker/searxng/" 2>/dev/null || print_warn "docker/searxng/ 不存在"

# skills/preloaded 目录（空目录占位）
mkdir -p "$PACKAGE_DIR/skills/preloaded"

# Dockerfiles（供 Linux 上构建用）
cp docker/Dockerfile.app "$PACKAGE_DIR/docker/"
cp docker/Dockerfile.docreader "$PACKAGE_DIR/docker/"
cp docker/Dockerfile.sandbox "$PACKAGE_DIR/docker/"
cp docker/Dockerfile.odl-hybrid "$PACKAGE_DIR/docker/" 2>/dev/null || true
cp frontend/Dockerfile "$PACKAGE_DIR/docker/Dockerfile.ui"

print_ok "项目文件复制完成"

# ========== 3. 复制镜像 tar 文件 ==========
print_info "========== 3/5 复制镜像 tar 文件 =========="

COPIED_IMAGES=0
if [ -f "$AMD64_DIR/xinwiki-ui.tar" ]; then
    cp "$AMD64_DIR/xinwiki-ui.tar" "$PACKAGE_DIR/docker/images/"
    print_ok "✓ xinwiki-ui.tar ($(du -h "$AMD64_DIR/xinwiki-ui.tar" | cut -f1))"
    COPIED_IMAGES=$((COPIED_IMAGES + 1))
else
    print_warn "✗ xinwiki-ui.tar 不存在（需先构建）"
fi

if [ -f "$AMD64_DIR/xinwiki-sandbox.tar" ]; then
    cp "$AMD64_DIR/xinwiki-sandbox.tar" "$PACKAGE_DIR/docker/images/"
    print_ok "✓ xinwiki-sandbox.tar ($(du -h "$AMD64_DIR/xinwiki-sandbox.tar" | cut -f1))"
    COPIED_IMAGES=$((COPIED_IMAGES + 1))
else
    print_warn "✗ xinwiki-sandbox.tar 不存在（需先构建）"
fi

if [ -f "$AMD64_DIR/xinwiki-app.tar" ]; then
    cp "$AMD64_DIR/xinwiki-app.tar" "$PACKAGE_DIR/docker/images/"
    print_ok "✓ xinwiki-app.tar ($(du -h "$AMD64_DIR/xinwiki-app.tar" | cut -f1))"
    COPIED_IMAGES=$((COPIED_IMAGES + 1))
else
    print_warn "✗ xinwiki-app.tar 不存在（需先构建或在 Linux 上构建）"
fi

if [ -f "$AMD64_DIR/xinwiki-docreader.tar" ]; then
    cp "$AMD64_DIR/xinwiki-docreader.tar" "$PACKAGE_DIR/docker/images/"
    print_ok "✓ xinwiki-docreader.tar ($(du -h "$AMD64_DIR/xinwiki-docreader.tar" | cut -f1))"
    COPIED_IMAGES=$((COPIED_IMAGES + 1))
else
    print_warn "✗ xinwiki-docreader.tar 不存在（需先构建或在 Linux 上构建）"
fi

print_info "已复制 $COPIED_IMAGES/4 个自定义镜像"

# ========== 4. 创建部署脚本 ==========
print_info "========== 4/5 创建部署脚本 =========="

# 4.1 镜像加载脚本
cat > "$PACKAGE_DIR/scripts/load-images.sh" << 'LOADEOF'
#!/bin/bash
# XinWiki 离线镜像加载脚本
# 加载所有本地 tar 镜像到 Docker

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGES_DIR="$SCRIPT_DIR/../docker/images"

echo "========================================="
echo "  XinWiki 离线镜像加载"
echo "========================================="
echo ""

# 加载自定义镜像
for tar_file in "$IMAGES_DIR"/xinwiki-*.tar; do
    if [ -f "$tar_file" ]; then
        filename=$(basename "$tar_file")
        echo "加载 $filename ..."
        docker load -i "$tar_file"
        echo "✓ $filename 加载完成"
        echo ""
    fi
done

# 如果有基础镜像 tar，也加载
for tar_file in "$IMAGES_DIR"/*.tar; do
    basename_file=$(basename "$tar_file")
    # 跳过已加载的 xinwiki-* 镜像
    case "$basename_file" in
        xinwiki-*) continue ;;
    esac
    if [ -f "$tar_file" ]; then
        echo "加载 $basename_file ..."
        docker load -i "$tar_file"
        echo "✓ $basename_file 加载完成"
        echo ""
    fi
done

echo "========================================="
echo "  镜像加载完成！"
echo "========================================="
echo ""
echo "当前 Docker 镜像列表:"
docker images --format "table {{.Repository}}:{{.Tag}}\t{{.Size}}" | grep -E "(xinwiki|paradedb|redis|nginx)" || true
echo ""
LOADEOF
chmod +x "$PACKAGE_DIR/scripts/load-images.sh"

# 4.2 一键部署脚本
cat > "$PACKAGE_DIR/deploy.sh" << 'DEPLOYEOF'
#!/bin/bash
# XinWiki 一键离线部署脚本
# 用法: ./deploy.sh [--profile full|neo4j|minio|langfuse|searxng]

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

print_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
print_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
print_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

PROFILE="${1:---profile full}"
if [ "$1" = "--minimal" ]; then
    PROFILE=""
fi

echo ""
echo "========================================="
echo "  XinWiki 离线部署"
echo "========================================="
echo ""

# 1. 检查 Docker
print_info "检查 Docker 环境..."
if ! command -v docker &>/dev/null; then
    print_error "Docker 未安装"
    exit 1
fi
print_ok "Docker 版本: $(docker --version)"

if ! docker compose version &>/dev/null; then
    print_error "Docker Compose 未安装"
    print_info "请安装 Docker Compose V2: sudo apt-get install docker-compose-plugin"
    exit 1
fi
print_ok "Docker Compose: $(docker compose version --short)"

# 2. 加载镜像
print_info ""
print_info "加载离线镜像..."
if [ -f "scripts/load-images.sh" ]; then
    bash scripts/load-images.sh
else
    print_warn "load-images.sh 不存在，跳过镜像加载"
fi

# 3. 配置环境
print_info "配置环境..."
if [ ! -f ".env" ]; then
    cp .env.example .env
    print_warn "已从 .env.example 创建 .env"
    print_warn "请编辑 .env 配置以下必要参数："
    echo "  - DB_PASSWORD (数据库密码)"
    echo "  - JWT_SECRET (JWT 密钥)"
    echo "  - 至少一个 LLM API Key (如 OPENAI_API_KEY)"
    echo ""
    echo "  生成随机密钥: openssl rand -hex 32"
    echo ""
    read -p "是否现在编辑 .env？(y/N): " edit_env
    if [ "$edit_env" = "y" ] || [ "$edit_env" = "Y" ]; then
        ${EDITOR:-vim} .env
    fi
else
    print_ok ".env 已存在"
fi

# 4. 启动服务
print_info ""
print_info "启动服务..."
if [ -n "$PROFILE" ]; then
    print_info "使用 profile: $PROFILE"
    docker compose $PROFILE up -d
else
    print_info "最小化部署（仅核心服务）"
    docker compose up -d
fi

# 5. 等待服务就绪
print_info ""
print_info "等待服务启动..."
sleep 10

# 检查服务状态
print_info "服务状态:"
docker compose ps

echo ""
print_ok "========================================="
print_ok "  XinWiki 部署完成！"
print_ok "========================================="
echo ""
echo "访问地址:"
echo "  Web UI: http://localhost (端口 80)"
echo "  API:    http://localhost:8080"
echo "  Swagger: http://localhost:8080/swagger/index.html"
echo ""
echo "默认管理员账号:"
echo "  邮箱: admin@xinwiki.com"
echo "  密码: admin123"
echo ""
echo "常用命令:"
echo "  查看日志: docker compose logs -f"
echo "  停止服务: docker compose down"
echo "  重启服务: docker compose restart"
echo ""

DEPLOYEOF
chmod +x "$PACKAGE_DIR/deploy.sh"

# 4.3 构建脚本（用于在 Linux 上构建缺失的镜像）
cp "$PROJECT_ROOT/scripts/build-offline-package.sh" "$PACKAGE_DIR/scripts/"

# 4.4 README
cat > "$PACKAGE_DIR/README.txt" << 'READMEEOF'
===================================
  XinWiki 离线部署包 (amd64)
===================================

1. 包含内容
-----------
- docker-compose.yml    Docker Compose 配置
- .env.example          环境变量模板
- config/               应用配置
- migrations/           数据库迁移
- docker/images/        Docker 镜像 tar 文件
- docker/Dockerfile.*   Dockerfile（用于本地构建）
- scripts/              工具脚本
- deploy.sh             一键部署脚本

2. 快速部署
-----------
  ./deploy.sh

3. 手动部署
-----------
  # 步骤 1: 加载镜像
  bash scripts/load-images.sh

  # 步骤 2: 配置环境
  cp .env.example .env
  vim .env
  # 必须配置: DB_PASSWORD, JWT_SECRET, 至少一个 LLM API Key

  # 步骤 3: 启动服务
  docker compose up -d                    # 最小化部署
  docker compose --profile full up -d     # 完整部署

4. 缺失镜像构建
---------------
  如果 xinwiki-app.tar 或 xinwiki-docreader.tar 不存在，
  需要在有网络的 Linux amd64 服务器上构建:

  # 方法一: 使用构建脚本
  bash scripts/build-offline-package.sh

  # 方法二: 手动构建
  docker build -t xinwiki/xinwiki-app:latest -f docker/Dockerfile.app .
  docker build -t xinwiki/xinwiki-docreader:latest -f docker/Dockerfile.docreader .
  docker build -t xinwiki/xinwiki-ui:latest -f docker/Dockerfile.ui frontend/
  docker build -t xinwiki/xinwiki-sandbox:latest -f docker/Dockerfile.sandbox .

5. 基础镜像
-----------
  基础镜像 (paradedb, redis, nginx 等) 需要从 GitHub Release 下载:
  https://github.com/tohnee/xinwiki-new/releases/tag/images-v0.1.0

  下载后放到 docker/images/ 目录，然后运行 load-images.sh 加载。

6. 访问
-------
  Web UI:  http://localhost
  API:     http://localhost:8080
  Swagger: http://localhost:8080/swagger/index.html
  默认账号: admin@xinwiki.com / admin123
READMEEOF

print_ok "部署脚本创建完成"

# ========== 5. 打包 ==========
print_info "========== 5/5 打包 =========="

OUTPUT_FILE="$AMD64_DIR/xinwiki-offline-deploy-amd64.tar.gz"

print_info "打包中..."
tar -czf "$OUTPUT_FILE" -C /tmp xinwiki-offline-deploy

print_ok "打包完成: $OUTPUT_FILE ($(du -h "$OUTPUT_FILE" | cut -f1))"

# 生成 SHA256
sha256sum "$OUTPUT_FILE" > "$OUTPUT_FILE.sha256"
print_ok "校验文件: $OUTPUT_FILE.sha256"

# 清理
rm -rf "$PACKAGE_DIR"

echo ""
print_ok "========== 全部完成 =========="
echo ""
print_info "输出文件:"
ls -lh "$OUTPUT_FILE" "$OUTPUT_FILE.sha256"
echo ""
print_info "下一步: 上传到 GitHub Release"
echo "  gh release upload images-v0.1.0 $OUTPUT_FILE $OUTPUT_FILE.sha256 --repo tohnee/xinwiki-new --clobber"
