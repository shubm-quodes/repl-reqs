package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	VarPattern    = `{{(.*?)}}`
	defaultPrompt = "repl"
	defaultMascot = "😼"
)

var appCfg *AppCfg

type RawCfg struct {
	Prompt  string `json:"prompt"`
	Mascot  string `json:"promptMascot"`
	BaseUrl string `json:"baseUrl"`
	Commons struct {
		Headers map[string]string
		vars    map[string]string
	} `json:"commons"`
	RawRequests []json.RawMessage `json:"requests"`
}

// TODO: check and un-export fields
type AppCfg struct {
	prompt          string
	mascot          string
	dirPath         string
	file            string
	defaultEditor   string
	HistoryFile     string
	tempFiles       []string
	vimMode         bool
	enableDebugging bool
	truncatePrompt  bool
	maxPromptChars  int32
	RawCfg          RawCfg
}

type ReqCmdCfg struct {
	Url         string            `json:"url"`
	HttpMethod  string            `json:"httpMethod"`
	Headers     map[string]string `json:"headers"`
	QueryParams any               `json:"queryParams"`
	UrlParams   any               `json:"urlParams"`
	Body        any               `json:"body"`
	Cmd         string            `json:"cmd"`
}

func NewAppCfg() *AppCfg {
	return &AppCfg{
		truncatePrompt: true,
		maxPromptChars: 20,
		defaultEditor:  getReplEditor(),
	}
}

func GetAppCfg() *AppCfg {
	return appCfg
}

func GetDefaultPrompt() string {
	return defaultPrompt
}

func GetDefaultMascot() string {
	return defaultMascot
}

// Creates a temporary file with the specified extension
func (ac *AppCfg) NewTempFile(extension string) (*os.File, error) {
	fileName := fmt.Sprintf("repl-reqs-temp-%d.%s", len(ac.tempFiles)+1, extension)
	return os.CreateTemp("", fileName)
}

func (ac *AppCfg) CfgFilePath() string {
	return ac.file
}

func (ac *AppCfg) DirPath() string {
	return ac.dirPath
}

func (ac *AppCfg) GetDefaultEditor() string {
	return ac.defaultEditor
}

func (ac *AppCfg) GetPrompt() string {
	return ac.prompt
}

func (ac *AppCfg) GetPromptMascot() string {
	return ac.mascot
}

func (ac *AppCfg) TruncatePrompt() bool {
	return ac.truncatePrompt
}

func (ac *AppCfg) MaxPromptChars() int32 {
	return ac.maxPromptChars
}

func (ac *AppCfg) UpdateDefaultPrompt(newPrompt string) error {
	if strings.Trim(newPrompt, " ") == "" {
		return errors.New("prompt cannot be empty")
	}

	ac.prompt = newPrompt
	return nil
}

func (ac *AppCfg) UpdateDefaultMascot(mascot string) error {
	if strings.Trim(mascot, " ") == "" {
		return errors.New("mascot can't be invisbile 😬")
	}

	ac.mascot = mascot
	return nil
}
