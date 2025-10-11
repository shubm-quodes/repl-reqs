package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nodding-noddy/repl-reqs/util"
)

func (c *AppCfg) Load() {
	if !c.isCfgTemplateExists() {
		c.initializeCfgTemplate()
	}
	c.loadCfg()
	activeVars := manager.GetActiveEnvVars()
	util.CopyMap(activeVars, c.RawCfg.Commons.vars)
}

func (c *AppCfg) setBaseURL(conf map[string]any) {
	if baseUrl, found := conf["baseUrl"].(string); found {
		if !strings.HasSuffix(baseUrl, "/") {
			baseUrl = baseUrl + "/"
		}
		c.BaseUrl = baseUrl
		return
	}
	fmt.Fprintln(os.Stderr,
		`failed to initialize config: No "baseUrl" was specified`,
	)
	os.Exit(1)
}

func (c *AppCfg) loadCfg() {
	fileContents, err := os.ReadFile(c.File)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config file: %s", err.Error())
		os.Exit(1)
	}

	if err := json.Unmarshal(fileContents, &c.RawCfg); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing config file: %s", err.Error())
		os.Exit(1)
	}
	c.prompt, c.mascot = defaultPrompt, defaultMascot
	if strings.Trim(c.RawCfg.Prompt, " ") != "" {
		c.prompt = c.RawCfg.Prompt
	}
	if strings.Trim(c.RawCfg.Mascot, " ") != "" {
		c.mascot = c.RawCfg.Mascot
	}
}
