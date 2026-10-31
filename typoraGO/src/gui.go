//go:build windows

package main

// 图形界面（业务层）：双击 exe 默认入口；控制台菜单加 --cli。
// Win32 原语全部封装在 ui 包，本文件只做布局与业务接线。
//
// 布局（560x520）：
//
//	Typora 路径  [______________________ ▼]
//	状态: 官方原版（未打补丁） · Typora 1.14.10
//	[ 一键激活 ]  [ 回滚还原 ]  [ 刷新状态 ]  [ 使用教程 ]
//	┌──────────── 日志区（只读）────────────┐
//	│ · 自动检测到 Typora…                 │
//	└──────────────────────────────────────┘
//	机器码与邮箱自动处理，无需填写；详情点「使用教程」

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"typoratool/src/console"
	"typoratool/src/fuses"
	"typoratool/src/patcher"
	"typoratool/src/system"
	"typoratool/src/ui"
)

// 控件 ID
const (
	idcPathLabel  = 1090
	idcPathCombo  = 1001
	idcStateLabel = 1002
	idcActivate   = 1003
	idcRollback   = 1004
	idcStatus     = 1005
	idcLog        = 1006
	idcTutorial   = 1007
	idcBrowse     = 1008
	idcHint       = 1091
)

// 工作线程 → UI 线程的自定义消息
const (
	msgProgress = 0x0400 + 1
	msgDone     = 0x0400 + 2
	msgFailed   = 0x0400 + 3
	msgState    = 0x0400 + 4
)

var (
	guiApp  *ui.App
	guiOnce sync.Once
	// actionMutex 保证同一时刻只跑一个动作
	actionMutex sync.Mutex
)

// RunGUI 进入图形界面。
//
// ⚠️ 必须 runtime.LockOSThread：窗口属于创建它的那个 OS 线程，
// 消息循环也必须在同一线程上跑；Go 调度器一旦把 goroutine 换到别的线程，
// GetMessage 就再也收不到这个消息队列的消息 → 窗口白屏 +「未响应」。
func RunGUI() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	guiOnce.Do(setupGUI)
	if guiApp != nil {
		guiApp.Run()
	}
}

// dbg 排错日志：只有设了 TYPORATOOL_DEBUG=1 才写 %TEMP%\typoratool-debug.log。
func dbg(step string) {
	if os.Getenv("TYPORATOOL_DEBUG") != "1" {
		return
	}
	p := filepath.Join(os.Getenv("TEMP"), "typoratool-debug.log")
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("15:04:05.000"), step)
}

func setupGUI() {
	dbg("setupGUI: begin")
	app, err := ui.NewApp("TyporaToolMainWnd", "Typora 激活工具  ·  Go 版", 640, 630, guiHandler)
	if err != nil {
		dbg("setupGUI: NewApp failed " + err.Error())
		console.Fail("创建窗口失败: " + err.Error())
		os.Exit(1)
	}
	guiApp = app
	dbg("setupGUI: window created")

	// 所有坐标都是 96 DPI 基准，ui.S() 按实际 DPI 换算，避免高缩放屏上糊/错位
	ctl := func(class string, style uintptr, x, y, w, h int32, id uintptr) {
		app.Control(class, style, ui.S(x), ui.S(y), ui.S(w), ui.S(h), id)
	}

	// ── 路径区（分组框里放标签 + 可编辑路径框 + 浏览按钮）────────
	ctl("BUTTON", ui.StyleGroupBox, 14, 10, 612, 58, 1089)
	app.SetText(1089, " 安装位置 ")
	ctl("STATIC", ui.StyleLabel, 30, 37, 92, 24, idcPathLabel)
	app.SetText(idcPathLabel, "Typora 路径")
	ctl("COMBOBOX", ui.StyleCombo, 126, 34, 392, 220, idcPathCombo)
	ctl("BUTTON", ui.StyleButton, 526, 33, 88, 26, idcBrowse)
	app.SetText(idcBrowse, "浏览…")

	// ── 状态行 ──────────────────────────────────────────────
	ctl("STATIC", ui.StyleLabel, 30, 80, 590, 24, idcStateLabel)

	// ── 操作按钮（等宽等高，留出手感）─────────────────────────
	ctl("BUTTON", ui.StyleDefaultButton, 14, 110, 148, 40, idcActivate)
	app.SetText(idcActivate, "一键激活")
	ctl("BUTTON", ui.StyleButton, 172, 110, 140, 40, idcRollback)
	app.SetText(idcRollback, "回滚还原")
	ctl("BUTTON", ui.StyleButton, 322, 110, 140, 40, idcStatus)
	app.SetText(idcStatus, "刷新状态")
	ctl("BUTTON", ui.StyleButton, 472, 110, 140, 40, idcTutorial)
	app.SetText(idcTutorial, "使用教程")

	// ── 日志区 ──────────────────────────────────────────────
	ctl("BUTTON", ui.StyleGroupBox, 14, 162, 612, 378, 1088)
	app.SetText(1088, " 运行日志 ")
	ctl("EDIT", ui.StyleLogEdit, 28, 188, 584, 340, idcLog)

	// ── 底部提示 ────────────────────────────────────────────
	ctl("STATIC", ui.StyleLabel, 30, 548, 590, 22, idcHint)
	app.SetText(idcHint, "机器码与邮箱自动处理，无需填写；详情点「使用教程」")
	dbg("setupGUI: controls created")

	app.Show()
	dbg("setupGUI: shown")

	// 预填路径 + 初始日志
	c := patcher.LoadCache()
	dbg("setupGUI: cache loaded")
	if d := patcher.DetectTypora(c.Path, nil); d != "" {
		app.SetText(idcPathCombo, d)
	} else if c.Path != "" {
		app.SetText(idcPathCombo, c.Path)
	}
	dbg("setupGUI: path filled")
	refreshStateLabel()
	dbg("setupGUI: state label done")
	app.AppendText(idcLog, "欢迎使用 Typora 激活工具（Go 版）")
	app.AppendText(idcLog, "点「一键激活」即可，机器码与邮箱自动处理。")
	app.AppendText(idcLog, "")
	dbg("setupGUI: done")
}

