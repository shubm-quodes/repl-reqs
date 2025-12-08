package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
	setupGracefulShutdown()

	err := safeRun()
	config.GetEnvManager().Shutdown()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func setupGracefulShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nShutting down gracefully...")
		config.GetEnvManager().Shutdown()
		os.Exit(0)
	}()
}

func safeRun() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("fatal runtime error: %v", r)
		}
	}()
	return run()
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
