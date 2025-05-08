package workflow

import (
	"fmt"
	"io"
	"net"
	"net/http"
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

// OpenCommand 显示未开放的服务列表
func OpenCommand(wf *aw.Workflow) {
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
	var frpcConf SimpleFrpcConfig // 使用与list.go相同的配置结构体
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

	// 获取所有规则（包括ACCEPT和DROP）
	allRules, err := getAllSecurityGroupRules(cfg, secretID, secretKey)
	if err != nil {
		log.Error("获取所有安全组规则失败: %v", err)
		wf.NewItem("获取所有安全组规则失败").Subtitle(err.Error()).Valid(false).Icon(aw.IconError)
		wf.SendFeedback()
		return
	}

	log.Info("openedRules: %v", openedRules)
	log.Info("allRules: %v", allRules)
	if len(frpcConf.Proxies) == 0 {
		log.Error("frpc.toml 中未找到任何 [[proxies]] 定义")
		wf.NewItem("frpc.toml 中未找到任何 [[proxies]] 定义").Subtitle("请确保 frpc.toml 中包含 [[proxies]] 配置块").Valid(false).Icon(aw.IconInfo)
		wf.SendFeedback()
		return
	}

	hasUnopened := false
	for _, p := range frpcConf.Proxies {
		actualServiceName := p.Name
		if actualServiceName == "" {
			log.Warn("发现一个未命名的代理配置，已跳过: LocalPort=%d, RemotePort=%d", p.LocalPort, p.RemotePort)
			continue
		}

		// 检查 p.Type, p.RemotePort 是否有效
		if p.Type == "" || p.RemotePort == 0 {
			log.Warn("跳过无效的代理配置: %s (Type: %s, RemotePort: %d)", actualServiceName, p.Type, p.RemotePort)
			continue
		}

		proto := "TCP" // 默认 TCP
		if strings.ToUpper(p.Type) == "UDP" {
			proto = "UDP"
		}

		// API 返回的 Port 是字符串，frpc.toml 的 RemotePort 是 int
		key := fmt.Sprintf("%s:%d", proto, p.RemotePort)
		isOpen := false

		if _, ok := openedRules[key]; ok {
			isOpen = true
		}

		// 检查是否存在对应的DROP规则
		dropKey := fmt.Sprintf("%s:%d:%s", proto, p.RemotePort, "") // 不关心CIDR
		hasDropRule := false
		for ruleKey, ruleInfo := range allRules {
			if strings.HasPrefix(ruleKey, dropKey) && ruleInfo.Action == "DROP" {
				// 如果有DROP规则，标记它，但不视为已开放
				hasDropRule = true
				log.Info("服务 %s [%s:%d] 存在拒绝规则，视为未开放", actualServiceName, proto, p.RemotePort)
				break
			}
		}

		// 只显示未开放的服务（包括DROP状态的规则）
		if !isOpen || hasDropRule {
			hasUnopened = true
			title := fmt.Sprintf("%s [%s]", actualServiceName, proto)
			subtitle := ""

			if hasDropRule {
				subtitle = fmt.Sprintf("远程端口:%d  本地端口:%d | 状态: 已拒绝(DROP)",
					p.RemotePort, p.LocalPort)
				item := wf.NewItem("🔒 "+title).
					Subtitle(subtitle).
					Arg(fmt.Sprintf("open %s|%s|%d|%d", actualServiceName, proto, p.RemotePort, p.LocalPort)).
					Valid(true).
					Icon(aw.IconWarning).
					Var("action", "open")

				// 添加mod键功能，显示更多信息
				item.NewModifier(aw.ModCmd).
					Subtitle("显示更多详细信息")
			} else {
				subtitle = fmt.Sprintf("远程端口:%d  本地端口:%d | 状态: 未开放",
					p.RemotePort, p.LocalPort)
				item := wf.NewItem("➕ "+title).
					Subtitle(subtitle).
					Arg(fmt.Sprintf("open %s|%s|%d|%d", actualServiceName, proto, p.RemotePort, p.LocalPort)).
					Valid(true).
					Icon(aw.IconWarning).
					Var("action", "open")

				// 添加mod键功能，显示更多信息
				item.NewModifier(aw.ModCmd).
					Subtitle("显示更多详细信息")
			}
		}
	}

	if !hasUnopened {
		wf.NewItem("所有服务已在安全组中开放").Subtitle("没有需要开放的新服务").Valid(false).Icon(aw.IconInfo)
	}

	wf.SendFeedback()
}

// OpenPort 开放指定的端口
func OpenPort(wf *aw.Workflow, args []string) {
	// 检查参数格式，需要接收服务名称|协议|远程端口|本地端口
	if len(args) < 1 {
		log.Error("缺少参数，期望格式: 服务名|协议|远程端口|本地端口")
		wf.NewItem("参数错误").Subtitle("缺少参数，期望格式: 服务名|协议|远程端口|本地端口").Icon(aw.IconError)
		wf.SendFeedback()
		return
	}

	parts := strings.Split(args[0], "|")
	if len(parts) < 4 {
		log.Error("参数格式错误，期望格式: 服务名|协议|远程端口|本地端口，实际: %s", args[0])
		wf.NewItem("参数格式错误").Subtitle("期望格式: 服务名|协议|远程端口|本地端口").Icon(aw.IconError)
		wf.SendFeedback()
		return
	}

	serviceName := parts[0]
	protocol := parts[1]
	remotePort := parts[2]
	localPort := parts[3]

	log.Info("开放端口，服务名: %s, 协议: %s, 远程端口: %s, 本地端口: %s", serviceName, protocol, remotePort, localPort)

	cfg, err := config.Load()
	if err != nil {
		log.Error("配置文件读取失败: %v", err)
		wf.FatalError(fmt.Errorf("配置文件读取失败: %v", err))
		return
	}

	// 获取当前公网IP
	currentIP, err := getCurrentPublicIP()
	if err != nil {
		log.Error("获取公网IP失败: %v", err)
		wf.NewItem("获取公网IP失败").Subtitle(err.Error()).Icon(aw.IconError)
		wf.SendFeedback()
		return
	}

	secretID, _ := config.GetSecretId()
	secretKey, _ := config.GetSecretKey()

	// 为端口规则创建说明标识
	ruleTag := fmt.Sprintf("AlfredFRP_%s_local%s", serviceName, localPort)

	// 调用腾讯云API创建安全组规则
	err = createSecurityGroupRule(cfg, secretID, secretKey, protocol, remotePort, currentIP, ruleTag)
	if err != nil {
		log.Error("创建安全组规则失败: %v", err)
		wf.NewItem("创建安全组规则失败").Subtitle(err.Error()).Icon(aw.IconError)
		wf.SendFeedback()
		return
	}

	// 操作成功
	wf.NewItem(fmt.Sprintf("已成功开放服务: %s", serviceName)).
		Subtitle(fmt.Sprintf("协议: %s, 远程端口: %s, 本地端口: %s, IP: %s", protocol, remotePort, localPort, currentIP)).
		Icon(aw.IconInfo)
	wf.SendFeedback()
}

// getCurrentPublicIP 获取当前公网IP
func getCurrentPublicIP() (string, error) {
	// 尝试从多个服务获取公网IP，以提高可靠性
	ipServices := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
		"https://ipinfo.io/ip",
	}

	var lastErr error
	for _, service := range ipServices {
		resp, err := http.Get(service)
		if err != nil {
			lastErr = err
			log.Warn("从 %s 获取IP失败: %v", service, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP 状态码非正常: %d", resp.StatusCode)
			log.Warn("从 %s 获取IP失败: HTTP状态码 %d", service, resp.StatusCode)
			continue
		}

		// 读取完整响应
		ipBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			log.Warn("从 %s 读取响应失败: %v", service, err)
			continue
		}

		// 清理IP地址（去除空白字符）
		ip := strings.TrimSpace(string(ipBytes))

		// 验证是否为有效IP地址
		if net.ParseIP(ip) != nil {
			log.Info("成功从 %s 获取公网IP: %s", service, ip)
			return ip, nil
		}

		lastErr = fmt.Errorf("获取到无效的IP地址: %s", ip)
		log.Warn("从 %s 获取到无效的IP格式: %s", service, ip)
	}

	if lastErr != nil {
		return "", fmt.Errorf("无法获取公网IP: %w", lastErr)
	}
	return "", fmt.Errorf("所有IP服务均失败")
}

