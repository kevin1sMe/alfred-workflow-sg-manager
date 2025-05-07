#!/bin/bash
# frpc_parser.sh - Parse frpc.toml for service names and remote ports

# Parse frpc.toml and output: <service_name> <remote_port>
parse_frpc_services() {
  local toml_file="$1"
  local proxy_block=false
  local name=""
  local port=""
  
  # 检查文件是否存在
  if [[ ! -f "$toml_file" ]]; then
    echo "错误：文件 '$toml_file' 不存在" >&2
    return 1
  fi
  
  while IFS= read -r line; do
    # 移除不可见的特殊字符
    line=$(echo "$line" | tr -d '\000-\011\013-\037\177%')
    
    # 跳过注释和空行
    if [[ $line =~ ^[[:space:]]*# || -z $line ]]; then
      continue
    fi
    
    # 检测代理块的开始
    if [[ $line =~ ^\[\[proxies\]\] ]]; then
      proxy_block=true
      
      # 如果已经处理过一个代理，输出它
      if [[ -n "$name" && -n "$port" ]]; then
        echo "$name $port"
      fi
      
      name=""
      port=""
      continue
    fi
    
    # 只在代理块内处理配置
    if [[ "$proxy_block" = true ]]; then
      # 提取名称
      if [[ $line =~ ^name[[:space:]]*=[[:space:]]*\"([^\"]+)\" ]]; then
        name="${BASH_REMATCH[1]}"
      fi
      
      # 提取端口
      if [[ $line =~ ^remotePort[[:space:]]*=[[:space:]]*([0-9]+) ]]; then
        port="${BASH_REMATCH[1]}"
      fi
    fi
  done < "$toml_file"
  
  # 确保处理最后一个代理
  if [[ -n "$name" && -n "$port" ]]; then
    echo "$name $port"
  fi
}

# 自动调用函数（当脚本直接运行时）
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  parse_frpc_services "$1"
fi