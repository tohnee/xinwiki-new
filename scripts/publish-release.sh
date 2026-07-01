#!/bin/bash
# 发布 amd64 离线镜像包到 GitHub Release
# 用法: ./scripts/publish-release.sh [tag_name] [repo]
#
# 默认:
#   tag_name = images-v0.1.0
#   repo     = tohnee/xinwiki-new (从 git remote 推断)

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
AMD64_DIR="$PROJECT_ROOT/docker/images/amd64"
TAG_NAME="${1:-images-v0.1.0}"
REPO="${2:-}"

cd "$PROJECT_ROOT"

print_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
print_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }
print_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 推断仓库名
if [ -z "$REPO" ]; then
    REPO=$(git remote get-url origin 2>/dev/null | sed -E 's#.*github.com[:/]([^/]+/[^.]+)(\.git)?$#\1#')
    if [ -z "$REPO" ]; then
        print_error "无法推断仓库名，请通过参数指定: $0 <tag> <owner/repo>"
        exit 1
    fi
fi

print_info "目标仓库: $REPO"
print_info "Release Tag: $TAG_NAME"
print_info "镜像目录: $AMD64_DIR"
echo ""

# ========== 1. 前置检查 ==========
print_info "========== 1/4 前置检查 =========="

if ! command -v gh &>/dev/null; then
    print_error "gh CLI 未安装"
    print_info "安装: brew install gh"
    exit 1
fi
print_ok "gh CLI 可用: $(gh --version | head -1)"

if ! gh auth status &>/dev/null; then
    print_error "gh CLI 未登录"
    print_info "请先登录: gh auth login"
    exit 1
fi
print_ok "gh CLI 已登录: $(gh auth status 2>&1 | grep -i 'account' | head -1)"

if [ ! -d "$AMD64_DIR" ]; then
    print_error "镜像目录不存在: $AMD64_DIR"
    exit 1
fi

cd "$AMD64_DIR"
TAR_COUNT=$(ls -1 *.tar 2>/dev/null | wc -l | tr -d ' ')
print_ok "找到 $TAR_COUNT 个 tar 文件"
echo ""

# ========== 2. 检查 Release 是否已存在 ==========
print_info "========== 2/4 检查 Release =========="

if gh release view "$TAG_NAME" --repo "$REPO" &>/dev/null; then
    print_warn "Release $TAG_NAME 已存在，将上传文件到该 release"
    CREATE_NEW=0
else
    print_info "Release $TAG_NAME 不存在，将创建新 release"
    CREATE_NEW=1
fi
echo ""

# ========== 3. 生成 Release Notes ==========
print_info "========== 3/4 准备 Release Notes =========="

RELEASE_NOTES="$AMD64_DIR/../RELEASE_NOTES_${TAG_NAME}.md"

cat > "$RELEASE_NOTES" << 'NOTESEOF'
# XinWiki 离线镜像包 (amd64)

适用于 Linux amd64 架构的 XinWiki 离线部署镜像。

## 包含内容

### 基础设施镜像（运行时必需）
- `xinwiki-infra-amd64.tar` (509MB) - paradedb, redis
- `xinwiki-build-base-amd64.tar` (820MB) - golang, python, node, debian, nginx, uv

### 可选组件镜像
- `xinwiki-optional-amd64.tar` (1.3GB) - minio, neo4j, qdrant, searxng, clickhouse, langfuse, langfuse-worker

### 单镜像（可选单独下载）
- `clickhouse.tar` (171MB)
- `debian.tar` (27MB)
- `golang.tar` (283MB)
- `langfuse.tar` (280MB)
- `langfuse-worker.tar` (313MB)
- `minio.tar` (59MB)
- `neo4j.tar` (383MB)
- `nginx.tar` (25MB)
- `node.tar` (68MB)
- `paradedb.tar` (496MB)
- `python310.tar` (359MB)
- `python311.tar` (43MB)
- `qdrant.tar` (68MB)
- `redis.tar` (13MB)
- `searxng.tar` (93MB)
- `uv.tar` (16MB)
- `SHA256SUMS` - 校验文件

