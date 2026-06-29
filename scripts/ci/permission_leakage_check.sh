#!/bin/bash
# CI 红线脚本：权限泄露检测
# 在 CI 流程中运行权限泄露测试，Leakage Rate 非 0 则退出
# 用法: bash scripts/ci/permission_leakage_check.sh

set -euo pipefail

echo "=========================================="
echo "  Permission Leakage CI Check"
echo "  红线: Permission Leakage Rate = 0"
echo "=========================================="

# 进入项目根目录
cd "$(dirname "$0")/../.."

# 1. 运行权限泄露评测单元测试
echo ""
echo "[1/3] Running permission leakage unit tests..."
if ! go test -v -count=1 -timeout 60s \
  ./internal/application/service/metric/... \
  -run "TestPermissionLeakage"; then
  echo "❌ FAIL: Permission leakage unit tests failed"
  exit 1
fi
echo "✅ PASS: Permission leakage unit tests"

# 2. 运行 ACL 传播测试（确保派生 ACL 计算正确）
echo ""
echo "[2/3] Running ACL propagation tests..."
if ! go test -v -count=1 -timeout 60s \
  ./internal/acl/... \
  -run "TestACL|TestCalculate|TestFilter|TestUser"; then
  echo "❌ FAIL: ACL propagation tests failed"
  exit 1
fi
echo "✅ PASS: ACL propagation tests"

# 3. 运行 ACL 重算测试（确保事件驱动重算正常）
echo ""
echo "[3/3] Running ACL recompute tests..."
if ! go test -v -count=1 -timeout 60s \
  ./internal/acl/... \
  -run "TestACLRecomputer"; then
  echo "❌ FAIL: ACL recompute tests failed"
  exit 1
fi
echo "✅ PASS: ACL recompute tests"

# 4. 运行语义缓存 ACL 测试（确保跨权限缓存命中 = 0）
echo ""
echo "[4/4] Running semantic cache ACL tests..."
if ! go test -v -count=1 -timeout 60s \
  ./internal/application/service/... \
  -run "TestSemanticCache_ACL"; then
  echo "❌ FAIL: Semantic cache ACL tests failed"
  exit 1
fi
echo "✅ PASS: Semantic cache ACL tests"

echo ""
echo "=========================================="
echo "  ✅ ALL PERMISSION LEAKAGE CHECKS PASSED"
echo "  Permission Leakage Rate = 0"
echo "=========================================="
