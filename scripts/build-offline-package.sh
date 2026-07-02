#!/bin/bash
# ==============================================================================
# XinWiki 离线部署包构建脚本
# ==============================================================================
#
# 用途:
#   在 Linux amd64 服务器上运行，构建 XinWiki 项目所有自定义 Docker 镜像，
#   拉取第三方基础镜像，并打包成完整的离线部署包
#   (xinwiki-offline-deploy-amd64.tar.gz)。
#
# 用法:
#   ./scripts/build-offline-package.sh              # 构建所有镜像并打包
#   ./scripts/build-offline-package.sh --ui          # 仅构建前端镜像
#   ./scripts/build-offline-package.sh --app         # 仅构建后端镜像
#   ./scripts/build-offline-package.sh --docreader   # 仅构建文档解析镜像
#   ./scripts/build-offline-package.sh --sandbox     # 仅构建沙箱镜像
#   ./scripts/build-offline-package.sh --ui --app    # 构建指定多个镜像
#   ./scripts/build-offline-package.sh --skip-frontend-build  # 跳过前端 dist 构建
#   ./scripts/build-offline-package.sh --skip-pull-base       # 跳过拉取第三方基础镜像
#   ./scripts/build-offline-package.sh --apt-mirror http://mirrors.tuna.tsinghua.edu.cn  # 指定 docreader apt 镜像
#   ./scripts/build-offline-package.sh --help        # 显示帮助
#
# 环境要求:
#   - Linux amd64 (x86_64)
#   - Docker 已安装并运行
#   - Git (用于获取 commit ID)
#   - Node.js + npm (构建前端静态资源，除非使用 --skip-frontend-build)
#   - Go (用于获取 Go 版本信息)
#
# 输出:
#   - docker/images/amd64/xinwiki-ui.tar
#   - docker/images/amd64/xinwiki-app.tar
#   - docker/images/amd64/xinwiki-docreader.tar
#   - docker/images/amd64/xinwiki-sandbox.tar
#   - docker/images/amd64/paradedb.tar
#   - docker/images/amd64/redis.tar
#   - docker/images/amd64/nginx.tar
#   - docker/images/amd64/SHA256SUMS
#   - xinwiki-offline-deploy-amd64.tar.gz (最终离线部署包)
#
# ==============================================================================

set -e

# ------------------------------------------------------------------------------
# 颜色定义
# ------------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # 无颜色

# ------------------------------------------------------------------------------
# 路径变量
# ------------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
IMAGE_OUTPUT_DIR="$PROJECT_ROOT/docker/images/amd64"

cd "$PROJECT_ROOT"

# ------------------------------------------------------------------------------
# 日志函数
# ------------------------------------------------------------------------------
print_info()    { echo -e "${BLUE}[INFO]${NC}  $1"; }
print_ok()      { echo -e "${GREEN}[OK]${NC}    $1"; }
print_warn()    { echo -e "${YELLOW}[WARN]${NC}  $1"; }
print_error()   { echo -e "${RED}[ERROR]${NC} $1"; }
print_step()    { echo -e "${BLUE}========== $1 ==========${NC}"; }

# ------------------------------------------------------------------------------
# 帮助信息
# ------------------------------------------------------------------------------
show_help() {
    echo "XinWiki 离线部署包构建脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  --ui              仅构建前端镜像 (xinwiki-ui)"
    echo "  --app             仅构建后端镜像 (xinwiki-app)"
    echo "  --docreader       仅构建文档解析镜像 (xinwiki-docreader)"
    echo "  --sandbox         仅构建沙箱镜像 (xinwiki-sandbox)"
    echo "  --skip-frontend-build  跳过前端 dist 构建 (假设 frontend/dist 已存在)"
    echo "  --skip-pull-base  跳过拉取第三方基础镜像 (paradedb, redis, nginx)"
    echo "  --apt-mirror URL  指定 docreader 构建时的 apt 镜像源"
    echo "  -h, --help        显示此帮助信息"
    echo ""
    echo "不带参数时默认构建所有镜像、拉取第三方基础镜像并打包离线部署包。"
    exit 0
}

