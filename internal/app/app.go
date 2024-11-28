package app

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"

	"github.com/yahao333/x-script/internal/script"
	"github.com/yahao333/x-script/internal/utils"
	"github.com/yahao333/x-script/pkg/config"
	"github.com/yahao333/x-script/pkg/logger"
)

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
	procGetMessage       = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessage  = user32.NewProc("DispatchMessageW")
)

const (
	MOD_ALT      = 0x0001
	MOD_CONTROL  = 0x0002
	MOD_SHIFT    = 0x0004
	MOD_WIN      = 0x0008
	WM_HOTKEY    = 0x0312
	MOD_NOREPEAT = 0x4000
)

type MSG struct {
	HWND   uintptr
	UINT   uint32
	WPARAM uintptr
	LPARAM uintptr
	Time   uint32
	Pt     struct{ X, Y int32 }
}

var showHelpAction *walk.Action

// 辅助函数：从窗口句柄获取窗口对象
type XScript struct {
	window     *walk.MainWindow
	notifyIcon *walk.NotifyIcon
	searchBox  *walk.LineEdit
	logView    *walk.TextEdit
	config     *config.AppConfig
	logger     *logger.Logger
	scripts    *script.Manager
	resultList *walk.ListBox
	hotkey     *walk.GlobalHotKey
}

// 创建 XScript 实例
func New(cfg *config.AppConfig, log *logger.Logger) *XScript {
	return &XScript{
		config:     cfg,
		logger:     log,
		scripts:    script.NewManager(cfg, log),
		resultList: nil,
		hotkey:     nil,
	}
}

// 清理
func (app *XScript) cleanup() {
	// 取消注册热键
	if app.hotkey != nil {
		app.hotkey.Unregister()
	}
	app.logger.Debug("Hotkey unregistered")
}

// 运行
func (app *XScript) Run() error {
	app.logger.Info("Starting application initialization")

	// 加载脚本
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

	// 设置清理函数
	app.window.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		app.cleanup()
	})

	app.logger.Info("Application initialization completed")
	app.window.Run()

	return nil
}

// 切换窗口显示状态
func (app *XScript) toggleWindow() {
	if app.window.Visible() {
		app.window.Hide()
	} else {
		app.showWindow()
	}
}

