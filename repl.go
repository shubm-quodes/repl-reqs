package main

import (
	"fmt"
	"os"

	"github.com/shubm-quodes/repl-reqs/cmd"
	"github.com/shubm-quodes/repl-reqs/cmd/syscmd"
	"github.com/shubm-quodes/repl-reqs/config"
)

func main() {
	flags := config.InitializeFlags()
	flags.Process()

	cfg := config.Initialize(flags)
	reg := cmd.NewCmdRegistry()

	syscmd.RegisterCmds(reg)

	if cmdHandler, err := cmd.NewCmdHandler(cfg, config.GetShellCfg(cfg), reg); err != nil {
		fmt.Println("failed to initialize command handler", err)
		os.Exit(1)
	} else {
		if err := syscmd.InitNetCmds(cfg.RawCfg, cmdHandler); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		cmdHandler.Bootstrap()
	}
}
