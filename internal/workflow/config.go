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
		// 直接调用 showConfigHelp 展示子命令列表
		showConfigHelp(wf)
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
	default:
		showConfigHelp(wf)
	}
}

func showConfigHelp(wf *aw.Workflow) {
	cfg, _ := config.Load()

	// frpc.toml 路径
	tomlPath := "未设置"
	if cfg.FrpcTomlPath != "" {
		tomlPath = cfg.FrpcTomlPath
	}
	wf.NewItem("设置 frpc.toml 路径").
		Subtitle(tomlPath).
		Valid(true).
		Arg("set_toml_path," + tomlPath)

	// 安全组ID
	sgid := "未设置"
	if cfg.SecurityGroupId != "" {
		sgid = cfg.SecurityGroupId
	}
	wf.NewItem("设置安全组 ID").
		Subtitle(sgid).
		Valid(true).
		Arg("set_sgid," + sgid)

	// 区域
	region := "未设置"
	if cfg.Region != "" {
		region = cfg.Region
	}
	wf.NewItem("设置腾讯云 API 区域").
		Subtitle(region).
		Valid(true).
		Arg("set_region," + region)

	// 密钥（可选，按需展示）
	id, _ := config.GetSecretId()
	key, _ := config.GetSecretKey()
	idShow := "未设置"
	keyShow := "未设置"
	if id != "" {
		idShow = maskSecret(id)
	}
	if key != "" {
		keyShow = maskSecret(key)
	}
	wf.NewItem("设置腾讯云 API 密钥").
		Subtitle(fmt.Sprintf("id: %s, key: %s", idShow, keyShow)).
		Valid(true).
		Arg(fmt.Sprintf("setup_keys,%s,%s", id, key))

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

func maskSecret(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:4] + strings.Repeat("*", 4) + s[len(s)-4:]
}
