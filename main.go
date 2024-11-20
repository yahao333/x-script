package main

import (
	"log"

	"github.com/sirupsen/logrus"
	"github.com/yahao333/x-script/internal/app"
	"github.com/yahao333/x-script/internal/utils"
	"github.com/yahao333/x-script/pkg/config"
	"github.com/yahao333/x-script/pkg/logger"
)

func main() {
	// 获取应用数据目录
	appDataDir := utils.GetAppDataDir()

	// 加载配置
	cfg, err := config.Load(appDataDir)
	if err != nil {
		log.Fatal(err)
	}

	// 初始化日志
	logger, err := logger.New(cfg, appDataDir)
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Close()

	logger.Info("Application starting...")
	logger.WithFields(logrus.Fields{
		"appDataDir": appDataDir,
		"debugMode":  cfg.DebugMode,
	}).Debug("Configuration loaded")

	// 创建并运行应用
	app := app.New(cfg, logger)
	if err := app.Run(); err != nil {
		logger.WithError(err).Error("Application failed to start")
		log.Fatal(err)
	}
}
