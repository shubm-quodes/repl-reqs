package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shubm-quodes/repl-reqs/log"
	"github.com/shubm-quodes/repl-reqs/util"
)

const (
	EnvDefaultGlobal = "Global"
	envFileName      = "env.json"
	saveDebounce     = 500 * time.Millisecond
)

type Environment string

type envManager struct {
	variables    map[Environment]map[string]string
	mu           sync.RWMutex
	activeEnv    Environment
	filePath     string
	saveChan     chan struct{}
	saveTimer    *time.Timer
	wg           sync.WaitGroup
	shutdownChan chan struct{}
	shutdownOnce sync.Once
}

type envData struct {
	Variables map[string]map[string]string `json:"variables"`
	ActiveEnv string                       `json:"active_env"`
}

var manager *envManager

func init() {
	manager = &envManager{
		variables:    make(map[Environment]map[string]string),
		activeEnv:    EnvDefaultGlobal,
		filePath:     filepath.Join(GetDefConfDirPath(), envFileName),
		saveChan:     make(chan struct{}, 1),
		shutdownChan: make(chan struct{}),
	}

	manager.load()

	// Start background saver
	manager.wg.Add(1)
	go manager.backgroundSaver()
}

func GetEnvManager() *envManager {
	return manager
}

func (m *envManager) load() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.variables[EnvDefaultGlobal] = make(map[string]string)
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn("env_manager: env file not found")
			log.Debug("env_manager: %s", err.Error())
		}
		return
	}

	var envData envData
	if err := json.Unmarshal(data, &envData); err != nil {
		log.Warn("env_manager: failed to parse env configuration")
		log.Debug("env_manager: %s", err.Error())
		return
	}

	m.variables = make(map[Environment]map[string]string)
	for envName, vars := range envData.Variables {
		m.variables[Environment(envName)] = vars
	}

	if envData.ActiveEnv != "" {
		m.activeEnv = Environment(envData.ActiveEnv)
	}
}

func (m *envManager) save() error {
	m.mu.RLock()

	varsMap := make(map[string]map[string]string)
	for env, vars := range m.variables {
		varsCopy := make(map[string]string, len(vars))
		for k, v := range vars {
			varsCopy[k] = v
		}
		varsMap[string(env)] = varsCopy
	}

	data := envData{
		Variables: varsMap,
		ActiveEnv: string(m.activeEnv),
	}
	m.mu.RUnlock()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file first, then rename for atomic operation
	tempFile := m.filePath + ".tmp"
	if err := os.WriteFile(tempFile, jsonData, 0644); err != nil {
		return err
	}

	return os.Rename(tempFile, m.filePath)
}

func (m *envManager) triggerSave() {
	select {
	case m.saveChan <- struct{}{}:
	default:
		// Channel already has a pending save signal
	}
}

// run in a goroutine and handle debounced saves
func (m *envManager) backgroundSaver() {
	defer m.wg.Done()

	for {
		select {
		case <-m.saveChan:
			if m.saveTimer != nil {
				m.saveTimer.Stop()
			}
			m.saveTimer = time.AfterFunc(saveDebounce, func() {
				if err := m.save(); err != nil {
					log.Debug("env_manager: background saver attempt failed %w", err)
				}
			})

		case <-m.shutdownChan:
			if m.saveTimer != nil {
				m.saveTimer.Stop()
			}
			// Perform final save
			m.save()
			return
		}
	}
}

// gracefully stops the background saver and ensures final save
func (m *envManager) Shutdown() {
	m.shutdownOnce.Do(func() {
		close(m.shutdownChan)
		m.wg.Wait()
	})
}

func (m *envManager) SetVar(key, value string) {
	m.mu.Lock()
	e := m.activeEnv
	if _, exists := m.variables[e]; !exists {
		m.variables[e] = make(map[string]string)
	}
	m.variables[e][key] = value
	m.mu.Unlock()

	m.triggerSave()
}

func (m *envManager) GetVar(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	vals, ok := m.variables[m.activeEnv]
	if !ok {
		return "", false
	}
	val, found := vals[key]
	return val, found
}

func (m *envManager) SetActiveEnv(env string) {
	m.mu.Lock()
	cleanEnv := sanitizeEnvName(env)
	m.activeEnv = Environment(cleanEnv)
	if _, exists := m.variables[m.activeEnv]; !exists {
		m.variables[m.activeEnv] = make(map[string]string)
	}
	m.mu.Unlock()

	m.triggerSave()
}

func sanitizeEnvName(s string) string {
	// Remove control characters (ASCII 0-31 and 127)
	result := make([]rune, 0, len(s))
	for _, r := range s {
		if r > 31 && r != 127 {
			result = append(result, r)
		}
	}
	return strings.TrimSpace(string(result))
}

func (m *envManager) GetActiveEnvName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return string(m.activeEnv)
}

func (m *envManager) ListEnvs() []Environment {
	m.mu.RLock()
	defer m.mu.RUnlock()
	envs := make([]Environment, 0, len(m.variables))
	for e := range m.variables {
		envs = append(envs, e)
	}
	return envs
}

func (m *envManager) GetActiveEnvVars() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	activeEnv := m.activeEnv
	vars := make(map[string]string)
	for k, v := range m.variables[activeEnv] {
		vars[k] = v
	}
	return vars
}

func (m *envManager) GetMatchingVars(partial string) [][]rune {
	m.mu.RLock()
	defer m.mu.RUnlock()
	criteria := &util.MatchCriteria[string]{
		M:      m.variables[m.activeEnv],
		Search: partial,
		Offset: len(partial),
	}
	return util.GetMatchingMapKeysAsRunes(criteria)
}

func (m *envManager) DeleteVar(name string) {
	m.mu.Lock()
	if vars, exists := m.variables[m.activeEnv]; exists {
		delete(vars, name)
	}
	m.mu.Unlock()

	m.triggerSave()
}

func (m *envManager) GetAllVariables() map[string]map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]map[string]string)
	for env, vars := range m.variables {
		result[string(env)] = make(map[string]string)
		for k, v := range vars {
			result[string(env)][k] = v
		}
	}
	return result
}
