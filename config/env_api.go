package config

import (
	"github.com/nodding-noddy/repl-reqs/util"
)

func GetEnvManager() *envManager {
	return manager
}

// SetVar sets a variable in the given environment
func (manager *envManager) SetVar(key, value string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	e := manager.activeEnv
	// value = util.ReplaceStrPattern(value, manager.variables[e]) //TODO: No need for it here, cmd handler will itself do this.
	if _, exists := manager.variables[e]; !exists {
		manager.variables[e] = make(map[string]string)
	}
	manager.variables[e][key] = value
}

// GetVar retrieves a variable from the active environment
func (manager *envManager) GetVar(key string) (string, bool) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	vals, ok := manager.variables[manager.activeEnv]
	if !ok {
		return "", false
	}
	val, found := vals[key]
	return val, found
}

// SetActiveEnv sets the current active environment (e.g. "prod", "dev")
func (manager *envManager) SetActiveEnv(env string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.activeEnv = environment(env)
}

// GetActiveEnvName returns the current active environment
func (manager *envManager) GetActiveEnvName() string {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return string(manager.activeEnv)
}

// ListEnvs returns a list of all existing environments
func (manager *envManager) ListEnvs() []environment {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	envs := make([]environment, 0, len(manager.variables))
	for e := range manager.variables {
		envs = append(envs, e)
	}
	return envs
}

func (manager *envManager) GetActiveEnvVars() map[string]string {
	activeEnv := manager.activeEnv
	activeEnvVars := manager.variables[activeEnv]
	return activeEnvVars
}

func (manager *envManager) GetMatchingVars(partial string) [][]rune {
	criteria := &util.MatchCriteria[string]{
		M:   manager.variables[manager.activeEnv],
		Search: partial,
		Offset: len(partial),
	}
	return util.GetMatchingMapKeysAsRunes(criteria)
}

func (manager *envManager) DeleteVar(name string) {
	vars := manager.GetActiveEnvVars()
	delete(vars, name)
}
