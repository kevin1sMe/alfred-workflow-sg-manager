#!/bin/bash
# frp_open.sh - Open a port in Tencent Cloud Security Group for selected frpc service

# 如果遇到错误立即退出
set -e

# Load utils
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/utils.sh"
source "$SCRIPT_DIR/frpc_parser.sh"

# Load config if exists
WORKFLOW_CONFIG_FILE="$HOME/.frp_workflow_config"
if [[ -f "$WORKFLOW_CONFIG_FILE" ]]; then
  source "$WORKFLOW_CONFIG_FILE"
fi

# Check config
check_env_or_exit "FRPC_TOML_PATH"
check_env_or_exit "SECURITY_GROUP_ID"

FRPC_TOML_PATH="$FRPC_TOML_PATH"
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

# Parse frpc.toml and list services
SERVICES=$(parse_frpc_services "$FRPC_TOML_PATH")
if [[ -z "$SERVICES" ]]; then
  echo "未在 frpc.toml 中找到服务配置" >&2
  exit 1
fi

# 如果没有提供参数，显示可用服务列表并退出
if [[ -z "$1" ]]; then
  echo "可用服务:"
  echo "$SERVICES" | nl
  echo "用法: $0 <服务名称|数字>"
  exit 0
fi

# 选择服务
if [[ "$1" =~ ^[0-9]+$ ]]; then
  # 根据数字选择
  LINE_NUMBER=$1
  SELECTED_SERVICE=$(echo "$SERVICES" | sed -n "${LINE_NUMBER}p")
else
  # 根据名称选择
  SELECTED_SERVICE=$(echo "$SERVICES" | grep -i "^$1 " || echo "")
fi

if [[ -z "$SELECTED_SERVICE" ]]; then
  echo "未找到服务: $1" >&2
  echo "可用服务:"
  echo "$SERVICES" | nl
  exit 1
fi

SELECTED_PORT=$(echo "$SELECTED_SERVICE" | awk '{print $2}')
SELECTED_NAME=$(echo "$SELECTED_SERVICE" | awk '{print $1}')

# 获取本地端口信息（如果存在）
LOCAL_PORT=""
if grep -q "name = \"$SELECTED_NAME\"" "$FRPC_TOML_PATH"; then
  LOCAL_PORT=$(grep -A10 "name = \"$SELECTED_NAME\"" "$FRPC_TOML_PATH" | grep "localPort" | head -1 | sed 's/.*= *\([0-9]*\).*/\1/')
fi

# Get current public IP
CURRENT_IP=$(curl -s ifconfig.me)
if [[ -z "$CURRENT_IP" ]]; then
  echo "无法获取公网 IP" >&2
  exit 1
fi

# 为调试添加输出
echo "服务名称: $SELECTED_NAME"
echo "远程端口: $SELECTED_PORT"
if [[ ! -z "$LOCAL_PORT" ]]; then
  echo "本地端口: $LOCAL_PORT"
fi
echo "当前IP: $CURRENT_IP"
echo "安全组ID: $SECURITY_GROUP_ID"
echo "区域: $REGION"

# 查询现有安全组规则
echo "查询现有安全组规则..."
EXISTING_RULES=$($TCCLI_PATH vpc DescribeSecurityGroupPolicies --region $REGION --profile default --SecurityGroupId $SECURITY_GROUP_ID)
EXISTING_RULE_ID=""

# 保存API返回结果以便调试
echo "安全组规则查询结果:" > /tmp/sg_rules_debug.json
echo "$EXISTING_RULES" >> /tmp/sg_rules_debug.json

# 使用服务名称作为唯一标识符
if [[ ! -z "$LOCAL_PORT" ]]; then
  RULE_TAG="AlfredFRP_${SELECTED_NAME}_local${LOCAL_PORT}"
else
  RULE_TAG="AlfredFRP_${SELECTED_NAME}"
fi
DESCRIPTION="$RULE_TAG"

# 检查是否已存在相同端口和名称的规则
echo "检查是否已存在相同端口($SELECTED_PORT)和名称($SELECTED_NAME)的规则..."
if [[ ! -z "$EXISTING_RULES" ]]; then
  if command -v jq >/dev/null 2>&1; then
    echo "使用jq解析规则..."
    # 打印入站规则列表（仅供调试）
    echo "$EXISTING_RULES" | jq '.SecurityGroupPolicySet.Ingress' > /tmp/ingress_rules.json
    
    # 查找基于端口和服务名称描述的匹配规则
    EXISTING_RULE_ID=$(echo "$EXISTING_RULES" | jq -r --arg PORT "$SELECTED_PORT" --arg NAME "$SELECTED_NAME" '.SecurityGroupPolicySet.Ingress[] | select(.Protocol=="tcp" and .Port==$PORT and (.PolicyDescription | contains($NAME))) | .PolicyIndex' 2>/dev/null | head -1)
    
    # 如果找不到匹配的规则，打印现有规则以便调试
    if [[ -z "$EXISTING_RULE_ID" ]]; then
      echo "当前安全组规则:"
      echo "$EXISTING_RULES" | jq -r '.SecurityGroupPolicySet.Ingress[] | "端口: \(.Port), 描述: \(.PolicyDescription), ID: \(.PolicyIndex)"'
    fi
  else
    # 如果没有jq，使用grep和简单的文本处理
    echo "使用grep解析规则..."
    # 先尝试匹配端口和描述中包含服务名称的规则
    EXISTING_RULE_ID=$(echo "$EXISTING_RULES" | grep -A15 "\"Port\": *\"$SELECTED_PORT\"" | grep -A5 "\"PolicyDescription\".*$SELECTED_NAME" | grep "PolicyIndex" | head -1 | sed 's/.*: *\([0-9]*\).*/\1/' || echo "")
  fi
  
  echo "找到的规则ID: $EXISTING_RULE_ID"
