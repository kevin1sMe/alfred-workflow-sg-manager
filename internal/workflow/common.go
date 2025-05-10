package workflow

import (
	"fmt"
	"strings"

	"github.com/kevin1sMe/alfred-workflow-sg-manager/internal/config"
	"github.com/kevin1sMe/alfred-workflow-sg-manager/internal/log"

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
	ModifyTime        string
	Action            string
	LocalPort         string
}

// getAllSecurityGroupRules 获取所有安全组规则（无论Accept还是Drop）
func getAllSecurityGroupRules(cfg *config.Config, secretID, secretKey string) (map[string]FetchedRuleInfo, error) {
	cred := common.NewCredential(secretID, secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "vpc.tencentcloudapi.com"
	client, err := vpc.NewClient(cred, cfg.Region, cpf)
	if err != nil {
		log.Error("创建腾讯云 VPC 客户端失败: %v", err)
		return nil, fmt.Errorf("创建腾讯云 VPC 客户端失败: %w", err)
	}

	log.Info("开始查询所有安全组规则, 安全组ID: %s", cfg.SecurityGroupId)
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

	allRules := make(map[string]FetchedRuleInfo)

	if response.Response != nil && response.Response.SecurityGroupPolicySet != nil {
		log.Info("成功获取安全组规则，开始解析所有规则")
		for _, policy := range response.Response.SecurityGroupPolicySet.Ingress {
			if policy.PolicyDescription != nil && strings.HasPrefix(*policy.PolicyDescription, "AlfredFRP_") {
				if policy.Protocol != nil && policy.Port != nil && policy.CidrBlock != nil && policy.Action != nil {
					proxyName := extractServiceName(*policy.PolicyDescription)
					localPort := extractLocalPort(*policy.PolicyDescription)

					var policyIndex int64 = -1
					if policy.PolicyIndex != nil {
						policyIndex = *policy.PolicyIndex
					}

					desc := ""
					if policy.PolicyDescription != nil {
						desc = *policy.PolicyDescription
					}

					action := ""
					if policy.Action != nil {
						action = *policy.Action
					}

					mtime := ""
					if policy.ModifyTime != nil {
						mtime = *policy.ModifyTime
					}

					log.Debug("找到规则: %s, 协议: %s, 端口: %s, CIDR: %s, 动作: %s, 描述: %s, 索引: %d, 修改时间: %s, 本地端口: %s",
						proxyName, *policy.Protocol, *policy.Port, *policy.CidrBlock, action, desc, policyIndex, mtime, localPort)

					allRules[proxyName] = FetchedRuleInfo{
						PolicyDescription: desc,
						Protocol:          strings.ToUpper(*policy.Protocol),
						Port:              *policy.Port,
						CidrBlock:         *policy.CidrBlock,
						PolicyIndex:       policyIndex,
						ModifyTime:        mtime,
						Action:            action,
						LocalPort:         localPort,
					}
				}
			}
		}
		log.Info("解析完成，找到 %d 个符合条件的规则", len(allRules))
	} else {
		log.Warn("API响应为空或没有安全组策略集")
	}
	return allRules, nil
}

// 从策略描述中提取服务名称
func extractServiceName(description string) string {
	// 预期格式: AlfredFRP_服务名_local端口
	if !strings.HasPrefix(description, "AlfredFRP_") {
		return "未知服务"
	}

	// 移除AlfredFRP_前缀
	nameWithPort := strings.TrimPrefix(description, "AlfredFRP_")

	// 查找_local分隔符
	idx := strings.LastIndex(nameWithPort, "_local")
	if idx == -1 {
		// 检查是否有_blocked后缀
		idx = strings.LastIndex(nameWithPort, "_blocked")
		if idx == -1 {
			return nameWithPort // 如果没有local或blocked部分，返回整个名称
		}
		return nameWithPort[:idx]
	}

	return nameWithPort[:idx]
}

// 从策略描述中提取本地端口
func extractLocalPort(description string) string {
	// 预期格式: AlfredFRP_服务名_local端口
	idx := strings.LastIndex(description, "_local")
	if idx == -1 {
		return "未知" // 如果没有local部分，返回未知
	}

	return description[idx+6:] // +6是为了跳过"_local"
}
