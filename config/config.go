package config

import (
	"encoding/json"
	"errors"
	"strings"
)

const (
	defaultPrompt = "repl"
	defaultMascot = "😼"
)

var appCfg *AppConfig

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

type AppConfig struct {
	prompt          string
	mascot          string
	BaseUrl         string
	DirPath         string
	File            string
	DefaultEditor   string
	HistoryFile     string
	VimMode         bool
	RawCfg          RawCfg
	EnableDebugging bool
	TempFiles       []string
}

type ReqCmdCfg struct {
	Url         string            `json:"url"`
	HttpMethod  string            `json:"httpMethod"`
	Headers     map[string]string `json:"headers"`
	QueryParams any               `json:"queryParams"`
	UrlParams   any               `json:"urlParams"`
	Payload     any               `json:"payload"`
	Cmd         string            `json:"cmd"`
}

func GetAppCfg() *AppConfig {
	return appCfg
}

func GetDefaultPrompt() string {
	return defaultPrompt
}

func GetDefaultMascot() string {
	return defaultMascot
}

func (ac *AppConfig) GetPrompt() string {
	return ac.prompt
}

func (ac *AppConfig) GetPromptMascot() string {
	return ac.mascot
}

func (ac *AppConfig) UpdateDefaultPrompt(newPrompt string) error {
	if strings.Trim(newPrompt, " ") == "" {
		return errors.New("prompt cannot be empty")
	}

	ac.prompt = newPrompt
	return nil
}

func (ac *AppConfig) UpdateDefaultMascot(mascot string) error {
	if strings.Trim(mascot, " ") == "" {
		return errors.New("mascot can't be invisbile 😬")
	}

	ac.mascot = mascot
	return nil
}
