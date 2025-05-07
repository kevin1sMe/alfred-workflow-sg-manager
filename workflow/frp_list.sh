#!/bin/bash
# frp_list.sh - List all security group rules created by this workflow

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/utils.sh"

check_env_or_exit "SECURITY_GROUP_ID"
TCCLI_PATH="${TCCLI_PATH:-tccli}"

# Get SecretId/SecretKey from Keychain
SECRET_ID=$(get_keychain_value "Alfred_TencentCloud_FRP_Keys" "TENCENTCLOUD_FRP_SECRET_ID")
SECRET_KEY=$(get_keychain_value "Alfred_TencentCloud_FRP_Keys" "TENCENTCLOUD_FRP_SECRET_KEY")
if [[ -z "$SECRET_ID" || -z "$SECRET_KEY" ]]; then
  echo "API 密钥未配置，请先运行 frp config setup_keys" >&2
  exit 1
fi

# Query current AlfredFRP_ rules
RULES=$($TCCLI_PATH vpc DescribeSecurityGroupPolicies --Version 2017-03-12 \
  --SecurityGroupId "$SECURITY_GROUP_ID" | jq -r '.SecurityGroupPolicySet.Ingress[] | select(.PolicyDescription | startswith("AlfredFRP_")) | "描述: \(.PolicyDescription) 协议: \(.Protocol) 端口: \(.Port) IP: \(.CidrBlock) Index: \(.PolicyIndex)"')

if [[ -z "$RULES" ]]; then
  echo "未找到由 Workflow 创建的规则" >&2
  exit 0
fi

echo "$RULES" 