# ------------------------------------------------------------------------------
# 参数解析
# ------------------------------------------------------------------------------
BUILD_UI=false
BUILD_APP=false
BUILD_DOCREADER=false
BUILD_SANDBOX=false
BUILD_ALL=true
SKIP_FRONTEND_BUILD=false
SKIP_PULL_BASE=false
APT_MIRROR_VAL=""

if [ $# -gt 0 ]; then
    BUILD_ALL=true
    HAS_IMAGE_FILTER=false
    while [ "$1" != "" ]; do
        case $1 in
            --ui )               BUILD_UI=true; HAS_IMAGE_FILTER=true ;;
            --app )              BUILD_APP=true; HAS_IMAGE_FILTER=true ;;
            --docreader )        BUILD_DOCREADER=true; HAS_IMAGE_FILTER=true ;;
            --sandbox )          BUILD_SANDBOX=true; HAS_IMAGE_FILTER=true ;;
            --skip-frontend-build ) SKIP_FRONTEND_BUILD=true ;;
            --skip-pull-base )   SKIP_PULL_BASE=true ;;
            --apt-mirror )       shift; APT_MIRROR_VAL="$1" ;;
            -h | --help )        show_help ;;
            * )
                print_error "未知选项: $1"
                show_help
                ;;
        esac
        shift
    done
    # 如果指定了任何镜像过滤参数，则不构建全部
    if [ "$HAS_IMAGE_FILTER" = true ]; then
        BUILD_ALL=false
    fi
fi

# ------------------------------------------------------------------------------
# 镜像定义
# ------------------------------------------------------------------------------
# 自定义镜像: tar 文件名 -> docker image tag
declare -A IMAGE_TARS
IMAGE_TARS=(
    ["ui"]="xinwiki/xinwiki-ui:latest"
    ["app"]="xinwiki/xinwiki-app:latest"
    ["docreader"]="xinwiki/xinwiki-docreader:latest"
    ["sandbox"]="xinwiki/xinwiki-sandbox:latest"
)

declare -A TAR_FILES
TAR_FILES=(
    ["ui"]="xinwiki-ui.tar"
    ["app"]="xinwiki-app.tar"
    ["docreader"]="xinwiki-docreader.tar"
    ["sandbox"]="xinwiki-sandbox.tar"
)

# 第三方基础镜像: tar 文件名 -> docker image tag
declare -A BASE_IMAGES
BASE_IMAGES=(
    ["paradedb.tar"]="paradedb/paradedb:v0.22.2-pg17"
    ["redis.tar"]="redis:7.0-alpine"
    ["nginx.tar"]="nginx:stable-alpine"
)

# ------------------------------------------------------------------------------
# 环境检查
# ------------------------------------------------------------------------------
print_step "步骤 0: 环境检查"

if ! command -v docker &> /dev/null; then
    print_error "未安装 Docker，请先安装 Docker"
    exit 1
fi

if ! docker info &> /dev/null; then
    print_error "Docker 服务未运行，请启动 Docker 服务"
    exit 1
fi
print_ok "Docker 环境检查通过"

ARCH=$(uname -m)
if [ "$ARCH" != "x86_64" ]; then
    print_warn "当前架构为 $ARCH，本脚本专为 linux/amd64 设计，继续构建可能遇到问题"
fi
print_info "当前架构: $ARCH"

# ------------------------------------------------------------------------------
# 获取版本信息
# ------------------------------------------------------------------------------
print_info "获取版本信息..."

if [ -f "$PROJECT_ROOT/VERSION" ]; then
    VERSION=$(tr -d '\n\r' < "$PROJECT_ROOT/VERSION")
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

print_info "版本: $VERSION"
print_info "Commit ID: $COMMIT_ID"
print_info "构建时间: $BUILD_TIME"
print_info "Go 版本: $GO_VERSION"

