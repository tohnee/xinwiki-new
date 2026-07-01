#!/bin/bash
# 上传所有 amd64 镜像文件到 GitHub Release
# 按从小到大顺序上传，便于观察进度
# 支持断点续传（gh release upload --clobber）

set +e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

REPO="tohnee/xinwiki-new"
TAG="images-v0.1.0"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AMD64_DIR="$(cd "$SCRIPT_DIR/../docker/images/amd64" && pwd)"
LOG_FILE="$AMD64_DIR/upload.log"

cd "$AMD64_DIR" || exit 1

# 已存在的文件（不重传）
EXISTING_ASSETS=$(gh release view "$TAG" --repo "$REPO" --json assets --jq '.assets[].name' 2>/dev/null)

# 收集所有要上传的文件，按大小排序
FILES=()
while IFS= read -r line; do
    f=$(echo "$line" | awk '{print $NF}')
    [ -n "$f" ] && [ "$f" != "." ] && FILES+=("$f")
done < <(find . -maxdepth 1 \( -name "*.tar" -o -name "SHA256SUMS" \) -print0 | xargs -0 ls -la 2>/dev/null | awk '{print $5"\t"$NF}' | sort -n)

TOTAL=${#FILES[@]}
UPLOADED=0
SKIPPED=0
FAILED=0
FAILED_FILES=()

# 打印函数
print_info()  { echo -e "${BLUE}[INFO]${NC}  $1" | tee -a "$LOG_FILE"; }
print_ok()    { echo -e "${GREEN}[OK]${NC}    $1" | tee -a "$LOG_FILE"; }
print_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1" | tee -a "$LOG_FILE"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1" | tee -a "$LOG_FILE"; }

echo "" | tee -a "$LOG_FILE"
echo "=========================================" | tee -a "$LOG_FILE"
echo "  开始上传 $(date)" | tee -a "$LOG_FILE"
echo "=========================================" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

for file in "${FILES[@]}"; do
    if [ -z "$file" ] || [ "$file" = "." ]; then continue; fi
    filename=$(basename "$file")
    size=$(ls -lh "$file" | awk '{print $5}')

    # 检查是否已存在
    if echo "$EXISTING_ASSETS" | grep -q "^${filename}$"; then
        print_warn "⊘ 跳过（已存在）: $filename ($size)"
        SKIPPED=$((SKIPPED + 1))
        continue
    fi

    print_info "↑ 上传 ($((UPLOADED + SKIPPED + 1))/$TOTAL): $filename ($size)"

    START_TIME=$(date +%s)
    if gh release upload "$TAG" "$file" --repo "$REPO" --clobber 2>&1 | tee -a "$LOG_FILE"; then
        END_TIME=$(date +%s)
        DURATION=$((END_TIME - START_TIME))
        UPLOADED=$((UPLOADED + 1))
        print_ok "✓ $filename (耗时 ${DURATION}s)"
    else
        END_TIME=$(date +%s)
        DURATION=$((END_TIME - START_TIME))
        FAILED=$((FAILED + 1))
        FAILED_FILES+=("$filename")
        print_error "✗ $filename 上传失败 (耗时 ${DURATION}s)"
    fi
    echo "" | tee -a "$LOG_FILE"
done

# 汇总
echo "" | tee -a "$LOG_FILE"
echo "=========================================" | tee -a "$LOG_FILE"
echo "  上传完成 $(date)" | tee -a "$LOG_FILE"
echo "=========================================" | tee -a "$LOG_FILE"
print_ok "成功: $UPLOADED 个"
print_warn "跳过（已存在）: $SKIPPED 个"
if [ $FAILED -gt 0 ]; then
    print_error "失败: $FAILED 个"
    for f in "${FAILED_FILES[@]}"; do
        echo "  - $f" | tee -a "$LOG_FILE"
    done
    print_info "重新运行此脚本可继续上传失败的文件（gh release upload --clobber 支持重传）"
fi
echo "" | tee -a "$LOG_FILE"
print_info "Release: https://github.com/$REPO/releases/tag/$TAG"
