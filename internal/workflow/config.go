package workflow

import (
	"fmt"
	"os"
	"strings"

	aw "github.com/deanishe/awgo"
	"github.com/kevin1sMe/alfred-workflow-sg-manager/internal/config"
)

// ConfigCommand 处理 config 子命令
func ConfigCommand(wf *aw.Workflow, args []string) {
	// 记录传入的参数，帮助调试
	fmt.Fprintf(os.Stderr, "ConfigCommand received args: %v\n", args)

	if len(args) == 1 {
		// 展示子命令列表
		wf.NewItem("设置腾讯云 API 密钥").Arg("config setup_keys").Valid(true).Subtitle("config setup_keys")
		wf.NewItem("设置 frpc.toml 路径").Arg("config set_toml_path").Valid(true).Subtitle("config set_toml_path <路径>")
		wf.NewItem("设置安全组 ID").Arg("config set_sgid").Valid(true).Subtitle("config set_sgid <安全组ID>")
		wf.NewItem("设置腾讯云 API 区域").Arg("config set_region").Valid(true).Subtitle("config set_region <区域代码>") // 新增设置区域
		wf.NewItem("查看当前配置").Arg("config view").Valid(true).Subtitle("config view")
		wf.SendFeedback()
		return
	}

	// 普通情况下处理子命令
	sub := args[1]
	switch sub {
	case "setup_keys":
		setupKeys(wf)
	case "set_toml_path":
		if len(args) < 3 {
			wf.NewItem("请输入 frpc.toml 路径后回车").Valid(false)
			wf.SendFeedback()
			return
		}
		setTomlPath(wf, args[2])
	case "set_sgid":
		if len(args) < 3 {
			wf.NewItem("请输入安全组ID后回车").Valid(false)
			wf.SendFeedback()
			return
		}
		setSgid(wf, args[2])
	case "set_region": // 新增 set_region case
		if len(args) < 3 {
			wf.NewItem("请输入区域代码后回车 (例如 ap-guangzhou)").Subtitle("例如：ap-shanghai, ap-beijing, ap-guangzhou").Valid(false)
			wf.SendFeedback()
			return
		}
		setRegion(wf, args[2])
	case "view":
		viewConfig(wf)
	default:
		showConfigHelp(wf)
	}
}

func showConfigHelp(wf *aw.Workflow) {
	wf.NewItem("frp config setup_keys").Subtitle("设置腾讯云 API 密钥").Valid(true).Arg("config setup_keys")
	wf.NewItem("frp config set_toml_path <路径>").Subtitle("设置 frpc.toml 路径").Valid(true).Arg("config set_toml_path")
	wf.NewItem("frp config set_sgid <安全组ID>").Subtitle("设置安全组 ID").Valid(true).Arg("config set_sgid")
	wf.NewItem("frp config set_region <区域代码>").Subtitle("设置腾讯云 API 区域 (例如 ap-guangzhou)").Valid(true).Arg("config set_region") // 新增帮助信息
	wf.NewItem("frp config view").Subtitle("查看当前配置").Valid(true).Arg("config view")
	wf.SendFeedback()
}

func setupKeys(wf *aw.Workflow) {
	// Alfred 交互式输入建议用 awgo 的 Arg/Script Filter 机制，这里简化为环境变量读取
	secretId := os.Getenv("FRP_SECRET_ID")
	secretKey := os.Getenv("FRP_SECRET_KEY")
	if secretId == "" || secretKey == "" {
		wf.NewItem("请设置 FRP_SECRET_ID 和 FRP_SECRET_KEY 环境变量后重试").Valid(false)
		wf.SendFeedback()
		return
	}
	err1 := config.SaveSecretId(secretId)
	err2 := config.SaveSecretKey(secretKey)
	if err1 != nil || err2 != nil {
		wf.NewItem("保存密钥失败").Subtitle(fmt.Sprintf("%v %v", err1, err2)).Valid(false)
	} else {
		wf.NewItem("API 密钥保存成功").Valid(false)
	}
	wf.SendFeedback()
}

func setTomlPath(wf *aw.Workflow, path string) {
	cfg, _ := config.Load()
	cfg.FrpcTomlPath = path
	if err := config.Save(cfg); err != nil {
		wf.NewItem("保存 frpc.toml 路径失败").Subtitle(err.Error()).Valid(false)
	} else {
		wf.NewItem("frpc.toml 路径已保存").Subtitle(path).Valid(false)
	}
	wf.SendFeedback()
}

