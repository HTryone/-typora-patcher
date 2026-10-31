// Package ui 封装本项目用到的 Win32 控件与窗口原语（纯 syscall，零第三方依赖）。
// 业务代码只跟 Window / Control 结构体打交道，不直接碰 Win32 常量。
package ui

import (
	"os"
	"sync"
	"syscall"
	"unsafe"
)

// ---- Win32 常量（本包内部使用）----

const (
	wsChild   = 0x40000000
	wsVisible = 0x10000000
	wsVScroll = 0x00200000
	wsBorder  = 0x00800000
	wsTabStop = 0x00010000
	wsCaption = 0x00C00000
	wsSysMenu = 0x00080000
	wsMinBox  = 0x00020000

	wsOverlappedWindow = wsCaption | wsSysMenu | wsMinBox | 0x00030000 // + THICKFRAME

	esMulti    = 0x0004
	esAutoV    = 0x0040
	esReadOnly = 0x0800
	esWantRet  = 0x1000

	cbsDropdown = 0x0002
	cbsAutoH    = 0x0040

	wmSetText  = 0x000C
	wmGetText  = 0x000D
	wmGetLen   = 0x000E
	wmSetFont  = 0x0030
	emSetSel   = 0x00B1
	emReplSel  = 0x00C2
	bmGetCheck = 0x00F0
	bmSetCheck = 0x00F1

	swShow       = 5
	swRestore    = 9
	colorBtnFace = 15
)

// ---- Win32 API ----

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")
	k32    = syscall.NewLazyDLL("kernel32.dll")

	pRegisterClassExW = user32.NewProc("RegisterClassExW")
	pCreateWindowExW  = user32.NewProc("CreateWindowExW")
	pShowWindow       = user32.NewProc("ShowWindow")
	pGetMessageW      = user32.NewProc("GetMessageW")
	pTranslateMessage = user32.NewProc("TranslateMessage")
	pDispatchMessageW = user32.NewProc("DispatchMessageW")
	pDefWindowProcW   = user32.NewProc("DefWindowProcW")
	pSendMessageW     = user32.NewProc("SendMessageW")
	pSetWindowTextW   = user32.NewProc("SetWindowTextW")
	pGetWindowTextW   = user32.NewProc("GetWindowTextW")
	pGetDlgItem       = user32.NewProc("GetDlgItem")
	pEnableWindow     = user32.NewProc("EnableWindow")
	pPostQuitMessage  = user32.NewProc("PostQuitMessage")
	pGetModuleHandleW = k32.NewProc("GetModuleHandleW")
	pCreateMutexW     = k32.NewProc("CreateMutexW")
	pExtractIconExW   = shell32.NewProc("ExtractIconExW")
	pFindWindowW      = user32.NewProc("FindWindowW")
	pSetForegroundNW  = user32.NewProc("SetForegroundWindow")
	pGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	pCreateFontW      = gdi32.NewProc("CreateFontW")

	// DPI 相关
	pSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	pSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	pGetDpiForSystem               = user32.NewProc("GetDpiForSystem")
	pGetDC                         = user32.NewProc("GetDC")
	pReleaseDC                     = user32.NewProc("ReleaseDC")
	pGetDeviceCaps                 = gdi32.NewProc("GetDeviceCaps")
)

const (
	smCXScreen = 0
	smCYScreen = 1

	dpiAwarenessPerMonitorV2 = ^uintptr(3) // (HANDLE)-4
	logPixelsX               = 88

	// 设计基准：96 DPI（100% 缩放）下的尺寸
	baseFontHeight = 13
)

// dpiScale 是本进程的 DPI 缩放系数（1.0 = 100%）。所有界面尺寸都乘它。
var dpiScale = 1.0
var dpiReady bool

// InitDPI 声明本进程为 Per-Monitor V2 DPI 感知并算出缩放系数。
// 必须在创建任何窗口之前调用；否则 Windows 会把界面位图拉伸 → 糊。
func InitDPI() {
	if dpiReady {
		return
	}
	dpiReady = true
	// 首选 Per-Monitor V2（Win10 1703+），失败退回系统级 DPI 感知
	if r, _, _ := pSetProcessDpiAwarenessContext.Call(dpiAwarenessPerMonitorV2); r == 0 {
		pSetProcessDPIAware.Call()
	}
	dpi := int(getSystemDPI())
	if dpi <= 0 {
		dpi = 96
	}
	dpiScale = float64(dpi) / 96.0
	if dpiScale < 1 {
		dpiScale = 1
	}
}