# ------------------------------------------------------------------------------
# 构建结果跟踪
# ------------------------------------------------------------------------------
declare -A BUILD_RESULTS
declare -A PULL_RESULTS
FAILED_COUNT=0

record_result() {
    local name="$1"
    local result="$2"
    BUILD_RESULTS["$name"]="$result"
    if [ "$result" != "0" ]; then
        FAILED_COUNT=$((FAILED_COUNT + 1))
    fi
}

record_pull_result() {
    local name="$1"
    local result="$2"
    PULL_RESULTS["$name"]="$result"
    if [ "$result" != "0" ]; then
        FAILED_COUNT=$((FAILED_COUNT + 1))
    fi
}

# ------------------------------------------------------------------------------
# 创建输出目录
# ------------------------------------------------------------------------------
mkdir -p "$IMAGE_OUTPUT_DIR"

# ==============================================================================
# Step 1: Build custom images
# ==============================================================================
print_step "Step 1: Build custom images"

# --- 1.1 Build frontend image ---
build_ui() {
    print_info "Building frontend image (xinwiki/xinwiki-ui:latest)..."
    if [ "$SKIP_FRONTEND_BUILD" = false ]; then
        print_info "Building frontend static assets (dist)..."
        if [ ! -f "$PROJECT_ROOT/frontend/package.json" ]; then
            print_error "frontend/package.json not found"
            record_result "ui" 1
            return
        fi
        cd "$PROJECT_ROOT/frontend"
        if ! npm ci; then
            print_error "npm ci failed"
            record_result "ui" 1
            cd "$PROJECT_ROOT"
            return
        fi
        if ! npm run build; then
            print_error "npm run build failed"
            record_result "ui" 1
            cd "$PROJECT_ROOT"
            return
        fi
        cd "$PROJECT_ROOT"
        print_ok "Frontend static assets built"
    else
        print_warn "Skipping frontend dist build (--skip-frontend-build)"
        if [ ! -d "$PROJECT_ROOT/frontend/dist" ]; then
            print_error "frontend/dist directory not found"
            record_result "ui" 1
            return
        fi
    fi
    if docker build \
        -t xinwiki/xinwiki-ui:latest \
        -f frontend/Dockerfile \
        frontend/ ; then
        print_ok "Frontend image built successfully"
        record_result "ui" 0
    else
        print_error "Frontend image build failed"
        record_result "ui" 1
    fi
}

# --- 1.2 Build backend image ---
build_app() {
    print_info "Building backend image (xinwiki/xinwiki-app:latest)..."
    local dockerfile="$PROJECT_ROOT/docker/Dockerfile.app"
    if [ ! -f "$dockerfile" ]; then
        print_error "Dockerfile not found: $dockerfile"
        record_result "app" 1
        return
    fi
    if docker build \
        --build-arg GOPROXY_ARG=https://goproxy.cn,direct \
        --build-arg GOSUMDB_ARG=off \
        --build-arg VERSION_ARG="$VERSION" \
        --build-arg COMMIT_ID_ARG="$COMMIT_ID" \
        --build-arg BUILD_TIME_ARG="$BUILD_TIME" \
        --build-arg GO_VERSION_ARG="$GO_VERSION" \
        -f docker/Dockerfile.app \
        -t xinwiki/xinwiki-app:latest \
        . ; then
        print_ok "Backend image built successfully"
        record_result "app" 0
    else
        print_error "Backend image build failed"
        record_result "app" 1
    fi
}

# --- 1.3 Build docreader image ---
build_docreader() {
    print_info "Building docreader image (xinwiki/xinwiki-docreader:latest)..."
    local dockerfile="$PROJECT_ROOT/docker/Dockerfile.docreader"
    if [ ! -f "$dockerfile" ]; then
        print_error "Dockerfile not found: $dockerfile"
        record_result "docreader" 1
        return
    fi
    local build_args=()
    build_args+=(--build-arg TARGETARCH=amd64)
    if [ -n "$APT_MIRROR_VAL" ]; then
        build_args+=(--build-arg APT_MIRROR="$APT_MIRROR_VAL")
        print_info "Using apt mirror: $APT_MIRROR_VAL"
    fi
    if docker build \
        "${build_args[@]}" \
        -f docker/Dockerfile.docreader \
        -t xinwiki/xinwiki-docreader:latest \
        . ; then
        print_ok "Docreader image built successfully"
        record_result "docreader" 0
    else
        print_error "Docreader image build failed"
        record_result "docreader" 1
    fi
}

