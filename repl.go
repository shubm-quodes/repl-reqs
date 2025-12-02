package main

import (
	"fmt"
	"os"

	"github.com/shubm-quodes/repl-reqs/cmd"
	"github.com/shubm-quodes/repl-reqs/cmd/syscmd"
	"github.com/shubm-quodes/repl-reqs/config"
)

// -ldflags
var (
	// Default value for local runs
	version            string = "0.0.0"
	buildDate          string = "N/A"
	omitSystemCommands string = "false"
)

func main() {
	flags := config.InitializeFlags()
	flags.Process()

	if flags.ShowVersion {
		fmt.Printf("repl-reqs: v%s %s\n", version, buildDate)
		os.Exit(0)
	}

	cfg := config.Initialize(flags, version)
	reg := cmd.NewCmdRegistry()

	if omitSystemCommands == "false" {
		syscmd.RegisterCmds(reg)
	}

	if cmdHandler, err := cmd.NewCmdHandler(cfg, config.NewShellCfg(cfg), reg); err != nil {
		fmt.Println("failed to initialize command handler", err)
		os.Exit(1)
	} else {
		if err := syscmd.InitNetCmds(cfg.RawCfg, cmdHandler); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		cmdHandler.Bootstrap(omitSystemCommands == "true")
	}
}
