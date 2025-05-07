#!/bin/bash
# utils.sh - Common utility functions for workflow scripts

# Check if required env var is set, exit if not
check_env_or_exit() {
  local var="$1"
  if [[ -z "${!var}" ]]; then
    echo "环境变量 $var 未设置，请先运行 frp config" >&2
    exit 1
  fi
}

# Get value from macOS Keychain
get_keychain_value() {
  local service="$1"
  local label="$2"
  
  # Try account-based approach
  local value=$(security find-generic-password -a "$USER" -s "$service" -l "$label" -w 2>/dev/null)
  if [[ -n "$value" ]]; then
    echo "$value"
    return 0
  fi
  
  # Try direct approach
  value=$(security find-generic-password -s "$service" -l "$label" -w 2>/dev/null)
  if [[ -n "$value" ]]; then
    echo "$value"
    return 0
  fi
  
  return 1
}

# Set workflow environment variable (using defaults write)
set_env_var() {
  local key="$1"
  local value="$2"
  # 这里假设 Alfred 环境变量通过 defaults 写入 info.plist
  defaults write "$(pwd)/info.plist" "$key" "$value"
  export "$key"="$value"
}