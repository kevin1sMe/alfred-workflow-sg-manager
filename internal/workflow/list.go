package workflow

import (
	"fmt"
	"os"

	"github.com/kevin1sMe/alfred-workflow-sg-manager/internal/config"

	"github.com/BurntSushi/toml"
	aw "github.com/deanishe/awgo"
)

type Proxy struct {
	Name       string `toml:"name"`
	Type       string `toml:"type"`
	LocalIP    string `toml:"localIP"`
	LocalPort  int    `toml:"localPort"`
	RemotePort int    `toml:"remotePort"`
}

type FrpcConfig struct {
	Proxies []Proxy `toml:"proxies"`
}

// Mock: 查询安全组规则，返回已开放的端口和协议
func getOpenedPorts() map[string]struct{} {
	// key: "protocol:port" 例如 "TCP:8022"
	return map[string]struct{}{
		"TCP:8022": {},
		"TCP:3306": {},
	}
}

func List(wf *aw.Workflow) {
	// 读取配置
	cfg, err := config.Load()
	if err != nil {
		wf.FatalError(fmt.Errorf("配置文件读取失败: %v", err))
	}
	tomlPath := cfg.FrpcTomlPath
	if tomlPath == "" {
		wf.FatalError(fmt.Errorf("frpc.toml 路径未配置"))
	}
	if _, err := os.Stat(tomlPath); err != nil {
		wf.FatalError(fmt.Errorf("frpc.toml 文件不存在: %v", err))
	}

	var conf FrpcConfig
	if _, err := toml.DecodeFile(tomlPath, &conf); err != nil {
		wf.FatalError(fmt.Errorf("frpc.toml 解析失败: %v", err))
	}

	opened := getOpenedPorts()

	for _, p := range conf.Proxies {
		proto := "TCP"
		if p.Type == "udp" || p.Type == "UDP" {
			proto = "UDP"
		}
		key := fmt.Sprintf("%s:%d", proto, p.RemotePort)
		status := "未开放"
		isOpen := false

		if _, ok := opened[key]; ok {
			status = "已开放"
			isOpen = true
		}

		// 只在主标题显示服务名和协议
		title := fmt.Sprintf("%s [%s]", p.Name, proto)

		// 将端口信息放在副标题中
		subtitle := fmt.Sprintf("远程端口:%d 本地端口:%d | 状态: %s",
			p.RemotePort, p.LocalPort, status)

		item := wf.NewItem(title).
			Subtitle(subtitle).
			Valid(false)

		// 根据状态设置不同图标
		if isOpen {
			item.Icon(&aw.Icon{Value: "/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/ToolbarFavoritesIcon.icns", Type: ""}) // 已开放使用星星图标
		} else {
			item.Icon(&aw.Icon{Value: "/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/AlertStopIcon.icns", Type: ""}) // 未开放使用停止图标
		}
	}

	wf.SendFeedback()
}
