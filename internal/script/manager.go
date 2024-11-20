package script

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yahao333/x-script/pkg/config"
	"github.com/yahao333/x-script/pkg/logger"
)

// 添加一个回调函数类型
type OutputCallback func(string)
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

func (m *Manager) Execute(script Script, callback OutputCallback) error {
	m.logger.WithFields(logger.Fields{
		"scriptName": script.Name,
		"scriptPath": script.Path,
	}).Info("Executing script")

	m.logger.WithFields(logger.Fields{
		"name": script.Name,
		"path": script.Path,
	}).Info("Executing script")

	// 创建命令
	cmd := exec.Command(m.config.PythonPath, filepath.Join(m.config.ScriptsDir, script.Path))

	// 创建管道获取输出
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe failed: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create stderr pipe failed: %w", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start script failed: %w", err)
	}

	// 创建一个通道来接收输出
	outputChan := make(chan string)

	// 处理标准输出
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			outputChan <- scanner.Text()
		}
	}()

	// 处理标准错误
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			outputChan <- "ERROR: " + scanner.Text()
		}
	}()

	// 等待命令完成
	go func() {
		if err := cmd.Wait(); err != nil {
			outputChan <- fmt.Sprintf("Script execution failed: %v", err)
		}
		close(outputChan)
	}()

	// 修改输出处理部分
	for output := range outputChan {
		// 记录到日志
		m.logger.Info(output)
		// 调用回调函数处理输出
		if callback != nil {
			callback(output)
		}
	}
	return nil
}

func (m *Manager) GetScripts() []Script {
	return m.scripts
}