# --- 1.4 Build sandbox image ---
build_sandbox() {
    print_info "Building sandbox image (xinwiki/xinwiki-sandbox:latest)..."
    local dockerfile="$PROJECT_ROOT/docker/Dockerfile.sandbox"
    if [ ! -f "$dockerfile" ]; then
        print_error "Dockerfile not found: $dockerfile"
        record_result "sandbox" 1
        return
    fi
    if docker build \
        -f docker/Dockerfile.sandbox \
        -t xinwiki/xinwiki-sandbox:latest \
        . ; then
        print_ok "Sandbox image built successfully"
        record_result "sandbox" 0
    else
        print_error "Sandbox image build failed"
        record_result "sandbox" 1
    fi
}

# Execute builds
if [ "$BUILD_ALL" = true ] || [ "$BUILD_UI" = true ]; then
    build_ui
fi
if [ "$BUILD_ALL" = true ] || [ "$BUILD_APP" = true ]; then
    build_app
fi
if [ "$BUILD_ALL" = true ] || [ "$BUILD_DOCREADER" = true ]; then
    build_docreader
fi
if [ "$BUILD_ALL" = true ] || [ "$BUILD_SANDBOX" = true ]; then
    build_sandbox
fi

# Build results summary
echo ""
print_step "Build Results Summary"
for name in ui app docreader sandbox; do
    result="${BUILD_RESULTS[$name]:-skipped}"
    if [ "$result" = "0" ]; then
        print_ok "xinwiki-$name: build succeeded"
    elif [ "$result" = "skipped" ]; then
        print_warn "xinwiki-$name: skipped"
    else
        print_error "xinwiki-$name: build failed"
    fi
done
if [ $FAILED_COUNT -gt 0 ]; then
    print_warn "$FAILED_COUNT image(s) failed, continuing"
fi

# ==============================================================================
# Step 2: Pull third-party base images
# ==============================================================================
print_step "Step 2: Pull third-party base images"

if [ "$SKIP_PULL_BASE" = true ]; then
    print_warn "Skipping base image pull (--skip-pull-base)"
    for tn in "${!BASE_IMAGES[@]}"; do
        record_pull_result "$tn" "skipped"
    done
else
    for tn in "${!BASE_IMAGES[@]}"; do
        img="${BASE_IMAGES[$tn]}"
        print_info "Pulling $img ..."
        if docker pull "$img"; then
            print_ok "Pulled: $img"
            record_pull_result "$tn" 0
        else
            print_error "Pull failed: $img"
            record_pull_result "$tn" 1
        fi
    done
    echo ""
    print_info "Base image pull results:"
    for tn in "${!BASE_IMAGES[@]}"; do
        result="${PULL_RESULTS[$tn]:-skipped}"
        img="${BASE_IMAGES[$tn]}"
        if [ "$result" = "0" ]; then
            print_ok "$img: pulled"
        elif [ "$result" = "skipped" ]; then
            print_warn "$img: skipped"
        else
            print_error "$img: failed"
        fi
    done
fi

# ==============================================================================
# Step 3: Save all images as tar files
# ==============================================================================
print_step "Step 3: Save all images as tar files"

SAVED_COUNT=0

