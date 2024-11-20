package app

import (
	"path/filepath"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/yahao333/x-script/internal/script"
	"github.com/yahao333/x-script/internal/utils"
	"github.com/yahao333/x-script/pkg/config"
	"github.com/yahao333/x-script/pkg/logger"
)

type XScript struct {
	window     *walk.MainWindow
	notifyIcon *walk.NotifyIcon
	searchBox  *walk.LineEdit
	logView    *walk.TextEdit
	config     *config.AppConfig
	logger     *logger.Logger
	scripts    *script.Manager
}

func New(cfg *config.AppConfig, log *logger.Logger) *XScript {
	return &XScript{
		config:  cfg,
		logger:  log,
		scripts: script.NewManager(cfg, log),
	}
}

func (app *XScript) Run() error {
	app.logger.Info("Starting application initialization")

	// 创建主窗口
	if err := app.createMainWindow(); err != nil {
		app.logger.WithError(err).Error("Failed to create main window")
		return err
	}
	app.logger.Debug("Main window created")

	// 创建托盘图标
	if err := app.createNotifyIcon(); err != nil {
		app.logger.WithError(err).Error("Failed to create notify icon")
		return err
	}
	app.logger.Debug("Notify icon created")

	// 注册热键
	if err := app.registerHotkey(); err != nil {
		app.logger.WithError(err).Error("Failed to register hotkey")
		return err
	}
	app.logger.Debug("Hotkey registered")

	app.logger.Info("Application initialization completed")
	app.window.Run()
	return nil
}

func (app *XScript) someMethod() {
	// 基本日志
	logger.Global.Debug("Debug message")
	logger.Global.Info("Info message")

	// 带字段的结构化日志
	logger.Global.WithFields(logger.Fields{
		"user":   "admin",
		"action": "login",
	}).Info("User logged in")

	// // 带错误的日志
	// err := someOperation()
	// if err != nil {
	// 	logger.Global.WithError(err).Error("Operation failed")
	// }

	// 带上下文的日志
	logger.Global.WithFields(logger.Fields{
		"component": "script",
		"name":      "test.py",
		"duration":  "100ms",
	}).Info("Script execution completed")
}

func (app *XScript) createMainWindow() error {
	// 创建主窗口
	mainWindow := MainWindow{
		AssignTo: &app.window,
		Title:    "X-Script",
		MinSize:  Size{Width: 400, Height: 300},
		Size:     Size{Width: app.config.WindowWidth, Height: app.config.WindowHeight},
		Layout:   VBox{},
		Children: []Widget{
			Composite{
				Layout: HBox{},
				Children: []Widget{
					LineEdit{
						AssignTo: &app.searchBox,
						Text:     "",
						OnTextChanged: func() {
							app.handleSearch()
						},
					},
					PushButton{
						Text: "▼",
						OnClicked: func() {
							app.showScriptList()
						},
					},
					PushButton{
						Text: "运行",
						OnClicked: func() {
							app.runScript()
						},
					},
				},
			},
			TextEdit{
				AssignTo: &app.logView,
				ReadOnly: true,
				VScroll:  true,
			},
		},
	}

	if err := mainWindow.Create(); err != nil {
		return app.logger.LogError(err, "Failed to create main window")
	}

	// 设置窗口位置
	if app.config.WindowX > 0 && app.config.WindowY > 0 {
		app.window.SetX(app.config.WindowX)
		app.window.SetY(app.config.WindowY)
	}

	// 监听窗口关闭事件
	app.window.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		// 保存窗口位置和大小
		bounds := app.window.Bounds()
		app.config.WindowX = bounds.X
		app.config.WindowY = bounds.Y
		app.config.WindowWidth = bounds.Width
		app.config.WindowHeight = bounds.Height

		if err := app.config.Save(app.getConfigDir()); err != nil {
			app.logger.WithError(err).Error("Failed to save window configuration")
		}
	})

	// // 隐藏窗口，等待热键触发
	// app.window.Hide()
	return nil
}

func (app *XScript) createNotifyIcon() error {
	var err error

	app.notifyIcon, err = walk.NewNotifyIcon(app.window)
	if err != nil {
		return app.logger.LogError(err, "Failed to create notify icon")
	}

	// 设置图标
	icon, err := walk.Resources.Icon("logo.ico")
	if err != nil {
		app.logger.WithError(err).Warn("Failed to load icon, using default")
		icon, _ = walk.NewIconFromResourceId(2) // 使用默认图标
	}
	app.notifyIcon.SetIcon(icon)
	app.notifyIcon.SetVisible(true)

	// 设置工具提示
	if err := app.notifyIcon.SetToolTip("X-Script"); err != nil {
		app.logger.WithError(err).Warn("Failed to set tooltip")
	}

	// 设置右键菜单
	menu, err := walk.NewMenu()
	if err != nil {
		return app.logger.LogError(err, "Failed to create context menu")
	}
	showAction := walk.NewAction()
	showAction.SetText("Show")
	showAction.Triggered().Attach(func() {
		app.logger.Debug("Showing main window")
		app.window.Show()
	})
	menu.Actions().Add(showAction)

	exitAction := walk.NewAction()
	exitAction.SetText("Exit")
	exitAction.Triggered().Attach(func() {
		app.logger.Debug("Exiting application")
		app.window.Close()
	})
	menu.Actions().Add(exitAction)

	for i := 0; i < menu.Actions().Len(); i++ {
		app.notifyIcon.ContextMenu().Actions().Insert(i, menu.Actions().At(i))
	}

	return nil
}

func (app *XScript) registerHotkey() error {
	// TODO: 实现热键注册功能
	app.logger.Info("Hotkey registration is not implemented yet")
	return nil
}

func (app *XScript) getConfigDir() string {
	// 使用 utils 包中的方法获取配置目录
	return filepath.Join(utils.GetAppDataDir())
}

// 添加日志到界面
func (app *XScript) appendLog(message string) {
	if app.logView != nil {
		app.logView.AppendText(message + "\n")
	}
}

// 新增的辅助方法
func (app *XScript) handleSearch() {
	keyword := app.searchBox.Text()
	_ = app.scripts.Search(keyword)
	app.logger.WithField("keyword", keyword).Debug("Searching scripts")
	// TODO: 显示搜索结果
}

func (app *XScript) showScriptList() {
	// TODO: 显示脚本列表下拉框
	app.logger.Debug("Showing script list")
}

func (app *XScript) runScript() {
	keyword := app.searchBox.Text()
	scripts := app.scripts.Search(keyword)
	if len(scripts) > 0 {
		if err := app.scripts.Execute(scripts[0]); err != nil {
			app.logger.WithError(err).Error("Failed to execute script")
		}
	}
}
