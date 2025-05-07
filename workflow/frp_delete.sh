#!/bin/bash
# frp_delete.sh - Permanently delete a port rule in Tencent Cloud Security Group created by this workflow

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/utils.sh"

# Load config if exists
WORKFLOW_CONFIG_FILE="$HOME/.frp_workflow_config"
if [[ -f "$WORKFLOW_CONFIG_FILE" ]]; then
  source "$WORKFLOW_CONFIG_FILE"
fi

check_env_or_exit "SECURITY_GROUP_ID"
SECURITY_GROUP_ID="$SECURITY_GROUP_ID"
TCCLI_PATH="${TCCLI_PATH:-tccli}"
REGION="${REGION:-ap-guangzhou}" # 默认使用广州区域

# Get SecretId/SecretKey from Keychain
SECRET_ID=$(get_keychain_value "Alfred_TencentCloud_FRP_Keys" "TENCENTCLOUD_FRP_SECRET_ID")
SECRET_KEY=$(get_keychain_value "Alfred_TencentCloud_FRP_Keys" "TENCENTCLOUD_FRP_SECRET_KEY")
if [[ -z "$SECRET_ID" || -z "$SECRET_KEY" ]]; then
  echo "API 密钥未配置，请先运行 frp config setup_keys" >&2
  exit 1
fi

# 临时设置环境变量，用于 tccli
export TENCENTCLOUD_SECRET_ID="$SECRET_ID"
export TENCENTCLOUD_SECRET_KEY="$SECRET_KEY"

# Query current AlfredFRP_ rules
echo "正在查询安全组规则..."
RULES_JSON=$($TCCLI_PATH vpc DescribeSecurityGroupPolicies --region $REGION --profile default --SecurityGroupId "$SECURITY_GROUP_ID")

# 保存查询结果用于调试
echo "$RULES_JSON" > /tmp/sg_delete_debug.json

# 提取 AlfredFRP_ 开头的规则（不管是ACCEPT还是DENY）
if command -v jq >/dev/null 2>&1; then
  RULES=$(echo "$RULES_JSON" | jq -r '.SecurityGroupPolicySet.Ingress[] | select(.PolicyDescription | startswith("AlfredFRP_")) | "\(.PolicyDescription) \(.Protocol) \(.Port) \(.CidrBlock) \(.Action) Index:\(.PolicyIndex)"')
else
  # 如果没有jq，使用grep和简单的文本处理
  echo "未找到jq命令，将使用基本文本处理（可能不准确）" >&2
  RULES=$(echo "$RULES_JSON" | grep -A6 "\"PolicyDescription\": *\"AlfredFRP_" | grep -E "PolicyDescription|Protocol|Port|CidrBlock|Action|PolicyIndex" | paste -sd ' ' | sed 's/"PolicyDescription": *"\([^"]*\)".*"Protocol": *"\([^"]*\)".*"Port": *"\([^"]*\)".*"CidrBlock": *"\([^"]*\)".*"Action": *"\([^"]*\)".*"PolicyIndex": *\([0-9]*\).*/\1 \2 \3 \4 \5 Index:\6/g')
fi

if [[ -z "$RULES" ]]; then
  echo "未找到由 Workflow 创建的规则" >&2
  echo "提示: 查询结果保存在 /tmp/sg_delete_debug.json" >&2
  exit 1
fi

# 打印规则列表
echo "找到以下由Workflow创建的规则:"
echo "$RULES" | nl

# 显示警告信息
echo "警告: 删除操作将永久移除选中的规则，此操作不可撤销！" >&2
echo "如果只是想暂时禁用规则，请使用 frp_close.sh 脚本" >&2
echo ""

# 询问确认
read -p "是否确定要删除规则？(y/N): " confirm
if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
  echo "操作已取消" >&2
  exit 0
fi

# Alfred Script Filter: query argument as $1
QUERY="$1"

# 如果没有提供查询参数，使用fzf选择
if [[ -z "$QUERY" ]]; then
  if command -v fzf >/dev/null 2>&1; then
    echo "请选择要永久删除的规则:"
    SELECTED_RULE=$(echo "$RULES" | fzf --header="选择要永久删除的规则")
  else
    echo "未安装fzf工具，请提供规则编号(1-$(echo "$RULES" | wc -l | tr -d ' ')):" >&2
    read -r RULE_NUM
    SELECTED_RULE=$(echo "$RULES" | sed -n "${RULE_NUM}p")
  fi
else
  # 使用提供的查询参数
  if [[ "$QUERY" =~ ^[0-9]+$ ]] && [[ "$QUERY" -le $(echo "$RULES" | wc -l) ]]; then
    # 按数字选择
    SELECTED_RULE=$(echo "$RULES" | sed -n "${QUERY}p")
  else
    # 按名称查询，使用fzf过滤
    if command -v fzf >/dev/null 2>&1; then
      SELECTED_RULE=$(echo "$RULES" | fzf --query="$QUERY" --select-1 --exit-0)
    else
      # 简单匹配
      SELECTED_RULE=$(echo "$RULES" | grep -i "$QUERY" | head -1)
    fi
  fi
fi

if [[ -z "$SELECTED_RULE" ]]; then
  echo "未选择规则" >&2
  exit 1
fi

echo "选择的规则: $SELECTED_RULE"
SELECTED_INDEX=$(echo "$SELECTED_RULE" | sed -n 's/.*Index:\([0-9]*\)$/\1/p')
SELECTED_DESC=$(echo "$SELECTED_RULE" | awk '{print $1}')

# 再次确认
read -p "确定要永久删除规则 \"$SELECTED_DESC\"？(y/N): " final_confirm
if [[ ! "$final_confirm" =~ ^[Yy]$ ]]; then
  echo "操作已取消" >&2
  exit 0
fi

# 创建临时JSON文件
DELETE_JSON_FILE=$(mktemp)
cat > $DELETE_JSON_FILE <<EOF
{
  "SecurityGroupId": "$SECURITY_GROUP_ID",
  "SecurityGroupPolicySet": {
    "Ingress": [
      {
        "PolicyIndex": $SELECTED_INDEX
      }
    ]
  }
}
EOF

# Delete rule
echo "正在永久删除规则..."
DELETE_RESULT=$($TCCLI_PATH vpc DeleteSecurityGroupPolicies --region $REGION --profile default --cli-input-json "file://$DELETE_JSON_FILE" 2>&1)
DELETE_CODE=$?
rm -f "$DELETE_JSON_FILE"

if [ $DELETE_CODE -ne 0 ] || echo "$DELETE_RESULT" | grep -q 'Error'; then
  echo "删除规则失败: $DELETE_RESULT" >&2
  echo "提示: 查询结果保存在 /tmp/sg_delete_debug.json" >&2
  exit 1
fi

echo "已成功永久删除规则: $SELECTED_DESC"
echo "提示: 本次操作的调试日志保存在 /tmp/sg_delete_debug.json" 