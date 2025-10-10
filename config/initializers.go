package config

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"slices"

	"github.com/chzyer/readline"
	"github.com/nodding-noddy/repl-reqs/log"
	"github.com/nodding-noddy/repl-reqs/util"
)

// Loads and initializes configuration parameters based on user supplied flags.
func Initialize(flags *FlagVal) *AppCfg {
	appCfg = NewAppCfg()
	appCfg.DirPath = flags.configPath
	appCfg.File = path.Join(flags.configPath, "config.json")
	appCfg.HistoryFile = path.Join(flags.configPath, "history")
	appCfg.VimMode = flags.enableVimMode
	appCfg.Load()
	return appCfg
}

func getReplEditor() string {
	envEditor := os.Getenv("REPL_EDITOR")
	if IsNotValidEditor(envEditor) {
		defaultEditor := getDefaultEditor()
		log.Debug(
			`Invalid/or no env var "%s", defaulting to "%s"`,
			"REPL_EDITOR",
			defaultEditor,
		)
		return defaultEditor
	}
	log.Debug(`Default editor configured as "%s"`, envEditor)
	return envEditor
}

func GetShellCfg(cfg *AppCfg) *readline.Config {
	return &readline.Config{
		Prompt:            cfg.prompt,
		HistoryFile:       cfg.HistoryFile,
		AutoComplete:      nil,
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		VimMode:           cfg.VimMode,
		HistorySearchFold: true,
		FuncFilterInputRune: func(r rune) (rune, bool) {
			switch r {
			case readline.CharCtrlZ:
				return r, false
			case readline.CharCtrlH:
				return r, false
			}
			return r, true
		},
	}
}

func IsNotValidEditor(name string) bool {
	return name == "" ||
		!slices.Contains([]string{"vi", "vim", "nano", "notepad", "sam"}, name)
}

func getDefaultEditor() string {
	switch runtime.GOOS {
	case "windows":
		return "notepad"
	case "darwin":
		return "nano"
	case "linux":
		return "nano"
	case "freebsd", "openbsd", "netbsd", "dragonfly":
		return "nano"
	case "plan9":
		return "sam"
	case "aix", "solaris", "illumos":
		return "vi"
	default:
		return "vi"
	}
}

// Creates a configuration template/boilerplate along with the required files
func (c *AppCfg) initializeCfgTemplate() {
	if util.FileDoesNotExist(c.DirPath) {
		os.Mkdir(c.DirPath, 0700)
	}
	// config.json is that file, which is going to hold all the accumulated configuration.
	for file, contents := range map[string]string{
		"config.json": `{"requests": []}`,
		"history":     "",
	} {
		absPath := path.Join(c.DirPath, file)
		if util.FileDoesNotExist(file) {
			createFile(absPath, contents)
		}
	}
}

func createFile(filePath string, contents string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("Failed to create config file: %s", err)
	}
	defer file.Close()
	if contents != "" {
		return writeContents(file, contents)
	}
	return nil
}

func writeContents(file *os.File, contents string) error {
	if _, err := file.Write([]byte(contents)); err != nil {
		return fmt.Errorf(
			`Failed to write contents to the file "%s": %s`, file.Name(),
			err.Error(),
		)
	}
	return nil
}

func (c *AppCfg) isCfgTemplateExists() bool {
	return !slices.ContainsFunc([]string{
		c.DirPath,
		c.File,
		c.HistoryFile,
	}, util.FileDoesNotExist)
}

func (c *AppCfg) getConfFilePath() string {
	return path.Join(getConfDirPath(c.DirPath), "conf.json")
}

func getConfDirPath(confPath string) string {
	if confPath == "" {
		confPath = GetDefConfDirPath()
	}
	return confPath
}

// Returns the default config path
func GetDefConfDirPath() (confPath string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to find the platform's home dir")
		os.Exit(1)
	}
	if util.OsIsUnixLike() {
		confPath = path.Join(homeDir, ".config", "repl-reqs")
	} else {
		confPath = path.Join(homeDir, "repl-reqs")
	}
	return
}

func GetStructuredReqConf() ([]byte, error) {
	homeDir := GetDefConfDirPath()
	sConfPath := path.Join(homeDir, "struct-reqs.json")
	return os.ReadFile(sConfPath)
}

func PersistStructuredReqConf(newConf []byte) error {
	filePath := path.Join(GetDefConfDirPath(), "struct-reqs.json")
	return os.WriteFile(filePath, newConf, 0700)
}
