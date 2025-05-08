package main

import (
	"fmt"
	"os"
	"strings"

	aw "github.com/deanishe/awgo"
	"github.com/kevin1sMe/alfred-workflow-sg-manager/internal/config"
	"github.com/kevin1sMe/alfred-workflow-sg-manager/internal/log"
	"github.com/kevin1sMe/alfred-workflow-sg-manager/internal/workflow"
)

func ensureAlfredEnv() {
	if os.Getenv("alfred_workflow_bundleid") == "" {
		os.Setenv("alfred_workflow_bundleid", "dev.test")
	}
	if os.Getenv("alfred_workflow_cache") == "" {
		os.Setenv("alfred_workflow_cache", "/tmp")
	}
	if os.Getenv("alfred_workflow_data") == "" {
		os.Setenv("alfred_workflow_data", "/tmp")
	}
}

func main() {
	ensureAlfredEnv()
	wf := aw.New()

	// 初始化配置
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	// 如果没有设置日志路径，使用默认路径
	logPath := cfg.LogPath
	if logPath == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			logPath = homeDir + "/.alfred-frp-sg/alfred-frp-sg.log"
		} else {
			logPath = "/tmp/alfred-frp-sg.log"
		}
		// 更新配置
		cfg.LogPath = logPath
		config.Save(cfg)
	}

	// 使用INFO级别初始化日志
	log.Init(logPath, log.INFO)
	log.Info("Alfred Workflow FRP 安全组助手启动")

	log.Info("os.Args: %#v, wf.Args(): %#v", os.Args, wf.Args())
	args := strings.Fields(strings.Join(os.Args, " "))
	wf.Run(func() {
		if len(args) > 1 && args[1] == "list" {
			workflow.List(wf)
		} else if len(args) > 1 && args[1] == "config" {
			workflow.ConfigCommand(wf, args[1:])
		} else {
			wf.NewItem("用法: alfred-frp-sg list | config").Subtitle("支持 list/config 子命令").Valid(false)
			wf.SendFeedback()
		}
	})
}
