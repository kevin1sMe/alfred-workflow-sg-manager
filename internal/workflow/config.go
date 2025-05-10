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

	// 只保留 setup_secretid 和 setup_secretkey 逻辑
	sub := args[1]
	switch sub {
	case "setup_secretid":
		setupSecretId(wf, args)
	case "setup_secretkey":
		setupSecretKey(wf, args)
	default:
		showConfigHelp(wf)
	}
}

func showConfigHelp(wf *aw.Workflow) {
	cfg, _ := config.Load()

	// SecretId
	id, _ := config.GetSecretId()
	idShow := "未设置"
	idTitle := "设置 SecretId"
	if id != "" {
		idShow = maskSecret(id)
		idTitle = "修改 SecretId"
	}
	wf.NewItem(idTitle).
		Subtitle(idShow).
		Valid(true).
		Arg("setup_secretid")

	// SecretKey
	key, _ := config.GetSecretKey()
	keyShow := "未设置"
	keyTitle := "设置 SecretKey"
	if key != "" {
		keyShow = maskSecret(key)
		keyTitle = "修改 SecretKey"
	}
	wf.NewItem(keyTitle).
		Subtitle(keyShow).
		Valid(true).
		Arg("setup_secretkey")

	// frpc.toml 路径
	tomlPath := "未设置"
	if cfg.FrpcTomlPath != "" {
		tomlPath = cfg.FrpcTomlPath
	}
	wf.NewItem("🔒 frpc.toml 路径").
		Subtitle(tomlPath).
		Valid(false)

	// 安全组ID
	sgid := "未设置"
	if cfg.SecurityGroupId != "" {
		sgid = cfg.SecurityGroupId
	}
	wf.NewItem("🔒 安全组 ID").
		Subtitle(sgid).
		Valid(false)

	// 区域
	region := "未设置"
	if cfg.Region != "" {
		region = cfg.Region
	}
	wf.NewItem("🔒 腾讯云 API 区域").
		Subtitle(region).
		Valid(false)

	// 日志路径
	logPath := "未设置"
	if cfg.LogPath != "" {
		logPath = cfg.LogPath
	}
	wf.NewItem("🔒 日志路径").
		Subtitle(logPath).
		Valid(false)

	// 提示
	if id == "" || key == "" {
		wf.NewItem("请先设置 SecretId 和 SecretKey，否则无法正常使用。").Valid(false)
	}
	wf.NewItem("如需修改 region、frpc.toml 路径等参数，请在 Alfred 的 Workflow 配置面板中设置。").Valid(false)

	wf.SendFeedback()
}

func setupSecretId(wf *aw.Workflow, args []string) {
	if len(args) < 3 {
		wf.NewItem("请输入 SecretId 后回车").Valid(false)
		wf.SendFeedback()
		return
	}
	secretId := args[2]
	err := config.SaveSecretId(secretId)
	if err != nil {
		wf.NewItem("保存 SecretId 失败").Subtitle(err.Error()).Valid(false)
	} else {
		wf.NewItem("SecretId 保存成功").Valid(false)
	}
	wf.SendFeedback()
}

func setupSecretKey(wf *aw.Workflow, args []string) {
	if len(args) < 3 {
		wf.NewItem("请输入 SecretKey 后回车").Valid(false)
		wf.SendFeedback()
		return
	}
	secretKey := args[2]
	err := config.SaveSecretKey(secretKey)
	if err != nil {
		wf.NewItem("保存 SecretKey 失败").Subtitle(err.Error()).Valid(false)
	} else {
		wf.NewItem("SecretKey 保存成功").Valid(false)
	}
	wf.SendFeedback()
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:4] + strings.Repeat("*", 4) + s[len(s)-4:]
}