# Save custom images
for name in ui app docreader sandbox; do
    image="${IMAGE_TARS[$name]}"
    tar_name="${TAR_FILES[$name]}"
    tar_path="$IMAGE_OUTPUT_DIR/$tar_name"
    result="${BUILD_RESULTS[$name]:-skipped}"
    if [ "$result" != "0" ]; then
        print_warn "Skip saving $image (not built or failed)"
        continue
    fi
    if ! docker image inspect "$image" &>/dev/null; then
        print_warn "Image not found: $image, skip"
        continue
    fi
    print_info "Saving $image -> $tar_name ..."
    if docker save -o "$tar_path" "$image"; then
        local_size=$(du -h "$tar_path" | cut -f1)
        print_ok "Saved: $tar_name ($local_size)"
        SAVED_COUNT=$((SAVED_COUNT + 1))
    else
        print_error "Save failed: $tar_name"
    fi
done

# Save base images
if [ "$SKIP_PULL_BASE" = false ]; then
    for tn in "${!BASE_IMAGES[@]}"; do
        img="${BASE_IMAGES[$tn]}"
        tp="$IMAGE_OUTPUT_DIR/$tn"
        result="${PULL_RESULTS[$tn]:-skipped}"
        if [ "$result" != "0" ]; then
            print_warn "Skip saving $img (not pulled or failed)"
            continue
        fi
        if ! docker image inspect "$img" &>/dev/null; then
            print_warn "Image not found: $img, skip"
            continue
        fi
        print_info "Saving $img -> $tn ..."
        if docker save -o "$tp" "$img"; then
            local_size=$(du -h "$tp" | cut -f1)
            print_ok "Saved: $tn ($local_size)"
            SAVED_COUNT=$((SAVED_COUNT + 1))
        else
            print_error "Save failed: $tn"
        fi
    done
fi

if [ $SAVED_COUNT -eq 0 ]; then
    print_error "No images to save, aborting"
    exit 1
fi
print_info "Saved $SAVED_COUNT image tar files to $IMAGE_OUTPUT_DIR"

# ==============================================================================
# Step 4: Generate SHA256 checksums
# ==============================================================================
print_step "Step 4: Generate SHA256 checksums"

SHA256_FILE="$IMAGE_OUTPUT_DIR/SHA256SUMS"
print_info "Generating: SHA256SUMS"
cd "$IMAGE_OUTPUT_DIR"
> "$SHA256_FILE"
for name in ui app docreader sandbox; do
    tar_name="${TAR_FILES[$name]}"
    if [ -f "$tar_name" ]; then
        sha256sum "$tar_name" >> "$SHA256_FILE"
    fi
done
for tn in "${!BASE_IMAGES[@]}"; do
    if [ -f "$tn" ]; then
        sha256sum "$tn" >> "$SHA256_FILE"
    fi
done
cd "$PROJECT_ROOT"
if [ -s "$SHA256_FILE" ]; then
    print_ok "SHA256SUMS generated: $SHA256_FILE"
else
    print_error "SHA256SUMS is empty"
    exit 1
fi

# ==============================================================================
# Step 5: Create offline deploy package
# ==============================================================================
print_step "Step 5: Create offline deploy package"

STAGING_DIR="$PROJECT_ROOT/dist/offline-deploy-amd64"
PACKAGE_NAME="xinwiki-offline-deploy-amd64"
FINAL_PACKAGE="$PROJECT_ROOT/${PACKAGE_NAME}.tar.gz"

if [ -d "$STAGING_DIR" ]; then
    print_info "Cleaning old staging dir..."
    rm -rf "$STAGING_DIR"
fi
mkdir -p "$STAGING_DIR"

# Copy project files
print_info "Copying project files..."
if [ -f "$PROJECT_ROOT/docker-compose.yml" ]; then
    cp "$PROJECT_ROOT/docker-compose.yml" "$STAGING_DIR/"
    print_ok "Copied docker-compose.yml"
else
    print_error "docker-compose.yml not found"
fi
if [ -f "$PROJECT_ROOT/.env.example" ]; then
    cp "$PROJECT_ROOT/.env.example" "$STAGING_DIR/"
    print_ok "Copied .env.example"
fi
if [ -d "$PROJECT_ROOT/config" ]; then
    cp -r "$PROJECT_ROOT/config" "$STAGING_DIR/"
    print_ok "Copied config/"
