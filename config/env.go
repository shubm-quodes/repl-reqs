package config

import "sync"

const EnvDefaultGlobal = "Global"

type environment string

type envManager struct {
	variables map[environment]map[string]string
	mu        sync.RWMutex
	activeEnv environment
}

var manager *envManager

func init() {
	manager = &envManager{
		variables: make(map[environment]map[string]string),
		activeEnv: EnvDefaultGlobal, // default environment
	}
}
