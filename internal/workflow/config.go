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
	if len(args) == 1 {
		// 展示子命令列表
		wf.NewItem("设置腾讯云 API 密钥").Arg("setup_keys").Valid(true).Subtitle("frp config setup_keys")
		wf.NewItem("设置 frpc.toml 路径").Arg("set_toml_path").Valid(true).Subtitle("frp config set_toml_path <路径>")
		wf.NewItem("设置安全组 ID").Arg("set_sgid").Valid(true).Subtitle("frp config set_sgid <安全组ID>")
		wf.NewItem("查看当前配置").Arg("config view").Valid(true).Subtitle("将会执行命令：frp config view")
		wf.SendFeedback()
		return
	}
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
	case "view":
		viewConfig(wf)
	default:
		showConfigHelp(wf)
	}
}

func showConfigHelp(wf *aw.Workflow) {
	wf.NewItem("frp config setup_keys").Subtitle("设置腾讯云 API 密钥").Valid(false)
	wf.NewItem("frp config set_toml_path <路径>").Subtitle("设置 frpc.toml 路径").Valid(false)
	wf.NewItem("frp config set_sgid <安全组ID>").Subtitle("设置安全组 ID").Valid(false)
	wf.NewItem("frp config view").Subtitle("查看当前配置").Valid(false)
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

func viewConfig(wf *aw.Workflow) {
	cfg, _ := config.Load()
	wf.NewItem("frpc.toml 路径").Subtitle(cfg.FrpcTomlPath).Valid(false)
	wf.NewItem("安全组ID").Subtitle(cfg.SecurityGroupId).Valid(false)
	// 密钥部分
	id, err1 := config.GetSecretId()
	key, err2 := config.GetSecretKey()
	idShow := maskSecret(id)
	keyShow := maskSecret(key)
	if err1 != nil {
		idShow = "未设置"
	}
	if err2 != nil {
		keyShow = "未设置"
	}
	wf.NewItem("SecretId").Subtitle(idShow).Valid(false)
	wf.NewItem("SecretKey").Subtitle(keyShow).Valid(false)
	wf.SendFeedback()
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}