// guiHandler 主窗口消息路由；返回 true 表示已处理。
func guiHandler(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) bool {
	switch msg {
	case 0x0111: // WM_COMMAND
		id := wParam & 0xFFFF
		notif := wParam >> 16
		if notif != 0 { // 只响应按钮点击（BN_CLICKED=0）
			return false
		}
		switch id {
		case idcActivate:
			onActivateClicked()
			return true
		case idcRollback:
			onRollbackClicked()
			return true
		case idcStatus:
			onStatusClicked()
			return true
		case idcTutorial:
			showTutorial()
			return true
		case idcBrowse:
			// 系统文件夹选择框（用户自己挑 Typora 安装目录）
			if dir, ok := ui.PickFolder(guiApp.Hwnd, "选择 Typora 安装目录（要含 Typora.exe）"); ok {
				guiApp.SetText(idcPathCombo, dir)
				// 选一次就记住（下次启动自动带出来）
				c := patcher.LoadCache()
				c.Path = dir
				_ = patcher.SaveCache(c)
				refreshStateLabel()
				guiApp.AppendText(idcLog, "已选择目录："+dir)
			}
			return true
		}
		return false
	case msgState:
		guiApp.SetText(idcStateLabel, ui.TakeString(wParam))
		return true
	case msgProgress:
		guiApp.AppendText(idcLog, ui.TakeString(wParam))
		return true
	case msgDone:
		guiApp.AppendText(idcLog, ui.TakeString(wParam))
		guiApp.SetButtonsEnabled([]uintptr{idcActivate, idcRollback, idcStatus, idcPathCombo, idcTutorial, idcBrowse}, true)
		refreshStateLabel()
		ui.Beep(0)
		return true
	case msgFailed:
		guiApp.AppendText(idcLog, "✗ "+ui.TakeString(wParam))
		guiApp.SetButtonsEnabled([]uintptr{idcActivate, idcRollback, idcStatus, idcPathCombo, idcTutorial, idcBrowse}, true)
		refreshStateLabel()
		ui.Beep(0xFFFFFFFF)
		return true
	case 0x0002: // WM_DESTROY
		user32PostQuit()
		return true
	}
	return false
}

func user32PostQuit() {
	// PostQuitMessage 由 ui 包的消息循环消费
	ui.PostQuit()
}

// postMessage 异步投递（PostMessage + 消息表）：立即返回，不阻塞工作线程，
// 也不会因为 UI 线程忙而卡住。
func postMessage(kind uint32, s string) {
	if guiApp == nil {
		return
	}
	ui.PostMessage(guiApp.Hwnd, kind, ui.PutString(s), 0)
}

func postProgress(s string) { postMessage(msgProgress, s) }
func postDone(s string)     { postMessage(msgDone, s) }
func postFailed(s string)   { postMessage(msgFailed, s) }

// ---------------------------------------------------------------- 动作

func setBusy(busy bool) {
	guiApp.SetButtonsEnabled([]uintptr{idcActivate, idcRollback, idcStatus, idcPathCombo, idcTutorial, idcBrowse}, !busy)
}

func onActivateClicked() {
	if !actionMutex.TryLock() {
		return
	}
	defer actionMutex.Unlock()
	setBusy(true)
	guiApp.AppendText(idcLog, "── 开始激活 ──")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				postFailed(fmt.Sprintf("内部错误: %v", r))
			}
		}()
		runAction(true)
	}()
}

