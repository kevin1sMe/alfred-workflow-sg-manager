package workflow

import (
	"fmt"
	"os"
	"strings"

	"github.com/kevin1sMe/alfred-workflow-sg-manager/internal/config"
	"github.com/kevin1sMe/alfred-workflow-sg-manager/internal/log"

	"github.com/BurntSushi/toml"
	aw "github.com/deanishe/awgo"
)

func List(wf *aw.Workflow) {
	cfg, err := config.Load()
	if err != nil {
		log.Error("配置文件读取失败: %v", err)
		wf.FatalError(fmt.Errorf("配置文件读取失败: %v", err))
		return
	}
	tomlPath := cfg.FrpcTomlPath
	if tomlPath == "" {
		log.Error("frpc.toml 路径未配置")
		wf.NewItem("frpc.toml 路径未配置").Subtitle("请使用 'frp config set_toml_path' 设置").Valid(false).Icon(aw.IconWarning)
		wf.SendFeedback()
		return
	}
	if _, err := os.Stat(tomlPath); err != nil {
		log.Error("frpc.toml 文件不存在: %s, 错误: %v", tomlPath, err)
		wf.NewItem(fmt.Sprintf("frpc.toml 文件不存在: %s", tomlPath)).Subtitle(err.Error()).Valid(false).Icon(aw.IconError)
		wf.SendFeedback()
		return
	}
	if cfg.SecurityGroupId == "" {
		log.Error("安全组 ID 未配置")
		wf.NewItem("安全组 ID 未配置").Subtitle("请使用 'frp config set_sgid' 设置").Valid(false).Icon(aw.IconWarning)
		wf.SendFeedback()
		return
	}
	if cfg.Region == "" {
		log.Error("腾讯云区域未配置")
		wf.NewItem("腾讯云区域未配置").Subtitle("请使用 'frp config set_region' 设置").Valid(false).Icon(aw.IconWarning)
		wf.SendFeedback()
		return
	}

	secretID, err := config.GetSecretId()
	if err != nil {
		log.Error("获取 SecretId 失败: %v", err)
		wf.NewItem("获取 SecretId 失败").Subtitle("请使用 'frp config setup_keys' 设置").Valid(false).Icon(aw.IconError)
		wf.SendFeedback()
		return
	}
	secretKey, err := config.GetSecretKey()
	if err != nil {
		log.Error("获取 SecretKey 失败: %v", err)
		wf.NewItem("获取 SecretKey 失败").Subtitle("请使用 'frp config setup_keys' 设置").Valid(false).Icon(aw.IconError)
		wf.SendFeedback()
		return
	}
	if secretID == "" || secretKey == "" {
		log.Error("SecretId 或 SecretKey 未配置")
		wf.NewItem("SecretId 或 SecretKey 未配置").Subtitle("请使用 'frp config setup_keys' 设置").Valid(false).Icon(aw.IconWarning)
		wf.SendFeedback()
		return
	}
	var frpcConf SimpleFrpcConfig // 使用新的根配置结构体
	if _, err := toml.DecodeFile(tomlPath, &frpcConf); err != nil {
		log.Error("frpc.toml 解析失败: %s, 错误: %v", tomlPath, err)
		wf.NewItem(fmt.Sprintf("frpc.toml 解析失败: %s", tomlPath)).Subtitle(err.Error()).Valid(false).Icon(aw.IconError)
		wf.SendFeedback()
		return
	}

	// 获取所有规则（包括ACCEPT和DROP）
	allRules, err := getAllSecurityGroupRules(cfg, secretID, secretKey)
	if err != nil {
		log.Error("获取所有安全组规则失败: %v", err)
		wf.NewItem("获取所有安全组规则失败").Subtitle(err.Error()).Valid(false).Icon(aw.IconError)
		wf.SendFeedback()
		return
	}

	// 过滤出 Action == "ACCEPT" 的规则，生成 openedPorts
	openedPorts := make(map[string]FetchedRuleInfo)
	for proxyName, v := range allRules {
		if v.Action == "ACCEPT" {
			openedPorts[proxyName] = v
		}
	}

	for _, p := range frpcConf.Proxies { // 遍历 Proxies 切片
		actualServiceName := p.Name // 直接使用 Proxy 结构中的 Name
		if actualServiceName == "" {
			// 如果代理配置中没有 name 字段，可以跳过或记录一个警告
			wf.Warn(fmt.Sprintf("发现一个未命名的代理配置，已跳过: LocalPort=%d, RemotePort=%d", p.LocalPort, p.RemotePort), "")
			log.Warn("发现一个未命名的代理配置，已跳过: LocalPort=%d, RemotePort=%d", p.LocalPort, p.RemotePort)
			continue
		}

		// 检查 p.Type, p.RemotePort 是否有效
		if p.Type == "" || p.RemotePort == 0 {
			wf.Warn(fmt.Sprintf("跳过无效的代理配置: %s (Type: %s, RemotePort: %d)", actualServiceName, p.Type, p.RemotePort), "")
			continue
		}

		isDrop := false
		isOpen := false
		var dropRule, openRule FetchedRuleInfo
		if ruleInfo, ok := allRules[actualServiceName]; ok {
			if ruleInfo.Action == "DROP" {
				isDrop = true
				dropRule = ruleInfo
			} else if ruleInfo.Action == "ACCEPT" {
				isOpen = true
				openRule = ruleInfo
			}
		}
		title := fmt.Sprintf("%s [%s]", actualServiceName, strings.ToUpper(p.Type))
		subtitle := fmt.Sprintf("远程端口:%d  本地端口:%d | 状态: ", p.RemotePort, p.LocalPort)
		var displayTitle string
		var policyDescription, lastMod string
		if isDrop {
			displayTitle = IconDrop + " " + title
			subtitle += "已拒绝(DROP)"
			policyDescription = dropRule.PolicyDescription
			lastMod = dropRule.ModifyTime
		} else if isOpen {
			displayTitle = IconOpen + " " + title
			subtitle += "已开放"
			policyDescription = openRule.PolicyDescription
			lastMod = openRule.ModifyTime
		} else {
			displayTitle = IconUnknown + " " + title
			subtitle += "未开放"
		}
		if lastMod != "" {
			lastMod = "最后修改时间: " + lastMod
		}

		log.Debug("actualServiceName: %s, p.Type: %s, p.RemotePort: %d, policyDescription: %s, lastMod: %s", actualServiceName, p.Type, p.RemotePort, policyDescription, lastMod)
		item := wf.NewItem(displayTitle).
			Subtitle(subtitle).
			Arg(fmt.Sprintf("%s %s %d", actualServiceName, strings.ToUpper(p.Type), p.RemotePort)).
			Valid(false)
		item.NewModifier(aw.ModCmd).
			Subtitle(fmt.Sprintf("%s %s", policyDescription, lastMod))
	}

	wf.SendFeedback()
}