func setSgid(wf *aw.Workflow, sgid string) {
	cfg, _ := config.Load()
	cfg.SecurityGroupId = sgid
	if err := config.Save(cfg); err != nil {
		wf.NewItem("保存安全组ID失败").Subtitle(err.Error()).Valid(false)
	} else {
		wf.NewItem("安全组ID已保存").Subtitle(sgid).Valid(false)
	}
	wf.SendFeedback()
}

// setRegion 保存腾讯云 API 区域
func setRegion(wf *aw.Workflow, region string) {
	cfg, err := config.Load()
	if err != nil {
		wf.NewItem("加载配置失败").Subtitle(err.Error()).Valid(false)
		wf.SendFeedback()
		return
	}
	cfg.Region = region
	if err := config.Save(cfg); err != nil {
		wf.NewItem("保存区域配置失败").Subtitle(err.Error()).Valid(false)
	} else {
		wf.NewItem("腾讯云 API 区域已保存").Subtitle(region).Valid(false)
	}
	wf.SendFeedback()
}

func viewConfig(wf *aw.Workflow) {
	cfg, _ := config.Load()

	// 创建可选择的项目以显示配置
	if cfg.FrpcTomlPath != "" {
		wf.NewItem("frpc.toml 路径").
			Subtitle(cfg.FrpcTomlPath).
			Arg(cfg.FrpcTomlPath).      // 设置 Arg 以便回车时传递
			Copytext(cfg.FrpcTomlPath). // 设置 Copytext 以便 Cmd+C 复制
			Valid(true).
			Icon(aw.IconNote)
	} else {
		wf.NewItem("frpc.toml 路径").
			Subtitle("未设置").
			Valid(false).
			Icon(aw.IconNote)
	}

	if cfg.SecurityGroupId != "" {
		wf.NewItem("安全组ID").
			Subtitle(cfg.SecurityGroupId).
			Arg(cfg.SecurityGroupId).      // 设置 Arg 以便回车时传递
			Copytext(cfg.SecurityGroupId). // 设置 Copytext 以便 Cmd+C 复制
			Valid(true).
			Icon(aw.IconNote)
	} else {
		wf.NewItem("安全组ID").
			Subtitle("未设置").
			Valid(false).
			Icon(aw.IconNote)
	}

	// 显示 Region
	if cfg.Region != "" {
		wf.NewItem("腾讯云 API 区域").
			Subtitle(cfg.Region).
			Arg(cfg.Region).
			Copytext(cfg.Region).
			Valid(true).
			Icon(aw.IconNote)
	} else {
		wf.NewItem("腾讯云 API 区域").
			Subtitle("未设置").
			Valid(false).
			Icon(aw.IconNote)
	}

	// 密钥部分
	id, err1 := config.GetSecretId()
	key, err2 := config.GetSecretKey()

	idShow := "未设置"
	actualId := "" // 用于存储真实的ID值
	if err1 == nil && id != "" {
		idShow = maskSecret(id)
		actualId = id
		wf.NewItem("SecretId").
			Subtitle(idShow).
			Arg(actualId).      // 设置 Arg 以便回车时传递实际的 ID
			Copytext(actualId). // 设置 Copytext 以便 Cmd+C 复制实际的 ID
			Valid(true).
			Icon(aw.IconSettings) // 使用 aw.IconSettings 替代
	} else {
		wf.NewItem("SecretId").
			Subtitle(idShow). // 显示 "未设置"
			Valid(false).
			Icon(aw.IconSettings) // 使用 aw.IconSettings 替代
	}

	keyShow := "未设置"
	actualKey := "" // 用于存储真实的Key值
	if err2 == nil && key != "" {
		keyShow = maskSecret(key)
		actualKey = key
		wf.NewItem("SecretKey").
			Subtitle(keyShow).
			Arg(actualKey).      // 设置 Arg 以便回车时传递实际的 Key
			Copytext(actualKey). // 设置 Copytext 以便 Cmd+C 复制实际的 Key
			Valid(true).
			Icon(aw.IconSettings) // 使用 aw.IconSettings 替代
	} else {
		wf.NewItem("SecretKey").
			Subtitle(keyShow). // 显示 "未设置"
			Valid(false).
			Icon(aw.IconSettings) // 使用 aw.IconSettings 替代
	}

	// 确保显示结果
	wf.SendFeedback()
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}