// createSecurityGroupRule 创建安全组规则
func createSecurityGroupRule(cfg *config.Config, secretID, secretKey, protocol, port, ip, description string) error {
	log.Info("开始创建安全组规则, 协议: %s, 端口: %s, IP: %s, 描述: %s", protocol, port, ip, description)

	// 创建腾讯云VPC客户端
	cred := common.NewCredential(secretID, secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "vpc.tencentcloudapi.com"
	client, err := vpc.NewClient(cred, cfg.Region, cpf)
	if err != nil {
		return fmt.Errorf("创建腾讯云VPC客户端失败: %w", err)
	}

	// 添加/32子网掩码
	cidrBlock := ip + "/32"

	// 先获取所有现有规则，查找服务名相同的规则
	allRules, err := getAllSecurityGroupRules(cfg, secretID, secretKey)
	if err == nil {
		// 从description中提取服务名
		serviceName := ""
		if strings.HasPrefix(description, "AlfredFRP_") {
			parts := strings.Split(description, "_local")
			if len(parts) > 0 {
				serviceName = strings.TrimPrefix(parts[0], "AlfredFRP_")
			}
		}

		// 如果找到服务名，寻找匹配的规则并删除
		if serviceName != "" {
			for key, rule := range allRules {
				// 检查是否是同名服务，不论协议和端口
				ruleServiceName := extractServiceName(rule.PolicyDescription)
				if ruleServiceName == serviceName && strings.HasPrefix(key, fmt.Sprintf("%s:%s:", protocol, port)) {
					log.Info("找到匹配的规则需要删除: %s, PolicyIndex: %d", key, rule.PolicyIndex)

					// 创建删除规则请求
					deleteRequest := vpc.NewDeleteSecurityGroupPoliciesRequest()
					deleteRequest.SecurityGroupId = common.StringPtr(cfg.SecurityGroupId)

					// 设置要删除的规则
					deletePolicySet := &vpc.SecurityGroupPolicySet{
						Ingress: []*vpc.SecurityGroupPolicy{
							{
								PolicyIndex: common.Int64Ptr(rule.PolicyIndex),
							},
						},
					}
					deleteRequest.SecurityGroupPolicySet = deletePolicySet

					// 发送删除请求
					deleteResponse, err := client.DeleteSecurityGroupPolicies(deleteRequest)
					if err != nil {
						log.Warn("删除旧规则失败，将继续创建新规则: %v", err)
					} else {
						log.Info("成功删除旧规则，响应: %s", deleteResponse.ToJsonString())
					}
				}
			}
		}
	}

	// 创建安全组规则请求
	request := vpc.NewCreateSecurityGroupPoliciesRequest()
	request.SecurityGroupId = common.StringPtr(cfg.SecurityGroupId)

	// 创建入站规则
	policySet := &vpc.SecurityGroupPolicySet{
		Ingress: []*vpc.SecurityGroupPolicy{
			{
				Protocol:          common.StringPtr(protocol),
				Port:              common.StringPtr(port),
				CidrBlock:         common.StringPtr(cidrBlock),
				Action:            common.StringPtr("ACCEPT"),
				PolicyDescription: common.StringPtr(description),
			},
		},
	}
	request.SecurityGroupPolicySet = policySet

	// 发送API请求
	response, err := client.CreateSecurityGroupPolicies(request)
	if err != nil {
		if sdkErr, ok := err.(*tcErrors.TencentCloudSDKError); ok {
			return fmt.Errorf("腾讯云API错误: Code=%s, Message=%s, RequestId=%s", sdkErr.GetCode(), sdkErr.GetMessage(), sdkErr.GetRequestId())
		}
		return fmt.Errorf("调用腾讯云API失败: %w", err)
	}

	log.Info("创建安全组规则成功，响应: %s", response.ToJsonString())
	return nil
}
