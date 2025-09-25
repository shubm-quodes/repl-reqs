package main

import (
	"fmt"
	"os"

	"github.com/nodding-noddy/repl-reqs/cmd"
	_ "github.com/nodding-noddy/repl-reqs/cmd/syscmd"
	"github.com/nodding-noddy/repl-reqs/config"
)

func main() {
	flags := config.InitFlags()
	flags.Process()
	cfg := config.Initialize(flags)

	if cmdHandler, err := cmd.NewCmdHandler(cfg, config.GetShellCfg(cfg)); err != nil {
		fmt.Println("failed to initialize command handler", err)
		os.Exit(1)
	} else {
		cmd.InjectCmdHandler(cmdHandler)
		cmdHandler.Repl(cfg.GetPrompt(), cfg.GetPromptMascot())
	}
}
