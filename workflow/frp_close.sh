#!/bin/bash
# frp_close.sh - Close a port in Tencent Cloud Security Group opened by this workflow

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

# 创建临时调试日志
DEBUG_LOG="/tmp/sg_close_debug.log"
echo "=================== $(date) ===================" > "$DEBUG_LOG"

# 检查tccli命令是否存在
if ! command -v $TCCLI_PATH &> /dev/null; then
    echo "错误: $TCCLI_PATH 命令未找到，请确保腾讯云CLI已安装" >&2
    echo "可以运行以下命令安装: pip install tccli" >&2
    exit 1
fi

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
QUERY_CMD="$TCCLI_PATH vpc DescribeSecurityGroupPolicies --region $REGION --profile default --SecurityGroupId $SECURITY_GROUP_ID"

RULES_JSON=$($QUERY_CMD 2>&1)
QUERY_CODE=$?

if [ $QUERY_CODE -ne 0 ]; then
  echo "查询安全组规则失败: $RULES_JSON" >&2
  echo "请检查您的API密钥和权限设置" >&2
  exit 1
fi

# 检查JSON格式是否有效
if ! echo "$RULES_JSON" | grep -q "SecurityGroupPolicySet"; then
  echo "安全组规则查询返回无效数据，无法解析" >&2
  echo "返回数据: $RULES_JSON" >&2
  exit 1
fi

# 提取 AlfredFRP_ 开头的规则，只选择ACCEPT状态的规则
if command -v jq >/dev/null 2>&1; then
  RULES=$(echo "$RULES_JSON" | jq -r '.SecurityGroupPolicySet.Ingress[] | select(.PolicyDescription | startswith("AlfredFRP_")) | select(.Action=="ACCEPT") | "\(.PolicyDescription) \(.Protocol) \(.Port) \(.CidrBlock) \(.Action) Index:\(.PolicyIndex)"')
else
  # 如果没有jq，使用grep和简单的文本处理
  echo "未找到jq命令，将使用基本文本处理（可能不准确）" >&2
  RULES=$(echo "$RULES_JSON" | grep -A6 "\"PolicyDescription\": *\"AlfredFRP_" | grep -A5 "\"Action\": *\"ACCEPT\"" | grep -E "PolicyDescription|Protocol|Port|CidrBlock|Action|PolicyIndex" | paste -sd ' ' | sed 's/"PolicyDescription": *"\([^"]*\)".*"Protocol": *"\([^"]*\)".*"Port": *"\([^"]*\)".*"CidrBlock": *"\([^"]*\)".*"Action": *"\([^"]*\)".*"PolicyIndex": *\([0-9]*\).*/\1 \2 \3 \4 \5 Index:\6/g')
fi

if [[ -z "$RULES" ]]; then
  echo "未找到由 Workflow 创建且状态为允许的规则" >&2
  exit 1
fi

# 打印规则列表
echo "找到以下由Workflow创建且状态为允许的规则:"
echo "$RULES" | nl

# Alfred Script Filter: query argument as $1
QUERY="$1"

# 如果没有提供查询参数，使用fzf选择
if [[ -z "$QUERY" ]]; then
  if command -v fzf >/dev/null 2>&1; then
    echo "请选择要禁用的规则:"
    SELECTED_RULE=$(echo "$RULES" | fzf --header="选择要禁用的规则")
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

if [[ -z "$SELECTED_INDEX" ]]; then
  echo "错误: 无法从规则中提取索引号" >&2
  echo "规则文本: $SELECTED_RULE" >&2
  exit 1
fi

# 获取完整的规则信息
if command -v jq >/dev/null 2>&1; then
  FULL_RULE=$(echo "$RULES_JSON" | jq -r --arg idx "$SELECTED_INDEX" '.SecurityGroupPolicySet.Ingress[] | select(.PolicyIndex == ($idx|tonumber))')
  
  # 检查是否已经不是ACCEPT状态
  CURRENT_ACTION=$(echo "$FULL_RULE" | jq -r '.Action')
  if [[ "$CURRENT_ACTION" != "ACCEPT" ]]; then
    echo "规则 $SELECTED_DESC 已经不是允许状态，无需修改" >&2
    exit 0
  fi
fi

# 提取规则所需的所有信息
if command -v jq >/dev/null 2>&1; then
  PROTOCOL=$(echo "$FULL_RULE" | jq -r '.Protocol')
  PORT=$(echo "$FULL_RULE" | jq -r '.Port')
  CIDR_BLOCK=$(echo "$FULL_RULE" | jq -r '.CidrBlock')
  DESCRIPTION=$(echo "$FULL_RULE" | jq -r '.PolicyDescription')