func getSystemDPI() uintptr {
	if r, _, _ := pGetDpiForSystem.Call(); r > 0 {
		return r
	}
	// 退回老 API：GetDeviceCaps(screenDC, LOGPIXELSX)
	hdc, _, _ := pGetDC.Call(0)
	if hdc == 0 {
		return 96
	}
	defer pReleaseDC.Call(0, hdc)
	r, _, _ := pGetDeviceCaps.Call(hdc, logPixelsX)
	if r == 0 {
		return 96
	}
	return r
}

// Scale 返回当前 DPI 缩放系数。
func Scale() float64 { return dpiScale }

// S 把设计尺寸（96 DPI 基准）换算成当前 DPI 的实际像素。
func S(v int32) int32 {
	f := float64(v) * dpiScale
	if f < 0 {
		f -= 0.5
	} else {
		f += 0.5
	}
	return int32(f)
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   syscall.Handle
	Icon       syscall.Handle
	Cursor     syscall.Handle
	Background syscall.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     syscall.Handle
}

type msgStruct struct {
	Hwnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

func w16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

// ---- 对外 API ----

// Handler 是窗口过程回调：返回 true 表示已处理消息。
type Handler func(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) bool

// App 管理一个主窗口与其控件。
type App struct {
	Hwnd  syscall.Handle
	font  uintptr
	items map[uintptr]syscall.Handle
}

// NewApp 创建主窗口（居中显示，微软雅黑字体）。
// class 必须全局唯一；title 是窗口标题；handler 是消息回调（可传 nil）。
func NewApp(class, title string, width, height int32, handler Handler) (*App, error) {
	InitDPI()
	inst, _, _ := pGetModuleHandleW.Call(0)

	var wc wndClassEx
	wc.Size = uint32(unsafe.Sizeof(wc))
	if handler != nil {
		cb := syscall.NewCallback(func(hwnd syscall.Handle, msg uint32, wp, lp uintptr) uintptr {
			if handler(hwnd, msg, wp, lp) {
				return 0
			}
			r, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wp, lp)
			return r
		})
		wc.WndProc = cb
	} else {
		wc.WndProc = pDefWindowProcW.Addr()
	}
	wc.Instance = syscall.Handle(inst)
	wc.ClassName = w16(class)
	wc.Background = syscall.Handle(colorBtnFace + 1)
	// 窗口/任务栏图标：从 exe 自身的图标资源按索引取（不依赖资源 ID），
	// 不设的话 Windows 会给默认的通用图标。
	if exe, err := os.Executable(); err == nil {
		var large, small uintptr
		n, _, _ := pExtractIconExW.Call(
			uintptr(unsafe.Pointer(w16(exe))), 0,
			uintptr(unsafe.Pointer(&large)), uintptr(unsafe.Pointer(&small)), 1)
		if n > 0 {
			wc.Icon = syscall.Handle(large)
			wc.IconSm = syscall.Handle(small)
		}
	}
	// 同一个类重复注册会返回 ERROR_CLASS_ALREADY_EXISTS(1410)，这是允许的
	//（教程窗口可以反复打开）
	if r, _, _ := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		if err := syscall.GetLastError(); err != nil && err.(syscall.Errno) != 1410 {
			return nil, err
		}
	}

	width = S(width)
	height = S(height)
	// 按屏幕尺寸居中
	sw, _, _ := pGetSystemMetrics.Call(smCXScreen)
	sh, _, _ := pGetSystemMetrics.Call(smCYScreen)
	x := (int32(sw) - width) / 2
	y := (int32(sh) - height) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	h, _, err := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(w16(class))),
		uintptr(unsafe.Pointer(w16(title))),
		uintptr(wsOverlappedWindow),
		uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		0, 0, inst, 0)
	if h == 0 {
		return nil, err
	}
	a := &App{Hwnd: syscall.Handle(h), items: map[uintptr]syscall.Handle{}}
	// 微软雅黑，字高随 DPI 放大（CreateFontW 负值 = 字符高度，用补码传）
	fontH := int32(-(float64(baseFontHeight) * dpiScale))
	a.font, _, _ = pCreateFontW.Call(
		uintptr(int64(fontH)), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(w16("Microsoft YaHei UI"))))
	return a, nil
}

