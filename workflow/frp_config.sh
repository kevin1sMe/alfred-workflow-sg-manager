#!/bin/bash
# frp_config.sh - Manage workflow configuration (API keys, paths, etc.)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/utils.sh"

# Workflow config file
WORKFLOW_CONFIG_FILE="$HOME/.frp_workflow_config"

# Load config if exists
if [[ -f "$WORKFLOW_CONFIG_FILE" ]]; then
  source "$WORKFLOW_CONFIG_FILE"
fi

# Override set_env_var function
set_env_var() {
  local key="$1"
  local value="$2"
  # Save to config file
  if grep -q "^export $key=" "$WORKFLOW_CONFIG_FILE" 2>/dev/null; then
    sed -i "" "s|^export $key=.*|export $key=\"$value\"|" "$WORKFLOW_CONFIG_FILE"
  else
    echo "export $key=\"$value\"" >> "$WORKFLOW_CONFIG_FILE"
  fi
  # Also set in current environment
  export "$key"="$value"
}

CMD="$1"
shift || true

case "$CMD" in
  setup_keys)
    # Setup SecretId
    read -p "请输入腾讯云 SecretId: " SECRET_ID
    security add-generic-password -a "$USER" -s "Alfred_TencentCloud_FRP_Keys" -U -D "TencentCloud FRP Secret ID" -w "$SECRET_ID" -l "TENCENTCLOUD_FRP_SECRET_ID"
    # Setup SecretKey
    read -s -p "请输入腾讯云 SecretKey: " SECRET_KEY
    echo
    security add-generic-password -a "$USER" -s "Alfred_TencentCloud_FRP_Keys" -U -D "TencentCloud FRP Secret Key" -w "$SECRET_KEY" -l "TENCENTCLOUD_FRP_SECRET_KEY"
    echo "API 密钥已保存到 Keychain"
    ;;
  set_toml_path)
    if [[ -z "$1" ]]; then
      echo "用法: frp config set_toml_path <路径>" >&2
      exit 1
    fi
    set_env_var "FRPC_TOML_PATH" "$1"
    echo "frpc.toml 路径已设置: $1"
    ;;
  set_sgid)
    if [[ -z "$1" ]]; then
      echo "用法: frp config set_sgid <安全组ID>" >&2
      exit 1
    fi
    set_env_var "SECURITY_GROUP_ID" "$1"
    echo "安全组ID已设置: $1"
    ;;
  set_region)
    if [[ -z "$1" ]]; then
      echo "用法: frp config set_region <区域ID>" >&2
      echo "例如: ap-guangzhou, ap-shanghai, ap-beijing 等" >&2
      exit 1
    fi
    set_env_var "REGION" "$1"
    echo "腾讯云区域已设置: $1"
    ;;
  set_tccli_path)
    if [[ -z "$1" ]]; then
      echo "用法: frp config set_tccli_path <路径>" >&2
      exit 1
    fi
    set_env_var "TCCLI_PATH" "$1"
    echo "tccli 路径已设置: $1"
    ;;
  view)
    echo "FRPC_TOML_PATH: $FRPC_TOML_PATH"
    echo "SECURITY_GROUP_ID: $SECURITY_GROUP_ID"
    echo "REGION: ${REGION:-未设置 (默认:ap-guangzhou)}"
    echo "TCCLI_PATH: $TCCLI_PATH"
    ;;
  *)
    echo "用法: frp config [setup_keys|set_toml_path|set_sgid|set_region|set_tccli_path|view]" >&2
    exit 1
    ;;
esac 