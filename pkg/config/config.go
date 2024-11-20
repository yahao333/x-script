package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AppConfig struct {
	// 窗口配置
	WindowWidth  int `json:"window_width"`
	WindowHeight int `json:"window_height"`
	WindowX      int `json:"window_x"`
	WindowY      int `json:"window_y"`

	// Python配置
	PythonPath string `json:"python_path"`
	ScriptsDir string `json:"scripts_dir"`

	// 日志配置
	LogFile     string `json:"log_file"`
	DebugMode   bool   `json:"debug_mode"`
	MaxLogSize  int64  `json:"max_log_size"`
	MaxLogFiles int    `json:"max_log_files"`
}

var DefaultConfig = AppConfig{
	WindowWidth:  600,
	WindowHeight: 400,
	PythonPath:   "python",
	ScriptsDir:   "scripts",
	LogFile:      "logs/x-script.log",
	DebugMode:    false,
	MaxLogSize:   10,
	MaxLogFiles:  3,
}

func Load(configDir string) (*AppConfig, error) {
	configPath := filepath.Join(configDir, "config.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return createDefault(configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *AppConfig) Save(configDir string) error {
	configPath := filepath.Join(configDir, "config.json")
	data, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func createDefault(configPath string) (*AppConfig, error) {
	config := DefaultConfig

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return nil, err
	}

	return &config, nil
}