**总大小: 约 5.2 GB**

## 使用方法

```bash
# 1. 下载需要的镜像包和 SHA256SUMS
wget https://github.com/REPLACE_REPO/releases/download/images-v0.1.0/xinwiki-infra-amd64.tar
wget https://github.com/REPLACE_REPO/releases/download/images-v0.1.0/xinwiki-build-base-amd64.tar
wget https://github.com/REPLACE_REPO/releases/download/images-v0.1.0/SHA256SUMS

# 2. 校验
sha256sum -c SHA256SUMS  # 或者只校验下载的包

# 3. 加载镜像
docker load -i xinwiki-infra-amd64.tar
docker load -i xinwiki-build-base-amd64.tar

# 4. 返回项目根目录启动
cd ../..
cp .env.example .env
vim .env
docker compose up -d
```

## 验证

下载后请使用 `SHA256SUMS` 文件校验完整性：

```bash
sha256sum -c SHA256SUMS
```

## 架构说明

- 目标平台: **linux/amd64**
- 适用于大多数生产环境服务器
- 不适用于 ARM64/Mac Apple Silicon（如果是 ARM64，请用源码 buildx 构建）
NOTESEOF

# 替换仓库占位符
sed -i '' "s/REPLACE_REPO/${REPO//\//\\/}/g" "$RELEASE_NOTES" 2>/dev/null || sed -i "s/REPLACE_REPO/${REPO//\//\\/}/g" "$RELEASE_NOTES"

print_ok "Release notes: $RELEASE_NOTES"
echo ""

# ========== 4. 创建 Release 并上传 ==========
print_info "========== 4/4 创建/上传 Release =========="

if [ $CREATE_NEW -eq 1 ]; then
    print_info "创建 Release: $TAG_NAME"
    print_warn "这将开始上传约 5.2 GB 文件，请确保网络稳定（可能需要 30 分钟 - 数小时）"
    echo ""
    read -p "继续？(y/N): " confirm
    if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
        print_info "已取消"
        exit 0
    fi
    
    gh release create "$TAG_NAME" \
        --repo "$REPO" \
        --title "XinWiki 离线镜像包 (amd64)" \
        --notes-file "$RELEASE_NOTES" \
        --target master
    print_ok "Release 创建成功"
else
    print_info "上传文件到已存在的 Release"
    read -p "继续？(y/N): " confirm
    if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
        print_info "已取消"
        exit 0
    fi
fi

# 上传所有 tar 文件和 SHA256SUMS
print_info "开始上传文件（这将需要较长时间）..."

cd "$AMD64_DIR"
FAILED_UPLOADS=()
UPLOADED=()

for tar_file in *.tar SHA256SUMS; do
    if [ -f "$tar_file" ]; then
        size=$(ls -lh "$tar_file" | awk '{print $5}')
        print_info "上传 $tar_file ($size) ..."
        
        if gh release upload "$TAG_NAME" "$tar_file" --repo "$REPO" --clobber 2>&1; then
            print_ok "✓ $tar_file"
            UPLOADED+=("$tar_file")
        else
            print_error "✗ $tar_file 上传失败"
            FAILED_UPLOADS+=("$tar_file")
        fi
        echo ""
    fi
done

# ========== 结果汇总 ==========
print_info "========== 完成 =========="
print_ok "成功上传: ${#UPLOADED[@]} 个文件"

if [ ${#FAILED_UPLOADS[@]} -gt 0 ]; then
    print_error "失败: ${#FAILED_UPLOADS[@]} 个"
    for f in "${FAILED_UPLOADS[@]}"; do
        echo "  - $f"
    done
    print_info "可以重新运行本脚本继续上传失败的文件（gh release upload 支持断点续传）"
fi

echo ""
print_info "Release 页面: https://github.com/$REPO/releases/tag/$TAG_NAME"
print_info ""
print_info "如果上传中断，可手动继续上传:"
echo "  cd $AMD64_DIR"
for f in *.tar SHA256SUMS; do
    echo "  gh release upload $TAG_NAME $f --repo $REPO --clobber"
done