fi
if [ -d "$PROJECT_ROOT/migrations" ]; then
    cp -r "$PROJECT_ROOT/migrations" "$STAGING_DIR/"
    print_ok "Copied migrations/"
fi
if [ -d "$PROJECT_ROOT/docker/searxng" ]; then
    mkdir -p "$STAGING_DIR/docker/searxng"
    cp -r "$PROJECT_ROOT/docker/searxng/"* "$STAGING_DIR/docker/searxng/" 2>/dev/null || true
    print_ok "Copied docker/searxng/"
fi

# Copy image tar files
mkdir -p "$STAGING_DIR/docker/images/amd64"
for name in ui app docreader sandbox; do
    tar_name="${TAR_FILES[$name]}"
    tar_path="$IMAGE_OUTPUT_DIR/$tar_name"
    if [ -f "$tar_path" ]; then
        cp "$tar_path" "$STAGING_DIR/docker/images/amd64/"
    fi
done
for tn in "${!BASE_IMAGES[@]}"; do
    tp="$IMAGE_OUTPUT_DIR/$tn"
    if [ -f "$tp" ]; then
        cp "$tp" "$STAGING_DIR/docker/images/amd64/"
    fi
done
cp "$SHA256_FILE" "$STAGING_DIR/docker/images/amd64/"
print_ok "Copied image tar files and SHA256SUMS"
if [ -f "$PROJECT_ROOT/VERSION" ]; then
    cp "$PROJECT_ROOT/VERSION" "$STAGING_DIR/"
fi

# Generate load-images.sh
print_info "Generating load-images.sh..."
LOAD_SCRIPT="$STAGING_DIR/scripts/load-images.sh"
mkdir -p "$STAGING_DIR/scripts"
cat > "$LOAD_SCRIPT" << 'LOADEOF'
#!/bin/bash
set -e
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
print_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
print_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
print_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }
LOADEOF
cat >> "$LOAD_SCRIPT" << 'L2'
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
IMAGE_DIR="$PROJECT_ROOT/docker/images/amd64"
SHA256_FILE="$IMAGE_DIR/SHA256SUMS"

if [ ! -d "$IMAGE_DIR" ]; then
    print_error "Image directory not found: $IMAGE_DIR"
    exit 1
fi

# Verify SHA256
if [ -f "$SHA256_FILE" ]; then
    print_info "Verifying image integrity..."
    (cd "$IMAGE_DIR" && sha256sum -c SHA256SUMS) || {
        print_error "Checksum verification failed"
        read -p "Continue anyway? (y/N): " confirm
        [ "$confirm" != "y" ] && exit 1
    }
    print_ok "Checksum verification passed"
else
    print_warn "SHA256SUMS not found, skipping verification"
fi
L2
cat >> "$DEPLOY_SCRIPT" << 'D2'
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo ""
echo "========================================="
echo "  XinWiki Offline Deploy"
echo "========================================="
echo ""

# Check Docker
print_info "Checking Docker..."
if ! command -v docker &> /dev/null; then
    print_error "Docker not installed"
    exit 1
fi
if ! docker info &> /dev/null; then
    print_error "Docker service not running"
    exit 1
fi
print_ok "Docker check passed"
if ! command -v docker compose &> /dev/null; then
    print_error "docker compose not available"
    exit 1
fi
print_ok "Docker Compose check passed"

# Load images
print_info "Loading offline Docker images..."
bash "$SCRIPT_DIR/scripts/load-images.sh"
D2

# Generate offline-install.sh
print_info "Generating offline-install.sh..."
INSTALL_SCRIPT="$STAGING_DIR/offline-install.sh"
cat > "$INSTALL_SCRIPT" << 'INSTEOF'
#!/bin/bash
set -e
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
print_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
print_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
print_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

RELEASE_URL="https://github.com/tohnee/xinwiki-new/releases/download/images-v0.1.0"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE_DIR="$SCRIPT_DIR/docker/images/amd64"
MAX_RETRIES=3
INSTEOF
