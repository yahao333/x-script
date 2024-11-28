package script

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yahao333/x-script/pkg/config"
	"github.com/yahao333/x-script/pkg/logger"
)

// 添加一个回调函数类型
type OutputCallback func(string)
type Script struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Description string    `json:"description"`
	Keywords    string    `json:"keywords"`
	LastRunTime time.Time `json:"last_run_time"`
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

	var results []Script
	if keyword == "" {
		// 复制所有脚本
		results = make([]Script, len(m.scripts))
		copy(results, m.scripts)
	} else {
		// 搜索匹配的脚本
		keyword = strings.ToLower(keyword)
		for _, script := range m.scripts {
			if strings.Contains(strings.ToLower(script.Name), keyword) ||
				strings.Contains(strings.ToLower(script.Keywords), keyword) {
				results = append(results, script)
			}
		}
	}

	// 按最后运行时间排序
	sort.Slice(results, func(i, j int) bool {
		// 如果两个脚本都没有运行过（零值），按名称排序
		if results[i].LastRunTime.IsZero() && results[j].LastRunTime.IsZero() {
			return results[i].Name < results[j].Name
		}
		// 未运行过的脚本放在后面
		if results[i].LastRunTime.IsZero() {
			return false
		}
		if results[j].LastRunTime.IsZero() {
			return true
		}
		// 按最后运行时间降序排序（最近的在前面）
		return results[i].LastRunTime.After(results[j].LastRunTime)
	})

	return results
}

func (m *Manager) Execute(script Script, callback OutputCallback) error {
	m.logger.WithFields(logger.Fields{
		"scriptName": script.Name,
		"scriptPath": script.Path,
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
		err := cmd.Wait()
		if err != nil {
			outputChan <- fmt.Sprintf("Script execution failed: %v", err)
		} else {
			outputChan <- fmt.Sprintf("Script '%s' completed successfully", script.Name)
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

	// 更新最后运行时间
	for i := range m.scripts {
		if m.scripts[i].Name == script.Name {
			m.scripts[i].LastRunTime = time.Now()
			// 保存到文件
			if err := m.saveScripts(); err != nil {
				m.logger.WithError(err).Error("Failed to save scripts")
			}
			break
		}
	}

	return nil
}

// 添加保存脚本配置的函数
func (m *Manager) saveScripts() error {
	config := struct {
		Scripts []Script `json:"scripts"`
	}{
		Scripts: m.scripts,
	}

	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal scripts config failed: %w", err)
	}

	configPath := filepath.Join(m.config.ScriptsDir, "scripts.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write scripts config failed: %w", err)
	}

	return nil
}

func (m *Manager) GetScripts() []Script {
	return m.scripts
}
