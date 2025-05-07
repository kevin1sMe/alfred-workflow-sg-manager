**项目说明书：Alfred Workflow - FRP 腾讯云安全组助手 (Bash 版)**

**1. 项目目标**

开发一个 Alfred Workflow，允许用户通过简单的 Alfred 命令，快速、安全地管理腾讯云特定安全组的入站规则。主要功能包括：从本地 `frpc.toml` 文件读取服务配置（服务名和公网端口），根据用户选择动态开放所选服务端口到当前公网 IP，并在使用完毕后关闭这些端口。API 密钥将安全地存储和访问 macOS Keychain。

**2. 核心技术栈与命令**

* **Alfred Workflow 结构**：使用 Alfred 的 Script Filter, Run Script 等组件。
* **脚本语言**：主要使用 Bash。
* **macOS Keychain 交互**：`security` 命令行工具 (用于存储和读取 API 密钥)。
* **腾讯云 API 交互**：`tccli` (腾讯云命令行工具)。假设用户已安装并配置了基础的 `tccli` 或 Workflow 会引导指定路径。
* **`frpc.toml` 解析**：使用 Bash 内建命令如 `grep`, `awk`, `sed` 来解析 TOML 文件中的服务名和 `remote_port`。由于 Bash 没有原生 TOML 解析器，解析将基于常见的 `frpc.toml` 结构进行模式匹配。
* **JSON 处理**：强烈建议使用 `jq` 来解析 `tccli` 返回的 JSON 数据。如果不能依赖 `jq`，则需使用更复杂的 `grep`/`awk`/`sed` 组合，这将显著增加脚本复杂度。本说明书将优先考虑使用 `jq` 的方案。
* **公网 IP 获取**：`curl ifconfig.me` 或 `curl ip.sb` 等命令。
* **Alfred Workflow 配置存储**：使用 Workflow 环境变量或通过 `defaults write/read` 修改 Workflow 的 `info.plist` 来存储如 `frpc.toml` 路径、安全组 ID 等配置。

**3. 配置项与初始化**

* **3.1. API 密钥 (腾讯云)**
    * **存储**：`SecretId` 和 `SecretKey` 存储在 macOS Keychain 中。
    * **Keychain 服务名称 (Service Name)**：建议使用如 `Alfred_TencentCloud_FRP_Keys`。
    * **Keychain 账户名称 (Account Name)**：`SecretId` 使用 `TENCENTCLOUD_FRP_SECRET_ID`，`SecretKey` 使用 `TENCENTCLOUD_FRP_SECRET_KEY`。
    * **设置方式**：Workflow 提供一个 `frp config setup_keys` 命令引导用户输入并存储。

* **3.2. `frpc.toml` 文件路径**
    * **存储**：存储在 Workflow 的环境变量 `FRPC_TOML_PATH` 中。
    * **设置方式**：Workflow 提供 `frp config set_toml_path <路径>` 命令。默认为空，脚本会提示用户设置。

* **3.3. 腾讯云安全组 ID**
    * **存储**：存储在 Workflow 的环境变量 `SECURITY_GROUP_ID` 中。
    * **设置方式**：Workflow 提供 `frp config set_sgid <安全组ID>` 命令。默认为空，脚本会提示用户设置。

* **3.4. `tccli` 路径 (可选)**
    * **存储**：Workflow 环境变量 `TCCLI_PATH`。
    * **设置方式**：如果 `tccli` 不在标准 `$PATH` 中，用户可通过 `frp config set_tccli_path <路径>` 设置。脚本会优先使用此变量，否则尝试直接调用 `tccli`。

**4. `frpc.toml` 解析逻辑 (Bash)**

* 脚本将读取由 `FRPC_TOML_PATH` 指定的 `frpc.toml` 文件。
* 使用 `grep '^\s*\[.*\]'` 提取服务区块名称 (如 `[ssh_nas]` 将提取 `ssh_nas`)。
* 对于每个区块，使用 `grep '^\s*remote_port\s*='` 和 `awk -F '=' '{print $2}'` 及 `sed 's/ //g'` 清理空格和引号，提取 `remote_port` 的值。
* **注意**：此解析方法依赖于 `remote_port` 和区块定义在各自的行上，且格式相对简单。复杂的 TOML 结构可能无法正确解析。
* **协议处理**：默认所有端口为 TCP。如果需要支持 UDP，可以在 `frpc.toml` 中通过注释约定 (例如 `# protocol = UDP` 在 `remote_port` 行之后)，脚本尝试解析此注释。或者在 Workflow 中为特定服务名硬编码协议类型。为简化初期开发，可先仅支持 TCP。

**5. Alfred Workflow 功能模块**

Workflow 触发关键词为 `frp`。

