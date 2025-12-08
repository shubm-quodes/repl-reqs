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
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Fatal runtime error: %v\n", r)
			os.Exit(1)
		}
	}()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	flags := config.InitializeFlags()
	flags.Process()

	if flags.ShowVersion {
		fmt.Printf("repl-reqs: v%s %s\n", version, buildDate)
		return nil
	}

	cfg := config.Initialize(flags, version)
	reg := cmd.NewCmdRegistry()

	if omitSystemCommands == "false" {
		syscmd.RegisterCmds(reg)
	}

	cmdHandler, err := cmd.NewCmdHandler(cfg, config.NewShellCfg(cfg), reg)
	if err != nil {
		return fmt.Errorf("failed to initialize command handler: %w", err)
	}

	if err := syscmd.InitNetCmds(cfg.RawCfg, cmdHandler); err != nil {
		return err
	}

	cmdHandler.Bootstrap(omitSystemCommands == "true")
	return nil
}