// Show 显示主窗口。
func (a *App) Show() { pShowWindow.Call(uintptr(a.Hwnd), swShow) }

// Control 创建一个子控件并返回其句柄。
// id 用于后续 Get()/事件路由；styleExtra 追加控件专属样式。
func (a *App) Control(class string, styleExtra uintptr, x, y, width, height int32, id uintptr) syscall.Handle {
	h, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(w16(class))),
		0,
		wsChild|wsVisible|wsTabStop|styleExtra,
		uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		uintptr(a.Hwnd), id, uintptr(a.Instance()), 0)
	if h != 0 && a.font != 0 {
		pSendMessageW.Call(h, wmSetFont, a.font, 1)
	}
	if h != 0 {
		a.items[id] = syscall.Handle(h)
	}
	return syscall.Handle(h)
}

// Instance 返回模块句柄。
func (a *App) Instance() syscall.Handle {
	h, _, _ := pGetModuleHandleW.Call(0)
	return syscall.Handle(h)
}

// Get 按 id 取控件句柄。
func (a *App) Get(id uintptr) syscall.Handle {
	if h, ok := a.items[id]; ok {
		return h
	}
	h, _, _ := pGetDlgItem.Call(uintptr(a.Hwnd), id)
	if h != 0 {
		a.items[id] = syscall.Handle(h)
	}
	return syscall.Handle(h)
}

// SetText 设置控件文本（用 SetWindowTextW，不走自消息，避免重入）。
func (a *App) SetText(id uintptr, text string) {
	if h := a.Get(id); h != 0 {
		pSetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(w16(text))))
	}
}

// Text 读取控件文本（用 GetWindowTextW，不向自身发消息 —— SendMessage 到自己的
// 窗口会触发 Go 回调重入，实测会卡死 UI 线程）。
func (a *App) Text(id uintptr) string {
	h := a.Get(id)
	if h == 0 {
		return ""
	}
	buf := make([]uint16, 1024)
	pGetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}

// AppendText 向多行编辑框末尾追加一行。
func (a *App) AppendText(id uintptr, line string) {
	h := a.Get(id)
	if h == 0 {
		return
	}
	pSendMessageW.Call(uintptr(h), emSetSel, ^uintptr(0), ^uintptr(0))
	pSendMessageW.Call(uintptr(h), emReplSel, 0, uintptr(unsafe.Pointer(w16(line+"\r\n"))))
}

// Enable 启用/禁用控件。
func (a *App) Enable(id uintptr, on bool) {
	v := uintptr(0)
	if on {
		v = 1
	}
	if h := a.Get(id); h != 0 {
		pEnableWindow.Call(uintptr(h), v)
	}
}

// SetButtonsEnabled 批量启用/禁用。
func (a *App) SetButtonsEnabled(ids []uintptr, on bool) {
	for _, id := range ids {
		a.Enable(id, on)
	}
}

// IsChecked 读复选框状态。
func (a *App) IsChecked(id uintptr) bool {
	h := a.Get(id)
	if h == 0 {
		return false
	}
	r, _, _ := pSendMessageW.Call(uintptr(h), bmGetCheck, 0, 0)
	return r == 1
}

// Beep 提示音（0=OK，-1=错误）。
func Beep(kind uint32) { user32.NewProc("MessageBeep").Call(uintptr(int32(kind))) }

// SendMessage 向窗口发送消息（同步）。
func SendMessage(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	r, _, _ := pSendMessageW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

// PostMessage 投递消息（异步，立即返回）。跨线程通知一律用它，避免死锁。
func PostMessage(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) {
	user32.NewProc("PostMessageW").Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
}

// PostQuit 结束消息循环。
func PostQuit() { pPostQuitMessage.Call(0) }

// ---- 单实例锁（PID 锁）----

var singleMutex syscall.Handle

// AcquireSingleInstance 用命名互斥体做单实例锁：返回 false 表示已有实例在跑。
// 句柄故意不释放，进程退出时由系统回收。
func AcquireSingleInstance(name string) bool {
	h, _, err := pCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(w16(name))))
	if h == 0 {
		return true // 创建失败就不拦，放行
	}
	if errno, ok := err.(syscall.Errno); ok && errno == 183 { // ERROR_ALREADY_EXISTS
		return false
	}
	singleMutex = syscall.Handle(h)
	return true
}