else
  # 使用awk从SELECTED_RULE中提取
  PROTOCOL=$(echo "$SELECTED_RULE" | awk '{print $2}')
  PORT=$(echo "$SELECTED_RULE" | awk '{print $3}')
  CIDR_BLOCK=$(echo "$SELECTED_RULE" | awk '{print $4}')
  DESCRIPTION="$SELECTED_DESC"
fi

# 验证提取的信息
if [[ -z "$PROTOCOL" || -z "$PORT" || -z "$CIDR_BLOCK" || -z "$DESCRIPTION" ]]; then
  echo "错误: 无法提取完整的规则信息" >&2
  echo "PROTOCOL: $PROTOCOL" >&2
  echo "PORT: $PORT" >&2
  echo "CIDR_BLOCK: $CIDR_BLOCK" >&2
  echo "DESCRIPTION: $DESCRIPTION" >&2
  exit 1
fi

echo "采用删除-添加方式关闭端口..."

# 创建新的DROP规则
CREATE_JSON_FILE=$(mktemp)

cat > $CREATE_JSON_FILE <<EOF
{
  "SecurityGroupId": "$SECURITY_GROUP_ID",
  "SecurityGroupPolicySet": {
    "Ingress": [
      {
        "Protocol": "$PROTOCOL",
        "Port": "$PORT",
        "CidrBlock": "$CIDR_BLOCK",
        "Action": "DROP",
        "PolicyDescription": "$DESCRIPTION"
      }
    ]
  }
}
EOF

echo "创建规则拒绝状态..."
# 直接使用命令，不使用eval，确保安全执行
CREATE_CMD="$TCCLI_PATH vpc CreateSecurityGroupPolicies --region $REGION --profile default --cli-input-json file://$CREATE_JSON_FILE"

# 分两步执行命令，确保能够捕获所有输出
(
  CREATE_RESULT=$(bash -c "$CREATE_CMD" 2>&1)
  CREATE_CODE=$?

  if [ $CREATE_CODE -ne 0 ]; then
    cp "$CREATE_JSON_FILE" "/tmp/sg_create_failed.json"
    echo "创建拒绝规则失败，返回码: $CREATE_CODE" >&2
    echo "错误信息: $CREATE_RESULT" >&2
    echo "临时JSON文件已保存为: /tmp/sg_create_failed.json" >&2
    rm -f "$CREATE_JSON_FILE"
    exit 1
  fi

  if echo "$CREATE_RESULT" | grep -q 'Error\|error\|InvalidParameterValue'; then
    cp "$CREATE_JSON_FILE" "/tmp/sg_create_failed.json"
    echo "创建拒绝规则失败: $CREATE_RESULT" >&2
    echo "临时JSON文件已保存为: /tmp/sg_create_failed.json" >&2
    rm -f "$CREATE_JSON_FILE"
    exit 1
  fi

  rm -f "$CREATE_JSON_FILE"

  # 然后删除原始规则
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

  echo "删除原始规则..."
  DELETE_CMD="$TCCLI_PATH vpc DeleteSecurityGroupPolicies --region $REGION --profile default --cli-input-json file://$DELETE_JSON_FILE"

  DELETE_RESULT=$(bash -c "$DELETE_CMD" 2>&1)
  DELETE_CODE=$?

  if [ $DELETE_CODE -ne 0 ]; then
    cp "$DELETE_JSON_FILE" "/tmp/sg_delete_failed.json"
    echo "删除原始规则失败，返回码: $DELETE_CODE" >&2
    echo "错误信息: $DELETE_RESULT" >&2
    echo "警告: 新的规则已创建，原规则未删除" >&2
    echo "临时JSON文件已保存为: /tmp/sg_delete_failed.json" >&2
    rm -f "$DELETE_JSON_FILE"
    exit 1
  fi

  if echo "$DELETE_RESULT" | grep -q 'Error\|error'; then
    cp "$DELETE_JSON_FILE" "/tmp/sg_delete_failed.json"
    echo "删除原始规则失败: $DELETE_RESULT" >&2
    echo "警告: 新的规则已创建，原规则未删除" >&2
    echo "临时JSON文件已保存为: /tmp/sg_delete_failed.json" >&2
    rm -f "$DELETE_JSON_FILE"
    exit 1
  fi

  rm -f "$DELETE_JSON_FILE"
  echo "已成功将规则 $SELECTED_DESC 设置为禁止状态"
  exit 0
) || {
  # 如果上面的子shell失败，这里处理错误
  ERROR_CODE=$?
  echo "执行过程中发生错误，代码: $ERROR_CODE" >&2
  exit $ERROR_CODE
}

# 检查子shell的退出状态
if [ $? -ne 0 ]; then
  echo "操作未成功完成" >&2
  exit 1
fi 