**项目说明书：Alfred Workflow - FRP 腾讯云安全组助手 (Go 版，基于 awgo)**

**1. 项目目标**

开发一个基于 Go 语言、使用 awgo 库的 Alfred Workflow，允许用户通过 Alfred 快速、安全地管理腾讯云特定安全组的入站规则。主要功能包括：从本地 `frpc.toml` 文件读取服务配置（服务名、公网端口、本地端口、协议），根据用户选择动态开放所选服务端口到当前公网 IP，并在使用完毕后关闭这些端口。API 密钥将安全地存储和访问 macOS Keychain。

**2. 技术栈与依赖**

- 主要语言：Go
- Alfred Workflow 交互：awgo（https://github.com/deanishe/awgo）
- 腾讯云 API 调用：直接通过 Go HTTP 请求腾讯云 API（不再依赖 tccli）
- frpc.toml 解析：Go TOML 解析库（如 BurntSushi/toml）
- Keychain 交互：go-keychain 库
- 公网 IP 获取：Go 调用外部 API（如 ifconfig.me）
- 配置存储：awgo 的配置机制（env/JSON/config文件等）

**3. 主要功能与 Alfred 交互流程**

- 采用 awgo 的 Script Filter 机制，所有交互均为"先出列表，用户选择，再进入下一步"。
- 操作结果主要通过 Alfred 的通知反馈。

主要命令及交互流程：

1. `frp open`
   - 读取并解析 frpc.toml，展示所有服务（服务名+端口+协议+本地端口）列表
   - 展示每个服务当前在安全组中的状态（已开放/未开放）
   - 用户选择服务后，获取当前公网 IP，调用腾讯云 API 开放端口（支持 TCP/UDP 协议）
   - 备注中记录 local_port 信息
   - 操作结果通过 Alfred 通知反馈

2. `frp close`
   - 查询当前由 Workflow 创建的安全组规则（PolicyDescription 以 AlfredFRP_ 开头）
   - 展示规则列表（服务名、端口、IP、协议、备注等）
   - 用户选择要关闭的规则，调用腾讯云 API 删除规则
   - 操作结果通过 Alfred 通知反馈

3. `frp list`
   - 结合 frpc.toml 展示所有服务及其在安全组中的状态（已开放/未开放），并显示详细信息

4. `frp config`
   - 子命令列表（setup_keys, set_toml_path, set_sgid, view）
   - 每个子命令均为交互式流程，按 awgo 交互规范实现
   - 由于采用 Go 直接调用 API，部分配置项（如 tccli 路径）可去除

**4. 配置项与初始化**

- API 密钥（SecretId/SecretKey）：存储于 macOS Keychain，Go 代码通过 go-keychain 读取
- frpc.toml 路径、安全组 ID：存储于 awgo 的配置机制
- 配置流程通过 `frp config` 子命令引导

**5. 解析 frpc.toml 逻辑**

- 使用 Go TOML 解析库，支持标准 frpc.toml 格式
- 解析所有服务区块，提取服务名、remote_port、local_port、protocol（支持 TCP/UDP，若未指定协议则默认为 TCP）
- local_port 信息会在安全组规则备注中体现

**6. 腾讯云 API 交互**

- 直接用 Go HTTP 请求腾讯云 VPC API（如 CreateSecurityGroupPolicies、DeleteSecurityGroupPolicies、DescribeSecurityGroupPolicies）
- 支持 TCP 和 UDP 协议
- 所有 API 调用需处理异常并友好反馈

**7. 错误处理与用户反馈**

- 检查所有依赖（keychain、frpc.toml、API 配置等）
- 任何错误都应通过 Alfred 通知清晰反馈
- 操作成功/失败均有明确提示

**8. 安全注意事项**

- API 密钥仅通过 Keychain 管理
- 提示用户为腾讯云子用户配置最小权限
- 配置文件权限需妥善设置

**9. 假设与前提**

- 用户已安装 Alfred 及 awgo 支持的 Go 运行环境
- 用户已具备腾讯云 API 权限