// 创建主窗口
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
						Visible:  false,
						OnTextChanged: func() {
							app.handleSearch()
						},
						OnKeyDown: func(key walk.Key) {
							app.handleSearchKeyDown(key)
						},
					},
					PushButton{
						Text:    "▼",
						Visible: false,
						OnClicked: func() {
							app.showScriptList()
						},
					},
					PushButton{
						Text:    "运行",
						Visible: false,
						OnClicked: func() {
							app.runScript()
						},
					},
				},
			},
			ListBox{
				AssignTo: &app.resultList,
				Model:    []string{},
				OnItemActivated: func() {
					app.runSelectedScript()
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

	// 初始化列表框数据
	app.handleSearch() // 执行空关键字搜索,显示所有脚本

	app.focus2ListBox()

	return nil
}

func (app *XScript) clearSearchBox() {
	if app.searchBox != nil {
		app.searchBox.SetText("")
	}
}

func (app *XScript) focus2ListBox() {
	// 设置窗口焦点
	app.window.SetFocus()

	// 如果列表框存在且有项目,设置焦点并选中第一项
	if app.resultList != nil {
		app.resultList.SetFocus()
	}

	// 清空搜索框文本
	app.clearSearchBox()
}

func (app *XScript) showWindow() {
	// 先恢复窗口状态
	if err := app.window.Restore(); err != nil {
		app.logger.WithError(err).Error("Failed to restore window")
	}

	// 显示窗口
	app.window.Show()

	// 确保窗口在最前面
	win.SetForegroundWindow(win.HWND(app.window.Handle()))

	app.focus2ListBox()

	// 设置窗口位置
	if app.config.WindowX > 0 && app.config.WindowY > 0 {
		app.window.SetX(app.config.WindowX)
		app.window.SetY(app.config.WindowY)
	}
}

// 注册热键
func registerHotKey(id int, modifiers uint, vk uint) error {
	ret, _, err := procRegisterHotKey.Call(0, uintptr(id), uintptr(modifiers), uintptr(vk))
	if ret == 0 {
		return err
	}
	return nil
}

// 注册热键
func (app *XScript) registerHotkey() error {
	shortcut := walk.Shortcut{
		Modifiers: walk.ModControl | walk.ModAlt,
		Key:       walk.KeyA,
	}
	hotkey, err := walk.RegisterGlobalHotKey(app.window.AsWindowBase(), shortcut, func() {
		// Handle hotkey press
		// walk.MsgBox(app.window, "Hotkey", "Global hotkey pressed!", walk.MsgBoxIconInformation)
		app.toggleWindow()
	})
	if err != nil {
		app.logger.Fatal(err)
		return err
	}
	app.hotkey = hotkey
	return nil
}

// 创建托盘图标
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

// 获取配置目录
func (app *XScript) getConfigDir() string {
	// 使用 utils 包中的方法获取配置目录
	return filepath.Join(utils.GetAppDataDir())
}

// 添加日志到界面， 是否换行
func (app *XScript) appendLog(message string, isNewLine bool) {
	if app.logView != nil {
		if isNewLine {
			app.logView.AppendText(message + "\r\n")
		} else {
			app.logView.AppendText(message)
		}
	}
}

// 搜索脚本
func (app *XScript) handleSearch() {
	keyword := app.searchBox.Text()
	results := app.scripts.Search(keyword)
	app.logger.WithField("keyword", keyword).Debug("Searching scripts")

	// Display search results in the log view
	app.logView.SetText("") // Clear previous results
	for _, script := range results {
		// app.logView.AppendText(fmt.Sprintf("Found script: %s\r\n", script.Name))
		app.appendLog(fmt.Sprintf("Found script: %s\r\n", script.Name), true)
	}

	// Update list model
	items := make([]string, len(results))
	for i, script := range results {
		items[i] = script.Name
	}
	app.resultList.SetModel(items)

	// Select first result
	if len(items) > 0 {
		app.resultList.SetCurrentIndex(0)
	}
}

// 显示关于对话框
func (app *XScript) showAbout() {
	app.logger.Debug("Showing about dialog")
}

// 显示脚本列表
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

// 运行脚本
func (app *XScript) runScript() {
	keyword := app.searchBox.Text()
	scripts := app.scripts.Search(keyword)

	if len(scripts) == 0 {
		app.appendLog("No matching script found", true)
		return
	}

	script := scripts[0]
	app.appendLog(fmt.Sprintf("Executing script: %s", script.Name), true)

	// 在新的 goroutine 中执行脚本
	go func() {
		// 传入回调函数来处理输出
		err := app.scripts.Execute(script, func(output string) {
			app.window.Synchronize(func() {
				if app.logView != nil {
					app.appendLog(output, true)
				}
			})
		})

		if err != nil {
			app.logger.WithError(err).Error("Failed to execute script")
			app.appendLog(fmt.Sprintf("Error executing script: %v", err), true)
			return
		}
	}()
}

// 获取鼠标位置
func getMousePosition() walk.Point {
	var pt win.POINT
	win.GetCursorPos(&pt)
	return walk.Point{X: int(pt.X), Y: int(pt.Y)}
}

// 添加菜单项
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

// 处理搜索框按键
func (app *XScript) handleSearchKeyDown(key walk.Key) {
	switch key {
	case walk.KeyEscape:
		app.window.Hide()
	case walk.KeyReturn:
		app.runSelectedScript()
	case walk.KeyDown:
		// Select next item in list
		if app.resultList != nil {
			curr := app.resultList.CurrentIndex()
			if curr < app.resultList.Model().(walk.ListModel).ItemCount()-1 {
				app.resultList.SetCurrentIndex(curr + 1)
			}
		}
	case walk.KeyUp:
		// Select previous item in list
		if app.resultList != nil {
			curr := app.resultList.CurrentIndex()
			if curr > 0 {
				app.resultList.SetCurrentIndex(curr - 1)
			}
		}
	}
}

// 运行选中的脚本
func (app *XScript) runSelectedScript() {
	if app.resultList == nil || app.resultList.CurrentIndex() < 0 {
		return
	}

	selectedName := app.resultList.Model().([]string)[app.resultList.CurrentIndex()]
	results := app.scripts.Search(selectedName)

	if len(results) > 0 {
		// app.window.Hide()
		app.runScript() // 使用当前搜索框中的文本执行脚本
	}
}