func onRollbackClicked() {
	if !actionMutex.TryLock() {
		return
	}
	defer actionMutex.Unlock()
	setBusy(true)
	guiApp.AppendText(idcLog, "── 开始回滚 ──")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				postFailed(fmt.Sprintf("内部错误: %v", r))
			}
		}()
		runAction(false)
	}()
}

func onStatusClicked() {
	go func() {
		p, err := currentPaths()
		if err != nil {
			postProgress(err.Error())
			return
		}
		postProgress("状态: " + p.Describe())
		if ver := patcher.ReadTyporaVersion(p); ver != "" {
			postProgress("版本: Typora " + ver)
		}
		if st, err := fuses.OnlyLoadAppFromAsarState(p.Exe); err == nil {
			if st == '0' {
				postProgress("保险丝 OnlyLoadAppFromAsar: 关闭（已打补丁）")
			} else {
				postProgress("保险丝 OnlyLoadAppFromAsar: 开启（原版）")
			}
		}
		if patcher.Exists(p.LaunchDist) {
			b, _ := os.ReadFile(p.LaunchDist)
			if strings.Contains(string(b), patcher.InjectedMarker) {
				postProgress("注入代码: 已注入")
			} else {
				postProgress("注入代码: 未注入")
			}
		}
	}()
}

func currentPaths() (patcher.Paths, error) {
	dir := strings.Trim(strings.TrimSpace(guiApp.Text(idcPathCombo)), `"`)
	if dir == "" {
		c := patcher.LoadCache()
		if d := patcher.DetectTypora(c.Path, nil); d != "" {
			dir = d
			guiApp.SetText(idcPathCombo, dir)
		}
	}
	if dir == "" {
		return patcher.Paths{}, fmt.Errorf("没有找到 Typora 安装目录；请在路径框填写 Typora.exe 所在文件夹")
	}
	p := patcher.NewPaths(filepath.Clean(dir))
	if err := p.Validate(); err != nil {
		return patcher.Paths{}, err
	}
	return p, nil
}

func runAction(activate bool) {
	p, err := currentPaths()
	if err != nil {
		postFailed(err.Error())
		return
	}
	c := patcher.LoadCache()
	c.Path = p.Root
	_ = patcher.SaveCache(c)

	st := p.State()
	if activate {
		if st == patcher.StatePatched {
			postFailed("这个 Typora 已经打过补丁了；要重打请先「回滚还原」")
			return
		}
		if st != patcher.StateClean {
			postFailed("目录状态异常（" + p.Describe() + "）；请先「回滚还原」再激活")
			return
		}
		if err := system.MustKillTypora(false, true, func(string, bool) bool { return true }); err != nil {
			postFailed(err.Error())
			return
		}
		// 全自动：机器码/邮箱（缓存优先，缺了自动生成）
		var mi patcher.MachineInfo
		mc := c.MachineCode
		if mc != "" {
			if m, err := patcher.DecodeMachineCode(mc); err == nil {
				mi = m
			}
		}
		if mi.I == "" {
			_, mi, err = patcher.AutoMachineCode(p)
			if err != nil {
				postFailed("自动生成机器码失败: " + err.Error())
				return
			}
		}
		email := c.Email
		if email == "" {
			email = patcher.AutoEmail()
		}
		c.MachineCode = mc
		c.Email = email
		_ = patcher.SaveCache(c)
		postProgress(fmt.Sprintf("目标: %s", p.Root))
		err = patcher.ApplyPatch(p, mi, email, func(s string) { postProgress(s) })
		if err != nil {
			postFailed("激活失败: " + err.Error())
			return
		}
		if ver := patcher.ReadTyporaVersion(p); ver != "" {
			postProgress("版本: Typora " + ver)
		}
		postDone("激活完成！Typora 已可用（自动更新已被拦截，升级请先回滚）")
		launchTypora(p)
	} else {
		if st == patcher.StateClean {
			postFailed("现在是官方原版，无需回滚")
			return
		}
		if err := system.MustKillTypora(false, true, func(string, bool) bool { return true }); err != nil {
			postFailed(err.Error())
			return
		}
		if err := patcher.RollbackPatch(p, func(s string) { postProgress(s) }); err != nil {
			postFailed("回滚失败: " + err.Error())
			return
		}
		postDone("已恢复为官方原版")
	}
}

// refreshStateLabel 立刻用「路径 + 状态」更新标签，版本号在后台线程查完后补上
// （版本号要读 asar/package.json，不能让 UI 线程等 IO）。
func refreshStateLabel() {
	dir := strings.Trim(strings.TrimSpace(guiApp.Text(idcPathCombo)), `"`)
	if dir == "" {
		guiApp.SetText(idcStateLabel, "状态: （未检测到，请填路径或点「使用教程」）")
		return
	}
	p := patcher.NewPaths(dir)
	if err := p.Validate(); err != nil {
		guiApp.SetText(idcStateLabel, "状态: 路径无效（没有 Typora.exe）")
		return
	}
	guiApp.SetText(idcStateLabel, "状态: "+p.Describe())
	go func() {
		ver := patcher.ReadTyporaVersion(p)
		if ver == "" {
			return
		}
		p := patcher.NewPaths(dir)
		ui.PostMessage(guiApp.Hwnd, msgState, ui.PutString("状态: "+p.Describe()+" · Typora "+ver), 0)
	}()
}

