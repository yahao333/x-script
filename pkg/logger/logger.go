package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yahao333/x-script/pkg/config"
)

var (
	// Global 全局logger实例
	Global *Logger
)

// Logger 封装 logrus.Logger
type Logger struct {
	*logrus.Logger
	config     *config.AppConfig
	file       *os.File
	outputPath string
}

// Fields 类型别名，用于结构化日志
type Fields = logrus.Fields

// Option 定义logger的配置选项
type Option func(*Logger)

// New 创建新的日志实例
func New(cfg *config.AppConfig, baseDir string, opts ...Option) (*Logger, error) {
	logPath := filepath.Join(baseDir, cfg.LogFile)

	// 确保日志目录存在
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, fmt.Errorf("create log directory failed: %w", err)
	}

	// 打开日志文件
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file failed: %w", err)
	}

	// 创建logger实例
	logger := &Logger{
		Logger:     logrus.New(),
		config:     cfg,
		file:       file,
		outputPath: logPath,
	}

	// 设置默认格式化器
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:    true,
		TimestampFormat:  "2006-01-02 15:04:05.000",
		QuoteEmptyFields: true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := filepath.Base(f.File)
			return "", fmt.Sprintf("%s:%d", filename, f.Line)
		},
	})

	// 启用调用者信息
	logger.SetReportCaller(true)

	// 设置输出
	if cfg.DebugMode {
		logger.SetOutput(io.MultiWriter(file, os.Stdout))
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetOutput(file)
		logger.SetLevel(logrus.InfoLevel)
	}

	// 应用自定义选项
	for _, opt := range opts {
		opt(logger)
	}

	// 添加文件轮转钩子
	logger.AddHook(newRotationHook(logger))

	// 设置全局实例
	Global = logger

	return logger, nil
}

// WithField 创建带有单个字段的Entry
func (l *Logger) WithField(key string, value interface{}) *logrus.Entry {
	return l.Logger.WithField(key, value)
}

// WithFields 创建带有多个字段的Entry
func (l *Logger) WithFields(fields Fields) *logrus.Entry {
	return l.Logger.WithFields(fields)
}

// WithError 创建带有错误信息的Entry
func (l *Logger) WithError(err error) *logrus.Entry {
	return l.Logger.WithError(err)
}

// Trace 跟踪级别日志
func (l *Logger) Trace(args ...interface{}) {
	l.Logger.Trace(args...)
}

// Debug 调试级别日志
func (l *Logger) Debug(args ...interface{}) {
	l.Logger.Debug(args...)
}

// Info 信息级别日志
func (l *Logger) Info(args ...interface{}) {
	l.Logger.Info(args...)
}

// Warn 警告级别日志
func (l *Logger) Warn(args ...interface{}) {
	l.Logger.Warn(args...)
}

// Error 错误级别日志
func (l *Logger) Error(args ...interface{}) {
	l.Logger.Error(args...)
}

// Fatal 致命错误日志
func (l *Logger) Fatal(args ...interface{}) {
	l.Logger.Fatal(args...)
}

// Close 关闭日志文件
func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

// rotationHook 实现日志轮转
type rotationHook struct {
	logger *Logger
}

func newRotationHook(logger *Logger) *rotationHook {
	hook := &rotationHook{logger: logger}
	go hook.rotationChecker()
	return hook
}

func (h *rotationHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *rotationHook) Fire(*logrus.Entry) error {
	return nil
}

func (h *rotationHook) rotationChecker() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		if err := h.checkRotation(); err != nil {
			h.logger.WithError(err).Error("Log rotation failed")
		}
	}
}

func (h *rotationHook) checkRotation() error {
	info, err := h.logger.file.Stat()
	if err != nil {
		return fmt.Errorf("get file stat failed: %w", err)
	}

	if info.Size() > h.logger.config.MaxLogSize*1024*1024 {
		return h.rotate()
	}
	return nil
}

func (h *rotationHook) rotate() error {
	h.logger.file.Close()

	for i := h.logger.config.MaxLogFiles - 1; i >= 0; i-- {
		oldPath := fmt.Sprintf("%s.%d", h.logger.outputPath, i)
		newPath := fmt.Sprintf("%s.%d", h.logger.outputPath, i+1)

		if i == 0 {
			oldPath = h.logger.outputPath
		}

		if _, err := os.Stat(oldPath); err == nil {
			if err := os.Rename(oldPath, newPath); err != nil {
				return fmt.Errorf("rename log file failed: %w", err)
			}
		}
	}

	file, err := os.OpenFile(h.logger.outputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("create new log file failed: %w", err)
	}

	h.logger.file = file
	if h.logger.config.DebugMode {
		h.logger.SetOutput(io.MultiWriter(file, os.Stdout))
	} else {
		h.logger.SetOutput(file)
	}

	return nil
}

// LogWithContext 添加上下文信息的日志
func (l *Logger) LogWithContext(level logrus.Level, ctx map[string]interface{}, msg string) {
	entry := l.WithFields(logrus.Fields(ctx))
	switch level {
	case logrus.DebugLevel:
		entry.Debug(msg)
	case logrus.InfoLevel:
		entry.Info(msg)
	case logrus.WarnLevel:
		entry.Warn(msg)
	case logrus.ErrorLevel:
		entry.Error(msg)
	}
}

// LogError 记录错误并返回包装后的错误
func (l *Logger) LogError(err error, msg string) error {
	l.WithError(err).Error(msg)
	return fmt.Errorf("%s: %w", msg, err)
}