// FocusWindow 把已存在的同类名窗口恢复并激活到前台。
func FocusWindow(class string) bool {
	h, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(w16(class))), 0)
	if h == 0 {
		return false
	}
	pShowWindow.Call(h, swRestore)
	pSetForegroundNW.Call(h)
	return true
}

// ---- 文件夹选择框 ----

var (
	shell32               = syscall.NewLazyDLL("shell32.dll")
	ole32                 = syscall.NewLazyDLL("ole32.dll")
	pSHBrowseForFolderW   = shell32.NewProc("SHBrowseForFolderW")
	pSHGetPathFromIDListW = shell32.NewProc("SHGetPathFromIDListW")
	pCoTaskMemFree        = ole32.NewProc("CoTaskMemFree")
)

type browseInfoW struct {
	HwndOwner      syscall.Handle
	PidlRoot       uintptr
	PszDisplayName *uint16
	LpszTitle      *uint16
	UlFlags        uint32
	Lpfn           uintptr
	LParam         uintptr
	IImage         int32
}

// PickFolder 弹出系统「选择文件夹」对话框；用户取消时 ok=false。
func PickFolder(owner syscall.Handle, title string) (string, bool) {
	const (
		bifReturnOnlyFSDirs = 0x0001
		bifNewDialogStyle   = 0x0040
		bifEditBox          = 0x0010
	)
	display := make([]uint16, 260)
	var bi browseInfoW
	bi.HwndOwner = owner
	bi.PszDisplayName = &display[0]
	bi.LpszTitle = w16(title)
	bi.UlFlags = bifReturnOnlyFSDirs | bifNewDialogStyle | bifEditBox

	pidl, _, _ := pSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return "", false
	}
	defer pCoTaskMemFree.Call(pidl)

	buf := make([]uint16, 1024)
	r, _, _ := pSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&buf[0])))
	if r == 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

// ---- 跨线程字符串传递（PostMessage 用）----

var (
	strMu    sync.Mutex
	strSeq   uintptr
	strStore = map[uintptr]string{}
)

// PutString 登记一个字符串，返回 ID；配合 TakeString 在 UI 线程取出。
func PutString(s string) uintptr {
	strMu.Lock()
	defer strMu.Unlock()
	strSeq++
	id := strSeq
	strStore[id] = s
	return id
}

// TakeString 取出并删除 PutString 登记的字符串。
func TakeString(id uintptr) string {
	strMu.Lock()
	defer strMu.Unlock()
	s := strStore[id]
	delete(strStore, id)
	return s
}

// MessageBox 弹提示框（0=信息，0x30=警告，0x10=错误）。
func MessageBox(title, text string, icon uintptr) {
	user32.NewProc("MessageBoxW").Call(0,
		uintptr(unsafe.Pointer(w16(text))),
		uintptr(unsafe.Pointer(w16(title))),
		icon)
}

// Run 进入消息循环（阻塞到窗口关闭）。
func (a *App) Run() {
	var m msgStruct
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if r == 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// ---- 样式常量（导出给业务层拼控件）----

const (
	// Window 样式
	StyleWindow = wsOverlappedWindow

	// Button
	StyleButton        = 0
	StyleDefaultButton = 0x0001 // BS_DEFPUSHBUTTON：回车触发、视觉加粗边框

	// 分组框（BUTTON + BS_GROUPBOX）
	StyleGroupBox = 0x0007 | wsGroup

	// Label（STATIC）
	StyleLabel = 0

	// 单行输入（EDIT）：ES_AUTOHSCROLL=0x80 + 边框
	StyleInput = 0x80 | wsBorder

	// 多行只读日志（EDIT）
	StyleLogEdit = wsVScroll | esMulti | esAutoV | esReadOnly | esWantRet | wsBorder

	// 下拉框（COMBOBOX）
	StyleCombo = wsVScroll | cbsDropdown | cbsAutoH

	// 复选框（BUTTON）：BS_CHECKBOX=0x2 + WS_GROUP
	StyleCheckbox = 0x0002 | wsGroup
)

const wsGroup = 0x00020000
