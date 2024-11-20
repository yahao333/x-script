package app

import (
	"fmt"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"github.com/yahao333/x-script/internal/script"
	"github.com/yahao333/x-script/internal/utils"
	"github.com/yahao333/x-script/pkg/config"
	"github.com/yahao333/x-script/pkg/logger"
)

var showHelpAction *walk.Action

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

	// Load scripts first
	if err := app.scripts.Load(); err != nil {
		app.logger.WithError(err).Error("Failed to load scripts")
		return err
	}
	app.logger.Debug("Scripts loaded successfully")

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
	showAction.SetText("&Show")
	showAction.Triggered().Attach(func() {
		app.logger.Debug("Showing main window")
		app.notifyIcon.SetVisible(true)
	})
	menu.Actions().Add(showAction)

	exitAction := walk.NewAction()
	exitAction.SetText("&Exit")
	exitAction.Triggered().Attach(func() {
		app.logger.Debug("Exiting application")
		app.window.Close()
	})
	menu.Actions().Add(exitAction)

	for i := 0; i < menu.Actions().Len(); i++ {
		app.logger.Debugf("Inserting action %d", i)
		app.notifyIcon.ContextMenu().Actions().Insert(i, menu.Actions().At(i))
	}

	return nil
}

func (app *XScript) registerHotkey() error {
	// Define a hotkey ID
	const hotkeyID = 1
	// Define MOD_CONTROL constant
	const MOD_CONTROL = 0x0002
	// Define VK_C constant
	const VK_C = 0x43
	const VK_A = 0x41
	// Define VK_CONTROL constant
	const VK_CONTROL = 0x11

	// Register the hotkey for Ctrl+C
	if !registerHotKey(syscall.Handle(app.window.Handle()), hotkeyID, MOD_CONTROL, VK_CONTROL) {
		err := syscall.GetLastError()
		app.logger.WithError(err).Error("Failed to register global hotkey")
		return err
	}

	// Listen for hotkey messages
	go func() {
		var msg win.MSG
		var lastPressTime int64
		const doublePressInterval = 500 // milliseconds

		for {
			win.GetMessage(&msg, 0, 0, 0)
			if msg.Message == win.WM_HOTKEY && msg.WParam == uintptr(hotkeyID) {
				app.logger.Debug("Global hotkey triggered")
				currentTime := time.Now().UnixMilli()
				if currentTime-lastPressTime <= doublePressInterval {
					app.logger.Debug("Double Ctrl key pressed")
					app.window.Show()
				}
				lastPressTime = currentTime
			}
			win.TranslateMessage(&msg)
			win.DispatchMessage(&msg)
		}
	}()

	app.logger.Info("Global hotkey registered")
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
	results := app.scripts.Search(keyword)
	app.logger.WithField("keyword", keyword).Debug("Searching scripts")

	// Display search results in the log view
	app.logView.SetText("") // Clear previous results
	for _, script := range results {
		app.logView.AppendText(fmt.Sprintf("Found script: %s\n", script.Name))
	}
}

func (app *XScript) showAbout() {
	app.logger.Debug("Showing about dialog")
}

func (app *XScript) showScriptList() {
	app.logger.Debug("Showing script list")

	// Create a popup menu using Windows API
	hMenu := win.CreatePopupMenu()
	if hMenu == 0 {
		app.logger.Error("Failed to create popup menu")
		return
	}
	defer win.DestroyMenu(hMenu)

	// Add menu items
	scripts := app.scripts.GetScripts()
	for i, script := range scripts {
		// Convert string to UTF16 for Windows API
		text, _ := syscall.UTF16PtrFromString(script.Name)
		app.logger.Debugf("Appending menu item: %s", script.Name)
		if !appendMenu(hMenu, win.MF_STRING, uint32(i+1), text) {
			app.logger.WithField("script", script.Name).Error("Failed to append menu item")
			continue
		}
	}

	// Get the position of the search box
	bounds := app.searchBox.Bounds()

	// Convert client coordinates to screen coordinates
	var point win.POINT
	point.X = int32(bounds.X + 10)
	point.Y = int32(bounds.Y + bounds.Height + 10)
	win.ClientToScreen(win.HWND(app.window.Handle()), &point)

	// Show the popup menu
	cmd := win.TrackPopupMenu(
		hMenu,
		win.TPM_LEFTALIGN|win.TPM_TOPALIGN|win.TPM_RETURNCMD,
		point.X,
		point.Y,
		0,
		win.HWND(app.window.Handle()),
		nil,
	)

	// Handle menu selection
	if cmd > 0 && int(cmd) <= len(scripts) {
		app.searchBox.SetText(scripts[cmd-1].Name)
	}
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

func getMousePosition() walk.Point {
	var pt win.POINT
	win.GetCursorPos(&pt)
	return walk.Point{X: int(pt.X), Y: int(pt.Y)}
}

func registerHotKey(hwnd syscall.Handle, id int, mod, vk uint) bool {
	user32 := syscall.NewLazyDLL("user32.dll")
	registerHotKey := user32.NewProc("RegisterHotKey")
	ret, _, _ := registerHotKey.Call(
		uintptr(hwnd),
		uintptr(id),
		uintptr(mod),
		uintptr(vk),
	)
	return ret != 0
}

func appendMenu(hMenu win.HMENU, flags uint32, id uint32, text *uint16) bool {
	user32 := syscall.NewLazyDLL("user32.dll")
	appendMenu := user32.NewProc("AppendMenuW")
	ret, _, _ := appendMenu.Call(
		uintptr(hMenu),
		uintptr(flags),
		uintptr(id),
		uintptr(unsafe.Pointer(text)),
	)
	return ret != 0
}
