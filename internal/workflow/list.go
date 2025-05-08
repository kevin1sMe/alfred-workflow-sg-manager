package workflow

import (
	"fmt"
	"os"
	"strings"

	"github.com/kevin1sMe/alfred-workflow-sg-manager/internal/config"
	"github.com/kevin1sMe/alfred-workflow-sg-manager/internal/log"

	"github.com/BurntSushi/toml"
	aw "github.com/deanishe/awgo"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcErrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	vpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
)

type SimpleFrpcConfig struct {
	Proxies []Proxy `toml:"proxies"`
}

type Proxy struct {
	Name       string `toml:"name"`
	Type       string `toml:"type"`
	LocalIP    string `toml:"localIP"`
	LocalPort  int    `toml:"localPort"`
	RemotePort int    `toml:"remotePort"`
	// 可以根据 frpc.toml 示例按需添加其他代理特有的字段
	// 例如: transport.bandwidthLimit, healthCheck 等。
	// metadatas 和 annotations 也可以作为 map[string]string 或更具体的结构体添加进来
}

// FetchedRuleInfo 存储从API获取并处理后的规则信息
type FetchedRuleInfo struct {
	PolicyDescription string
	Protocol          string
	Port              string
	CidrBlock         string
	PolicyIndex       int64
}

// getRealOpenedPorts 从腾讯云 API 获取安全组规则
func getRealOpenedPorts(cfg *config.Config, secretID, secretKey string) (map[string]FetchedRuleInfo, error) {
	if cfg.SecurityGroupId == "" {
		log.Error("安全组 ID 未配置")
		return nil, fmt.Errorf("安全组 ID 未配置")
	}
	if cfg.Region == "" {
		log.Error("腾讯云区域未配置")
		return nil, fmt.Errorf("腾讯云区域未配置")
	}
	if secretID == "" || secretKey == "" {
		log.Error("腾讯云 SecretId 或 SecretKey 未配置")
		return nil, fmt.Errorf("腾讯云 SecretId 或 SecretKey 未配置")
	}

	log.Info("正在创建腾讯云 VPC 客户端，区域: %s", cfg.Region)
	cred := common.NewCredential(secretID, secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "vpc.tencentcloudapi.com" // VPC API 的接入点
	client, err := vpc.NewClient(cred, cfg.Region, cpf)
	if err != nil {
		log.Error("创建腾讯云 VPC 客户端失败: %v", err)
		return nil, fmt.Errorf("创建腾讯云 VPC 客户端失败: %w", err)
	}

	log.Info("开始查询安全组规则, 安全组ID: %s", cfg.SecurityGroupId)
	request := vpc.NewDescribeSecurityGroupPoliciesRequest()
	request.SecurityGroupId = common.StringPtr(cfg.SecurityGroupId)

	response, err := client.DescribeSecurityGroupPolicies(request)
	if err != nil {
		if sdkErr, ok := err.(*tcErrors.TencentCloudSDKError); ok {
			log.Error("腾讯云 API 错误: Code=%s, Message=%s, RequestId=%s", sdkErr.GetCode(), sdkErr.GetMessage(), sdkErr.GetRequestId())
			return nil, fmt.Errorf("腾讯云 API 错误: Code=%s, Message=%s, RequestId=%s", sdkErr.GetCode(), sdkErr.GetMessage(), sdkErr.GetRequestId())
		}
		log.Error("调用腾讯云 API 失败: %v", err)
		return nil, fmt.Errorf("调用腾讯云 API 失败: %w", err)
	}

	openedPorts := make(map[string]FetchedRuleInfo)
	if response.Response != nil && response.Response.SecurityGroupPolicySet != nil {
		log.Info("成功获取安全组规则，开始解析入站规则")
		for _, policy := range response.Response.SecurityGroupPolicySet.Ingress {
			if policy.PolicyDescription != nil && strings.HasPrefix(*policy.PolicyDescription, "AlfredFRP_") {
				if policy.Protocol != nil && policy.Port != nil {
					key := fmt.Sprintf("%s:%s", strings.ToUpper(*policy.Protocol), *policy.Port) // 协议转大写，端口是字符串
					var cidr, desc string
					var pIndex int64 = -1 // 默认值，如果不存在
					if policy.CidrBlock != nil {
						cidr = *policy.CidrBlock
					}
					if policy.PolicyDescription != nil {
						desc = *policy.PolicyDescription
					}
					if policy.PolicyIndex != nil {
						pIndex = *policy.PolicyIndex
					}

					log.Debug("找到符合条件的规则: %s, 协议: %s, 端口: %s, CIDR: %s, 描述: %s", key, *policy.Protocol, *policy.Port, cidr, desc)
					openedPorts[key] = FetchedRuleInfo{
						PolicyDescription: desc,
						Protocol:          strings.ToUpper(*policy.Protocol),
						Port:              *policy.Port,
						CidrBlock:         cidr,
						PolicyIndex:       pIndex,
					}
				}
			}
		}
		log.Info("解析完成，找到 %d 个符合条件的入站规则", len(openedPorts))
	} else {
		log.Warn("API响应为空或没有安全组策略集")
	}
	return openedPorts, nil
}

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

	log.Info("frpcConf.proxies: %v", frpcConf.Proxies)
	openedRules, err := getRealOpenedPorts(cfg, secretID, secretKey)
	if err != nil {
		log.Error("获取安全组规则失败: %v", err)
		wf.NewItem("获取安全组规则失败").Subtitle(err.Error()).Valid(false).Icon(aw.IconError)
		wf.SendFeedback()
		return
	}

	log.Info("openedRules: %v", openedRules)
	if len(frpcConf.Proxies) == 0 { // 检查 Proxies 切片是否为空
		log.Error("frpc.toml 中未找到任何 [[proxies]] 定义")
		wf.NewItem("frpc.toml 中未找到任何 [[proxies]] 定义").Subtitle("请确保 frpc.toml 中包含 [[proxies]] 配置块").Valid(false).Icon(aw.IconInfo)
		wf.SendFeedback()
		return
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

		proto := "TCP" // 默认 TCP
		if strings.ToUpper(p.Type) == "UDP" {
			proto = "UDP"
		}

		// API 返回的 Port 是字符串，frpc.toml 的 RemotePort 是 int
		key := fmt.Sprintf("%s:%d", proto, p.RemotePort)
		status := "未开放"
		isOpen := false
		ruleInfoText := ""

		if rule, ok := openedRules[key]; ok {
			// 进一步检查 PolicyDescription 是否与服务名相关，如果需要的话
			// 例如，如果 PolicyDescription 是 "AlfredFRP_服务名_端口"
			// if strings.Contains(rule.PolicyDescription, actualServiceName) {
			status = "已开放"
			isOpen = true
			ruleInfoText = fmt.Sprintf(" | IP: %s (描述: %s)", rule.CidrBlock, rule.PolicyDescription)
			// }
		}

		title := fmt.Sprintf("%s [%s]", actualServiceName, proto)
		subtitle := fmt.Sprintf("远程端口:%d  本地端口:%d | 状态: %s%s",
			p.RemotePort, p.LocalPort, status, ruleInfoText)

		item := wf.NewItem(title).
			Subtitle(subtitle).
			Arg(fmt.Sprintf("%s %s %d", actualServiceName, proto, p.RemotePort)). // 为后续操作（如 close）准备参数
			Valid(true)                                                           // 使其可选，以便后续操作

		if isOpen {
			item.Icon(aw.IconFavorite) // 使用 awgo 内置图标
		} else {
			item.Icon(aw.IconWarning) // 使用 aw.IconWarning 替代 aw.IconRemove
		}
	}

	// 用户反馈：即使没有规则匹配，也应列出所有服务，而不是显示统一提示。
	// 原来的逻辑是如果 wf.IsEmpty() 且 frpcConf.Proxies 有内容，则显示 "所有服务未开放" 的消息。
	// 当前循环逻辑会为每个服务（无论是否开放）创建条目，所以如果 frpcConf.Proxies 有内容，
	// wf.IsEmpty() 通常不应为 true，除非所有代理条目因其他条件被跳过。
	// 为确保符合用户要求，移除此特定检查。
	// if wf.IsEmpty() && len(frpcConf.Proxies) > 0 {
	// 	wf.NewItem("所有有效的 frpc.toml 服务均未在安全组中开放 (或规则不匹配)").Valid(false).Icon(aw.IconInfo)
	// 	wf.SendFeedback()
	// 	return
	// }

	// 这部分检查 frpcConf.Proxies 为空的情况，已在前面 (line 164-169) 处理过，可以移除或保留作为双重检查。
	// if wf.IsEmpty() && len(frpcConf.Proxies) == 0 {
	// // 此情况已在前面处理
	// }

	wf.SendFeedback()
}
