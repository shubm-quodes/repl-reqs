package config

import (
	"flag"
	"fmt"
	"os"

	"github.com/shubm-quodes/repl-reqs/log"
)

// Represents values for the available CLI flags that correspond to configuration parameters.
type FlagVal struct {
	enableDebugging bool
	showVersion     bool
	enableVimMode   bool
	configPath      string
}

// Processes the provided flags.
func (f *FlagVal) Process() {
	if f.showVersion {
		fmt.Println("Running version 0.1.1 Beta")
		os.Exit(0)
	}
	log.SetDebug(f.enableDebugging)
}

// Initialize flags alongside their default values
func InitializeFlags() (fv *FlagVal) {
	fv = &FlagVal{}
	flag.BoolVar(&fv.enableDebugging, "enable-debugging", false, "Enable debugging logs")
	flag.BoolVar(&fv.showVersion, "v", false, "Show version information")
	flag.BoolVar(&fv.enableVimMode, "vim-mode", false, "Enable vim mode")
	flag.StringVar(
		&fv.configPath,
		"c",
		GetDefConfDirPath(),
		"Path for custom configuration",
	)
	flag.Parse()
	return
}
