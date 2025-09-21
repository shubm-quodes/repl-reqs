package config

import (
	"flag"
	"fmt"
	"os"
)

// Represents values for the available CLI flags that correspond to configuration parameters.
type Flags struct {
	enableDebugging bool
	showVersion     bool
	enableVimMode   bool
	configPath      string
}

// Processes the provided flags.
func (f *Flags) Process() {
	if f.showVersion {
		fmt.Println("Running version 0.1.1 Beta")
		os.Exit(0)
	}
	if f.enableDebugging {
		//TODO Do something here to enable debug logs
	}
	if f.configPath == "" {
		f.configPath = GetDefConfDirPath()
	}
}

// Initialize flags alongside their default values
func InitFlags() (f *Flags) {
	f = &Flags{}
	flag.BoolVar(&f.enableDebugging, "enable-debugging", false, "Enable debugging logs")
	flag.BoolVar(&f.showVersion, "v", false, "Show version information")
	flag.BoolVar(&f.enableVimMode, "vim-mode", false, "Enable vim mode")
	flag.StringVar(
		&f.configPath,
		"c",
		GetDefConfDirPath(),
		"Path for custom configuration",
	)
	flag.Parse()
	return
}
