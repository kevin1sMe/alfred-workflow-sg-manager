package workflow

import (
	"fmt"
	"os"
	"path/filepath"

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
	// 读取 frpc.toml 路径（此处假设为 test/frpc.toml，后续可从配置读取）
	tomlPath := filepath.Join("test", "frpc.toml")
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
		if _, ok := opened[key]; ok {
			status = "已开放"
		}
		title := fmt.Sprintf("%s [%s] 远程端口:%d 本地端口:%d", p.Name, proto, p.RemotePort, p.LocalPort)
		wf.NewItem(title).
			Subtitle(fmt.Sprintf("状态: %s", status)).
			Valid(false)
	}

	wf.SendFeedback()
}