fi

# 调试输出
echo "检测结果: PORT=$SELECTED_PORT, NAME=$SELECTED_NAME, RULE_ID=$EXISTING_RULE_ID"

# 创建临时JSON文件
TEMP_JSON_FILE=$(mktemp)

if [[ -z "$EXISTING_RULE_ID" ]]; then
  # 不存在规则，创建新规则
  echo "未找到现有规则，创建新规则..."
  cat > $TEMP_JSON_FILE <<EOF
{
  "SecurityGroupId": "$SECURITY_GROUP_ID",
  "SecurityGroupPolicySet": {
    "Ingress": [
      {
        "Protocol": "TCP",
        "Port": "$SELECTED_PORT",
        "CidrBlock": "$CURRENT_IP/32",
        "Action": "ACCEPT",
        "PolicyDescription": "$DESCRIPTION"
      }
    ]
  }
}
EOF

  # 使用JSON文件调用API创建规则
  echo "正在调用腾讯云API创建规则..."
  $TCCLI_PATH vpc CreateSecurityGroupPolicies --region $REGION --profile default --cli-input-json "file://$TEMP_JSON_FILE"
  RET_CODE=$?
else
  # 存在规则，更新规则（先删除再创建）
  echo "发现端口 $SELECTED_PORT 的现有规则(ID:$EXISTING_RULE_ID)，更新规则..."
  
  # 删除现有规则
  DELETE_JSON=$(mktemp)
  cat > $DELETE_JSON <<EOF
{
  "SecurityGroupId": "$SECURITY_GROUP_ID",
  "SecurityGroupPolicySet": {
    "Ingress": [
      {
        "PolicyIndex": $EXISTING_RULE_ID
      }
    ]
  }
}
EOF

  echo "删除旧规则..."
  DELETE_RESULT=$($TCCLI_PATH vpc DeleteSecurityGroupPolicies --region $REGION --profile default --cli-input-json "file://$DELETE_JSON")
  DELETE_CODE=$?
  echo "删除结果: $DELETE_RESULT (代码:$DELETE_CODE)"
  rm -f "$DELETE_JSON"
  
  # 创建新规则
  cat > $TEMP_JSON_FILE <<EOF
{
  "SecurityGroupId": "$SECURITY_GROUP_ID",
  "SecurityGroupPolicySet": {
    "Ingress": [
      {
        "Protocol": "TCP",
        "Port": "$SELECTED_PORT",
        "CidrBlock": "$CURRENT_IP/32",
        "Action": "ACCEPT",
        "PolicyDescription": "$DESCRIPTION"
      }
    ]
  }
}
EOF

  echo "创建更新后的规则..."
  $TCCLI_PATH vpc CreateSecurityGroupPolicies --region $REGION --profile default --cli-input-json "file://$TEMP_JSON_FILE"
  RET_CODE=$?
fi

# 删除临时文件
rm -f "$TEMP_JSON_FILE"

if [ $RET_CODE -ne 0 ]; then
  echo "操作失败，错误码: $RET_CODE" >&2
  exit 1
fi

if [[ -z "$EXISTING_RULE_ID" ]]; then
  if [[ ! -z "$LOCAL_PORT" ]]; then
    echo "已成功开放 $SELECTED_NAME (远程:$SELECTED_PORT, 本地:$LOCAL_PORT) 到 $CURRENT_IP/32" 
  else
    echo "已成功开放 $SELECTED_NAME ($SELECTED_PORT) 到 $CURRENT_IP/32" 
  fi
else
  if [[ ! -z "$LOCAL_PORT" ]]; then
    echo "已成功更新 $SELECTED_NAME (远程:$SELECTED_PORT, 本地:$LOCAL_PORT) 规则到 $CURRENT_IP/32"
  else
    echo "已成功更新 $SELECTED_NAME ($SELECTED_PORT) 规则到 $CURRENT_IP/32"
  fi
fi

# 添加调试提示
echo "提示: 本次操作的调试日志保存在 /tmp/sg_rules_debug.json" 