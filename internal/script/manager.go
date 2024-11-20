package script

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yahao333/x-script/pkg/config"
	"github.com/yahao333/x-script/pkg/logger"
)

type Script struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Keywords    string `json:"keywords"`
}

type Manager struct {
	config  *config.AppConfig
	logger  *logger.Logger
	scripts []Script
}

func NewManager(cfg *config.AppConfig, log *logger.Logger) *Manager {
	return &Manager{
		config:  cfg,
		logger:  log,
		scripts: make([]Script, 0),
	}
}

func (m *Manager) Load() error {
	m.logger.WithFields(logger.Fields{
		"scriptsDir": m.config.ScriptsDir,
	}).Debug("Loading scripts")

	// 读取 scripts.json
	configPath := filepath.Join(m.config.ScriptsDir, "scripts.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read scripts config failed: %w", err)
	}

	var config struct {
		Scripts []Script `json:"scripts"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse scripts config failed: %w", err)
	}

	m.scripts = config.Scripts
	m.logger.WithField("count", len(m.scripts)).Info("Scripts loaded")
	return nil
}

func (m *Manager) Search(keyword string) []Script {
	m.logger.WithFields(logger.Fields{
		"keyword": keyword,
	}).Debug("Searching scripts")

	if keyword == "" {
		return m.scripts
	}

	var results []Script
	keyword = strings.ToLower(keyword)
	for _, script := range m.scripts {
		if strings.Contains(strings.ToLower(script.Name), keyword) ||
			strings.Contains(strings.ToLower(script.Keywords), keyword) {
			results = append(results, script)
		}
	}
	return results
}

func (m *Manager) Execute(script Script) error {
	m.logger.WithFields(logger.Fields{
		"scriptName": script.Name,
		"scriptPath": script.Path,
	}).Info("Executing script")

	m.logger.WithFields(logger.Fields{
		"name": script.Name,
		"path": script.Path,
	}).Info("Executing script")

	cmd := exec.Command(m.config.PythonPath, filepath.Join(m.config.ScriptsDir, script.Path))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("execute script failed: %w", err)
	}

	m.logger.Info(string(output))
	return nil
}

func (m *Manager) GetScripts() []Script {
	return m.scripts
}