// ---------------------------------------------------------------- 教程窗口

// showTutorial 在独立线程开教程窗口：窗口与其消息循环必须同一个 OS 线程
// （否则主窗口会被它的消息循环卡成「未响应」）。
func showTutorial() {
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		app, err := ui.NewApp("TyporaToolTutorialWnd", "使用教程 — 机器码怎么找 · 邮箱怎么写", 760, 660, nil)
		if err != nil {
			return
		}
		app.Control("EDIT", ui.StyleLogEdit, ui.S(12), ui.S(12), ui.S(722), ui.S(598), 2001)
		// Win32 多行 EDIT 只认 \r\n，纯 \n 会被显示成一行，必须先转换
		app.SetText(2001, toCRLF(tutorialText))
		app.Show()
		app.Run()
	}()
}

// toCRLF 把 LF 转成 CRLF（Win32 控件换行要求）。
func toCRLF(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }

const tutorialText = `【一键激活 · 三步走】

  1. 点主窗口的「一键激活」按钮
  2. 等 10~30 秒（进度会显示在日志区）
  3. 看到「激活完成」即可，Typora 会自动打开

  机器码和邮箱都不用填 —— 工具会自动处理：
  · 机器码：工具按你本机信息自动生成，激活时自动校准到本机真实指纹
  · 邮箱：自动生成一个展示用邮箱（只影响激活页面显示什么邮箱，收不到邮件）

──────────────────────────────────────────────

【机器码是什么？去哪找？】

  机器码是 Typora 根据你的电脑硬件生成的一段编号，官方用它来签发许可证。

  位置：打开 Typora → 菜单「帮助」→「我的许可证」→ 点「离线激活」
  就能看到一串很长的字符（Base64 样子，可能带 + / = 号）。

  ※ 本工具不需要你手动粘贴它 —— 只有当你想用「某个特定机器码」时才需要：
    在命令行用 --machine 参数传入，或在控制台菜单（--cli）里选 3 手动填。

──────────────────────────────────────────────

【邮箱怎么写？】

  随便写。这个邮箱只是激活实体里的一个显示字段，Typora 不会真的发邮件过去。
  工具自动生成 <你的用户名>@local.device；想自定义的话：
  命令行 --email "you@example.com"，或控制台菜单（--cli）选 3。

──────────────────────────────────────────────

【安装路径怎么选？】

  要选「装了 Typora 的那个根文件夹」—— 也就是里面能直接看到 Typora.exe 的那一层。

    D:\Apps\Typora                 ✅ 正解（里面有 Typora.exe）
    D:\Apps\Typora\Typora.exe      ⚠️ 选到文件了 → 会自动纠正成上一层
    D:\Apps\Typora\resources       ⚠️ 选深了一层 → 会自动纠正
    D:\Apps\Typora\resources\app   ⚠️ 更深 → 也会自动纠正（往上找三层）

  一般不用你管：工具会先读注册表自己找到（官方安装包装的，装哪个盘都能找到）。
  绿色版 / 免安装版 / 装在非常规目录时，点「浏览…」选一次就记住了，下次自动带出来。

  自动检测顺序：上次记住的路径 → 注册表 App Paths（HKLM 再 HKCU）→ 常见安装位置。

──────────────────────────────────────────────

【常见问题】

  Q: 激活后 Typora 还是显示试用？
  A: ① 关掉 Typora 重新打开（补丁在启动时生效）
     ② 确认日志区没有红色报错
     ③ 如果之前装过别的版本 Typora，可能有旧试用数据残留 ——
        工具激活时会自动清理；还不行就先「回滚还原」再「一键激活」一次。

  Q: 打开就弹「未激活」并且每次都弹？
  A: 这是老版本 Typora 的试用残留（B 站反馈过的那个问题）。
     点「回滚还原」，然后重新「一键激活」。

  Q: 要升级 Typora 官方新版本？
  A: 本补丁已拦截自动更新。想手动升级：先「回滚还原」→ 安装新版 → 再「一键激活」。

  Q: 被杀毒软件拦截？
  A: 未签名 exe 的常见误报，把本工具加入白名单即可。

  Q: 想恢复原版？
  A: 点「回滚还原」，所有备份文件会自动还原，不留残留。

──────────────────────────────────────────────

  仅供学习与个人研究使用，请支持正版 Typora。
`
