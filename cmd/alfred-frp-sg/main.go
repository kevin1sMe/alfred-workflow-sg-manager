package main

import (
	"fmt"
	"os"

	aw "github.com/deanishe/awgo"
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
	fmt.Fprintf(os.Stderr, "os.Args: %#v\n", os.Args)
	wf.Run(func() {
		args := os.Args
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