* **5.1. `frp open [服务名查询]` (Script Filter & Run Script)**
    1.  **脚本检查**：检查 `FRPC_TOML_PATH` 和 `SECURITY_GROUP_ID` 是否已配置。未配置则提示用户使用 `frp config` 命令。
    2.  **解析 `frpc.toml`**：列出可识别的服务名及其 `remote_port`。用户可以通过 Alfred 搜索并选择服务。
    3.  **获取 API 密钥**：使用 `security find-generic-password ...` 从 Keychain 读取 `SecretId` 和 `SecretKey`。失败则提示用户运行 `frp config setup_keys`。
    4.  **获取公网 IP**：执行 `CURRENT_IP=$(curl -s ifconfig.me)/32`。
    5.  **构造并执行 `tccli` 命令 (添加入站规则)**：
        ```bash
        # 示例 tccli 命令 (需替换变量)
        "$TCCLI_PATH_VAR" vpc CreateSecurityGroupPolicies --Version 2017-03-12 \
            --SecurityGroupId "$SECURITY_GROUP_ID_VAR" \
            --SecurityGroupPolicySet.Ingress.0.Protocol "TCP" \ # 或根据解析/约定选择 UDP
            --SecurityGroupPolicySet.Ingress.0.Port "$SELECTED_REMOTE_PORT_VAR" \
            --SecurityGroupPolicySet.Ingress.0.CidrBlock "$CURRENT_IP_VAR" \
            --SecurityGroupPolicySet.Ingress.0.Action "ACCEPT" \
            --SecurityGroupPolicySet.Ingress.0.PolicyDescription "AlfredFRP_$(date +%s)_${SELECTED_SERVICE_NAME_VAR}"
        ```
    6.  **反馈**：向 Alfred 显示操作成功（包括 IP 和端口）或失败信息。

* **5.2. `frp close [已开放规则查询]` (Script Filter & Run Script)**
    1.  **获取 API 密钥**：同上。
    2.  **查询当前由 Workflow 创建的规则**：
        ```bash
        "$TCCLI_PATH_VAR" vpc DescribeSecurityGroupPolicies --Version 2017-03-12 \
            --SecurityGroupId "$SECURITY_GROUP_ID_VAR" \
            | jq -r '.SecurityGroupPolicySet.Ingress[] | select(.PolicyDescription | startswith("AlfredFRP_")) | "\(.PolicyDescription) \(.Protocol) \(.Port) \(.CidrBlock) Index:\(.PolicyIndex)"'
        ```
        *(如果不用 `jq`，则需要复杂的 `grep`/`awk` 解析)*
    3.  **展示规则列表**：用户选择要关闭的规则。
    4.  **构造并执行 `tccli` 命令 (删除入站规则)**：
        ```bash
        # 示例（需要获取选中规则的 PolicyIndex）
        "$TCCLI_PATH_VAR" vpc DeleteSecurityGroupPolicies --Version 2017-03-12 \
            --SecurityGroupId "$SECURITY_GROUP_ID_VAR" \
            --SecurityGroupPolicySet.Ingress.0.PolicyIndex $SELECTED_POLICY_INDEX_VAR
        ```
        *确保能准确获取并传递 `PolicyIndex`。如果 API 支持通过其他唯一标识删除则更好，但通常 `PolicyIndex` 是必须的。*
    5.  **反馈**：显示操作成功或失败。

* **5.3. `frp list` (Script Filter)**
    * 与 `frp close` 的查询步骤类似，获取并展示所有由 "AlfredFRP_" 开头的规则及其详情。

* **5.4. `frp config` (Keyword with Argument Required / Script Filter for sub-commands)**
    * **`frp config setup_keys` (Run Script)**：
        * 提示用户输入 `SecretId`。
        * 使用 `security add-generic-password -a "$USER" -s "Alfred_TencentCloud_FRP_Keys" -U -D "TencentCloud FRP Secret ID" -w "<SecretId_Input>" -l "TENCENTCLOUD_FRP_SECRET_ID"` (示例，具体参数可能需要调整)。
        * 提示用户输入 `SecretKey` (使用读取密码的安全方式，如 `read -s`)。
        * 使用 `security add-generic-password ... -l "TENCENTCLOUD_FRP_SECRET_KEY" ...` 存储。
    * **`frp config set_toml_path <路径>` (Run Script)**：
        * 将 `<路径>` 保存到 Workflow 环境变量 `FRPC_TOML_PATH`。
    * **`frp config set_sgid <安全组ID>` (Run Script)**：
        * 将 `<安全组ID>` 保存到 Workflow 环境变量 `SECURITY_GROUP_ID`。
    * **`frp config set_tccli_path <路径>` (Run Script)**：
        * 将 `<路径>` 保存到 Workflow 环境变量 `TCCLI_PATH`。
    * **`frp config view` (Script Filter)**：显示当前配置的 `FRPC_TOML_PATH`, `SECURITY_GROUP_ID`, `TCCLI_PATH`。

**6. 错误处理**

* 脚本需检查 `tccli`、`curl`、`security` (以及 `jq` 如果使用) 命令是否存在。
* `tccli` 执行的任何错误（权限、网络、参数错误）都应捕获并向用户显示有意义的错误信息。
* Keychain 访问失败（密钥不存在、用户拒绝访问）。
* `frpc.toml` 文件不存在或无法按预期解析。
* 获取公网 IP 失败。

**7. 输出与用户反馈**

* 所有操作都应通过 Alfred 的通知或结果列表给用户清晰的反馈（成功、失败、错误信息、当前状态）。
* 列表条目应包含足够的信息帮助用户做选择（例如，服务名、端口、已开放规则的 IP 和端口）。

**8. 安全注意事项**

* API 密钥严格通过 Keychain 管理，不在脚本中硬编码。
* 提醒用户为其腾讯云子用户配置最小必要权限（仅管理特定安全组的策略）。
* `tccli` 的配置文件权限（如果使用 `~/.tccli`）应妥善设置。

**9. 假设**

* 用户已安装 `tccli`。
* 强烈推荐用户安装 `jq`。如果AI生成的代码需要同时提供有 `jq` 和无 `jq` 的版本，请明确指出。
* Workflow 在 macOS 环境下运行。
