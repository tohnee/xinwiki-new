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
echo "  XinWiki 镜像加载完成！"
echo "========================================="
echo ""
echo "已加载的镜像:"
docker images --format "table {{.Repository}}:{{.Tag}}\t{{.Size}}" | grep -E "xinwiki|paradedb|redis|nginx"
echo ""
echo "下一步:"
echo "  1. 复制项目代码到部署目录"
echo "  2. cp .env.example .env && vim .env (配置必要参数)"
echo "  3. docker compose up -d"
echo ""
