package workflow

import (
	"fmt"
	"strings"

	"github.com/kevin1sMe/alfred-workflow-sg-manager/internal/config"
	"github.com/kevin1sMe/alfred-workflow-sg-manager/internal/log"

	aw "github.com/deanishe/awgo"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcErrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	vpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
)

// CloseCommand 显示可关闭的端口规则列表
func CloseCommand(wf *aw.Workflow) {
	cfg, err := config.Load()
	if err != nil {
		log.Error("配置文件读取失败: %v", err)
		wf.FatalError(fmt.Errorf("配置文件读取失败: %v", err))
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

	// openedRules、allRules 都以 proxy name 作为 key
	allRules, err := getAllSecurityGroupRules(cfg, secretID, secretKey)
	if err != nil {
		log.Error("获取所有安全组规则失败: %v", err)
		wf.NewItem("获取所有安全组规则失败").Subtitle(err.Error()).Valid(false).Icon(aw.IconError)
		wf.SendFeedback()
		return
	}
	openedRules := make(map[string]FetchedRuleInfo)
	for proxyName, v := range allRules {
		if v.Action == "ACCEPT" {
			openedRules[proxyName] = v
		}
	}

	log.Info("所有规则详情: %+v", allRules)
	log.Info("openedRules规则: %v", openedRules)

	if len(openedRules) == 0 {
		log.Info("未找到由 Workflow 创建的规则")
		wf.NewItem("未找到任何已开放的规则").Subtitle("没有可关闭的规则").Valid(false).Icon(aw.IconInfo)
		wf.SendFeedback()
		return
	}

	// 显示可以关闭的规则列表
	hasValidRules := false
	for proxyName, rule := range openedRules {
		protocol := rule.Protocol
		port := rule.Port
		localPort := rule.LocalPort
		// serviceName := extractServiceName(rule.PolicyDescription)
		// localPort := extractLocalPort(rule.PolicyDescription)

		// 检查这个规则在云端是否已经是拒绝状态
		icon := IconOpen // 默认已开放
		hasValidRules = true
		title := fmt.Sprintf("%s [%s]", proxyName, protocol)
		subtitle := fmt.Sprintf("远程端口:%s  本地端口:%s | IP: %s", port, localPort, rule.CidrBlock)

		item := wf.NewItem(icon+" "+title).
			Subtitle(subtitle).
			Arg(fmt.Sprintf("close %s|%s|%s|%s|%d|%s", proxyName, protocol, port, rule.CidrBlock, rule.PolicyIndex, localPort)).
			Valid(true).
			Var("action", "close")

		// 添加mod键功能，显示更多信息
		modSubtitle := rule.PolicyDescription
		if rule.ModifyTime != "" {
			modSubtitle += "最后修改时间: " + rule.ModifyTime
		}
		if modSubtitle == "" {
			modSubtitle = "无描述信息"
		}
		item.NewModifier(aw.ModCmd).
			Subtitle(modSubtitle)
	}

	if !hasValidRules {
		wf.NewItem("未找到可关闭的规则").Subtitle("所有规则已经是拒绝状态或未开放").Valid(false).Icon(aw.IconInfo)
	}

	wf.SendFeedback()
}

// ClosePort 关闭指定的端口
func ClosePort(wf *aw.Workflow, args []string) {
	// 检查参数格式，需要接收服务名称|协议|远程端口|CIDR|PolicyIndex
	if len(args) < 1 {
		log.Error("缺少参数，期望格式: 服务名|协议|远程端口|CIDR|PolicyIndex")
		wf.NewItem("参数错误").Subtitle("缺少参数，期望格式: 服务名|协议|远程端口|CIDR|PolicyIndex").Icon(aw.IconError)
		wf.SendFeedback()
		return
	}

	parts := strings.Split(args[0], "|")
	if len(parts) < 5 {
		log.Error("参数格式错误，期望格式: 服务名|协议|远程端口|CIDR|PolicyIndex，实际: %s", args[0])
		wf.NewItem("参数格式错误").Subtitle("期望格式: 服务名|协议|远程端口|CIDR|PolicyIndex").Icon(aw.IconError)
		wf.SendFeedback()
		return
	}

	serviceName := parts[0]
	protocol := parts[1]
	remotePort := parts[2]
	cidrBlock := parts[3]
	policyIndexStr := parts[4]

	// 获取本地端口，如果参数中提供则直接使用
	localPort := "0"
	if len(parts) >= 6 {
		localPort = parts[5]
	}

	log.Info("关闭端口，服务名: %s, 协议: %s, 远程端口: %s, CIDR: %s, PolicyIndex: %s, 本地端口: %s",
		serviceName, protocol, remotePort, cidrBlock, policyIndexStr, localPort)

	cfg, err := config.Load()
	if err != nil {
		log.Error("配置文件读取失败: %v", err)
		wf.FatalError(fmt.Errorf("配置文件读取失败: %v", err))
		return
	}

	secretID, _ := config.GetSecretId()
	secretKey, _ := config.GetSecretKey()

	// 使用"创建拒绝规则-删除原规则"的方式关闭端口
	err = createDenyRuleAndDeleteOriginal(cfg, secretID, secretKey, protocol, remotePort, cidrBlock, serviceName, policyIndexStr, localPort)
	if err != nil {
		log.Error("关闭端口失败: %v", err)
		wf.NewItem("关闭端口失败").Subtitle(err.Error()).Icon(aw.IconError)
		wf.SendFeedback()
		return
	}

	// 操作成功
	wf.NewItem(fmt.Sprintf("已成功关闭服务: %s", serviceName)).
		Subtitle(fmt.Sprintf("协议: %s, 远程端口: %s, IP: %s", protocol, remotePort, cidrBlock)).
		Icon(&aw.Icon{Value: "/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/ToolbarDeleteIcon.icns"})
	wf.SendFeedback()
}

// createDenyRuleAndDeleteOriginal 先创建拒绝规则，再删除原规则
func createDenyRuleAndDeleteOriginal(cfg *config.Config, secretID, secretKey, protocol, port, cidrBlock, serviceName, policyIndexStr, localPort string) error {
	log.Info("开始创建拒绝规则并删除原规则, 协议: %s, 端口: %s, IP: %s", protocol, port, cidrBlock)

	// 创建腾讯云VPC客户端
	cred := common.NewCredential(secretID, secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "vpc.tencentcloudapi.com"
	client, err := vpc.NewClient(cred, cfg.Region, cpf)
	if err != nil {
		return fmt.Errorf("创建腾讯云VPC客户端失败: %w", err)
	}

	// 1. 创建对应规则的DROP版本
	description := fmt.Sprintf("AlfredFRP_%s_local%s", serviceName, localPort)
	log.Info("正在创建拒绝规则，保持原备注格式: %s", description)

	createRequest := vpc.NewCreateSecurityGroupPoliciesRequest()
	createRequest.SecurityGroupId = common.StringPtr(cfg.SecurityGroupId)

	// 创建拒绝规则
	createPolicySet := &vpc.SecurityGroupPolicySet{
		Ingress: []*vpc.SecurityGroupPolicy{
			{
				Protocol:          common.StringPtr(protocol),
				Port:              common.StringPtr(port),
				CidrBlock:         common.StringPtr(cidrBlock),
				Action:            common.StringPtr("DROP"),
				PolicyDescription: common.StringPtr(description),
			},
		},
	}
	createRequest.SecurityGroupPolicySet = createPolicySet

	// 发送API请求创建拒绝规则
	createResponse, err := client.CreateSecurityGroupPolicies(createRequest)
	if err != nil {
		if sdkErr, ok := err.(*tcErrors.TencentCloudSDKError); ok {
			return fmt.Errorf("腾讯云API错误(创建拒绝规则): Code=%s, Message=%s, RequestId=%s", sdkErr.GetCode(), sdkErr.GetMessage(), sdkErr.GetRequestId())
		}
		return fmt.Errorf("调用腾讯云API创建拒绝规则失败: %w", err)
	}

	log.Info("创建拒绝规则成功，响应: %s", createResponse.ToJsonString())

	// 2. 删除原有的ACCEPT规则
	log.Info("正在删除原有规则, PolicyIndex: %s", policyIndexStr)

	deleteRequest := vpc.NewDeleteSecurityGroupPoliciesRequest()
	deleteRequest.SecurityGroupId = common.StringPtr(cfg.SecurityGroupId)

	// 将 policyIndexStr 转为 int64
	var policyIndex int64
	_, err = fmt.Sscanf(policyIndexStr, "%d", &policyIndex)
	if err != nil {
		return fmt.Errorf("无效的PolicyIndex: %s, 错误: %w", policyIndexStr, err)
	}

	// 设置要删除的规则ID
	deletePolicySet := &vpc.SecurityGroupPolicySet{
		Ingress: []*vpc.SecurityGroupPolicy{
			{
				PolicyIndex: common.Int64Ptr(policyIndex),
			},
		},
	}
	deleteRequest.SecurityGroupPolicySet = deletePolicySet

	// 发送API请求删除原规则
	deleteResponse, err := client.DeleteSecurityGroupPolicies(deleteRequest)
	if err != nil {
		if sdkErr, ok := err.(*tcErrors.TencentCloudSDKError); ok {
			return fmt.Errorf("腾讯云API错误(删除原规则): Code=%s, Message=%s, RequestId=%s", sdkErr.GetCode(), sdkErr.GetMessage(), sdkErr.GetRequestId())
		}
		return fmt.Errorf("调用腾讯云API删除原规则失败: %w", err)
	}

	log.Info("删除原规则成功，响应: %s", deleteResponse.ToJsonString())
	return nil
}
