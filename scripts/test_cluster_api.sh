#!/bin/bash

set -euo pipefail

# SkyTree 管理面 API 连通性检查

BASE_URL="${BASE_URL:-http://localhost:9526}"
API_V1="${BASE_URL}/api/v1"
ACL_USER="${ACL_USER:-}"
ACL_PASS="${ACL_PASS:-}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}=== SkyTree API Smoke Test ===${NC}"
echo -e "${YELLOW}BASE_URL: ${BASE_URL}${NC}"

echo -e "\n${YELLOW}1. /health${NC}"
curl -sS "${BASE_URL}/health"
echo

echo -e "\n${YELLOW}2. /metrics (head)${NC}"
curl -sSI "${BASE_URL}/metrics" | head -n 5

if [[ -n "${ACL_USER}" && -n "${ACL_PASS}" ]]; then
  echo -e "\n${YELLOW}3. /api/v1/acl/ruleset (with Basic Auth)${NC}"
  curl -sS -u "${ACL_USER}:${ACL_PASS}" "${API_V1}/acl/ruleset"
  echo
else
  echo -e "\n${YELLOW}3. 跳过 ACL API（未设置 ACL_USER/ACL_PASS）${NC}"
  echo "   如果 ACL 管理路由已启用，请设置环境变量后重试："
  echo "   ACL_USER=... ACL_PASS=... BASE_URL=${BASE_URL} ./scripts/test_cluster_api.sh"
fi

echo -e "\n${GREEN}=== 完成 ===${NC}"
