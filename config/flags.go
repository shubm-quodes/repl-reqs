package config

import (
	"flag"

	"github.com/shubm-quodes/repl-reqs/log"
)

// Represents values for the available CLI flags that correspond to configuration parameters.
type FlagVal struct {
	enableDebugging bool
	ShowVersion     bool
	enableVimMode   bool
	configPath      string
}

// Processes the provided flags.
func (f *FlagVal) Process() {
	log.SetDebug(f.enableDebugging)
}

// Initialize flags alongside their default values
func InitializeFlags() (fv *FlagVal) {
	fv = &FlagVal{}
	flag.BoolVar(&fv.enableDebugging, "enable-debugging", false, "Enable debugging logs")
	flag.BoolVar(&fv.ShowVersion, "v", false, "Show version information")
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
