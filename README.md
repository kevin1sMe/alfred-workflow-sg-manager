# Alfred Workflow - FRP 腾讯云安全组助手 (Bash 版)

## 简介
本项目为 Alfred Workflow，允许用户通过 Alfred 快速管理腾讯云安全组端口，适用于 frp 场景。

## 功能
- 解析 frpc.toml，列出服务及端口
- 一键开放/关闭指定端口到当前公网 IP
- 通过 macOS Keychain 安全存储 API 密钥
- 支持配置安全组 ID、frpc.toml 路径、tccli 路径

## 安装
1. 安装依赖：`tccli`、`jq`（推荐）
2. 导入 .alfredworkflow 文件或手动配置

## 使用
- `frp open` 选择服务开放端口
- `frp close` 关闭已开放端口
- `frp list` 查看已开放规则
- `frp config` 进行相关配置

详细用法见 Project.md。
