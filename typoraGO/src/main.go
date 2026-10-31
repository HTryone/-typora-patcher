package main

// 入口：控制台菜单 + 命令行开关。
//
//	typoratool.exe                       交互菜单
//	typoratool.exe --status              只读查看状态后退出
//	typoratool.exe --activate --yes      直接激活（全自动，零输入）
//	typoratool.exe --rollback --yes      直接回滚
//	typoratool.exe --dir "D:\Apps\Typora" 指定安装目录
//	typoratool.exe --machine <码> --email <邮箱>
//	typoratool.exe --no-elevate          不自动提权（调试用）
//	typoratool.exe --no-color            关闭彩色输出

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"typoratool/src/console"
	"typoratool/src/fuses"
	"typoratool/src/patcher"
	"typoratool/src/system"
	"typoratool/src/ui"
)

var (
	kernel32Attach        = syscall.NewLazyDLL("kernel32.dll")
	kernel32AttachConsole = kernel32Attach.NewProc("AttachConsole")
	kernel32AllocConsole  = kernel32Attach.NewProc("AllocConsole")
)

func main() {
	var (
		flagStatus   = flag.Bool("status", false, "只读查看状态")
		flagActivate = flag.Bool("activate", false, "直接激活")
		flagRollback = flag.Bool("rollback", false, "直接回滚")
		flagYes      = flag.Bool("yes", false, "跳过确认（配合 --activate / --rollback）")
		flagDir      = flag.String("dir", "", "指定 Typora 安装目录")
		flagMachine  = flag.String("machine", "", "机器码（Base64）")
		flagEmail    = flag.String("email", "", "邮箱")
		flagNoElev   = flag.Bool("no-elevate", false, "不自动提权")
		flagNoColor  = flag.Bool("no-color", false, "关闭彩色输出")
		flagCLI      = flag.Bool("cli", false, "用控制台菜单（默认是图形界面）")
	)
	flag.Usage = usage
	flag.Parse()

	console.SetColor(!*flagNoColor)
	// 自测用：设了该环境变量就不写真实注册表、不清真实 %APPDATA%\Typora
	if os.Getenv("TYPORATOOL_TEST") == "1" {
		patcher.TestMode = true
	}

	if !*flagNoElev && !system.IsElevated() {
		if err := system.RelaunchElevated(); err != nil {
			if *flagCLI || *flagStatus || *flagActivate || *flagRollback {
				console.Fail(err.Error())
				console.Pause()
			} else {
				ui.MessageBox("Typora 激活工具", err.Error()+"\n\n请右键本程序选择「以管理员身份运行」。", mbIconWarning)
			}
			os.Exit(1)
		}
		if *flagCLI || *flagStatus || *flagActivate || *flagRollback {
			console.Info("已请求管理员权限，请在新窗口继续操作…")
		}
		return
	}

	cache := patcher.LoadCache()

	switch {
	case *flagStatus:
		attachConsole()
		os.Exit(runStatus(cache, *flagDir))
	case *flagActivate:
		attachConsole()
		os.Exit(runActivate(cache, *flagDir, *flagMachine, *flagEmail, *flagYes))
	case *flagRollback:
		attachConsole()
		os.Exit(runRollback(cache, *flagDir, *flagYes))
	case *flagCLI:
		attachConsole()
		runMenu(cache, *flagDir)
	default:
		// 禁止重复启动：已有实例就把它唤到前台，然后静默退出
		if !ui.AcquireSingleInstance("Local\\TyporaTool.SingleInstance") {
			ui.FocusWindow("TyporaToolMainWnd")
			return
		}
		RunGUI()
	}
}

const mbIconWarning = 0x30

// attachConsole 让命令行模式能输出到「调用它的那个控制台」。
// 本程序编译为 GUI 子系统（双击不弹 CMD 窗口），所以需要手动挂接：
// 优先复用父进程的控制台（在 cmd 里运行时就打印在同一个窗口），
// 没有父控制台时（双击带 --cli）才新开一个。
//
// 注意：输出被重定向（`| more`、`> log.txt`）时标准句柄本来就有效，
// 这时绝不能抢过来改成 CONOUT$，否则管道里什么都读不到。
func attachConsole() {
	if _, err := os.Stdout.Stat(); err == nil {
		return
	}
	const attachParentProcess = ^uintptr(0) // -1
	r, _, _ := kernel32AttachConsole.Call(attachParentProcess)
	if r == 0 {
		kernel32AllocConsole.Call()
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = f
	}
	if f, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
		os.Stdin = f
	}
}

func usage() {
	fmt.Println("Typora 激活工具（Go 版）")
	fmt.Println()
	fmt.Println("  typoratool.exe                         交互菜单")
	fmt.Println("  typoratool.exe --status                查看状态")
	fmt.Println("  typoratool.exe --activate --yes        直接激活（零输入）")
	fmt.Println("  typoratool.exe --rollback --yes        回滚为官方原版")
	fmt.Println("  --dir <路径>   指定 Typora 安装目录")
	fmt.Println("  --machine <码> --email <邮箱>  指定机器码/邮箱")
	fmt.Println("  --no-elevate   不自动提权（调试）")
	fmt.Println("  --no-color     关闭彩色")
}

// ---------------------------------------------------------------- 菜单

func runMenu(cache patcher.Cache, dirOverride string) {
	if dirOverride != "" {
		cache.Path = dirOverride
	}
	for {
		console.Banner()
		showHeader(&cache)
		fmt.Println()
		fmt.Println("  1. 一键激活（机器码/邮箱全自动）")
		fmt.Println("  2. 回滚还原为官方原版")
		fmt.Println("  3. 修改安装目录 / 机器码 / 邮箱")
		fmt.Println("  4. 查看当前状态")
		fmt.Println("  0. 退出")
		fmt.Println()
		fmt.Print("  请选择: ")
		switch console.ReadLine() {
		case "1":
			doActivate(&cache, false)
			console.Pause()
		case "2":
			doRollback(&cache, false)
			console.Pause()
		case "3":
			doConfig(&cache)
			console.Pause()
		case "4":
			doStatus(cache)
			console.Pause()
		case "0", "q", "exit":
			return
		case "":
			// 直接回车：重绘菜单
		default:
			console.Fail("没有这个选项")
		}
	}
}

// showHeader 显示当前目录与缓存信息。
func showHeader(cache *patcher.Cache) {
	if cache.Path == "" {
		if d := patcher.DetectTypora(cache.Path, func(p string) { cache.Path = p; _ = patcher.SaveCache(*cache) }); d != "" {
			console.Info("自动检测到 Typora：" + d)
		}
	}
	dir := cache.Path
	if dir == "" {
		dir = "（未找到，请用菜单 3 手动指定）"
	}
	fmt.Printf("\n  安装目录: %s\n", console.Paint(console.ColorCyan, dir))
	if cache.Path != "" {
		p := patcher.NewPaths(cache.Path)
		fmt.Printf("  当前状态: %s\n", p.Describe())
	}
	mc, em := "未保存", "未保存"
	if cache.MachineCode != "" {
		mc = "已保存"
	}
	if cache.Email != "" {
		em = cache.Email
	}
	fmt.Printf("  机器码: %s    邮箱: %s\n", mc, em)
}

// ---------------------------------------------------------------- 命令行包装

func runStatus(cache patcher.Cache, dir string) int {
	if dir != "" {
		cache.Path = dir
	}
	if cache.Path == "" {
		cache.Path = patcher.DetectTypora(cache.Path, nil)
	}
	doStatus(cache)
	if cache.Path == "" {
		return 1
	}
	return 0
}

func runActivate(cache patcher.Cache, dir, machine, email string, yes bool) int {
	if dir != "" {
		cache.Path = dir
	}
	if machine != "" {
		cache.MachineCode = machine
	}
	if email != "" {
		cache.Email = email
	}
	if err := activate(&cache, yes); err != nil {
		console.Fail(err.Error())
		return 1
	}
	return 0
}

func runRollback(cache patcher.Cache, dir string, yes bool) int {
	if dir != "" {
		cache.Path = dir
	}
	if err := rollback(&cache, yes); err != nil {
		console.Fail(err.Error())
		return 1
	}
	return 0
}

// ---------------------------------------------------------------- 具体动作

// ensureDir 确定并校验安装目录。
func ensureDir(cache *patcher.Cache, assumeYes bool) (patcher.Paths, error) {
	if cache.Path != "" {
		p := patcher.NewPaths(cache.Path)
		if p.Validate() == nil {
			return p, nil
		}
		console.Warn("缓存里的目录已失效：" + cache.Path)
		cache.Path = ""
	}
	if d := patcher.DetectTypora(cache.Path, func(p string) { cache.Path = p; _ = patcher.SaveCache(*cache) }); d != "" {
		// 自动检测到就直接用，不再反问（真·一键）
		console.Info("自动检测到 Typora：" + d)
		cache.Path = d
		_ = patcher.SaveCache(*cache)
		return patcher.NewPaths(d), nil
	}
	for {
		d := console.Ask("请输入 Typora 安装目录（含 Typora.exe）", "")
		d = strings.Trim(d, `"`)
		if d == "" {
			return patcher.Paths{}, fmt.Errorf("未提供安装目录")
		}
		p := patcher.NewPaths(filepath.Clean(d))
		if err := p.Validate(); err != nil {
			console.Fail(err.Error())
			continue
		}
		cache.Path = p.Root
		_ = patcher.SaveCache(*cache)
		return p, nil
	}
}

func doActivate(cache *patcher.Cache, assumeYes bool) {
	if err := activate(cache, assumeYes); err != nil {
		console.Fail(err.Error())
	}
}

func activate(cache *patcher.Cache, assumeYes bool) error {
	p, err := ensureDir(cache, assumeYes)
	if err != nil {
		return err
	}
	fmt.Printf("\n  安装目录: %s\n  当前状态: %s\n\n", p.Root, p.Describe())
	if p.State() == patcher.StatePatched {
		return fmt.Errorf("已经是打过补丁的状态；要重打请先执行「回滚」")
	}

	// 机器码：缓存的直接用；没有就自动生成，全程零输入。
	// 依据：本地校验只比对 fingerprint，而 fingerprint 运行时由注入代码取本机真实值覆盖，
	// deviceId 不参与比对（2026-10-23 分离变量实验定案），所以自动生成完全等效。
	var mi patcher.MachineInfo
	mc := cache.MachineCode
	if mc != "" {
		m, err := patcher.DecodeMachineCode(mc)
		if err != nil {
			console.Warn("已保存的机器码无法解析（" + err.Error() + "），改用自动生成")
			mc = ""
		} else {
			mi = m
			console.Info("使用已保存的机器码：" + mi.I)
		}
	}
	if mc == "" {
		mc, mi, err = patcher.AutoMachineCode(p)
		if err != nil {
			return fmt.Errorf("自动生成机器码失败：%w", err)
		}
		console.Info("机器码已自动生成（fingerprint 运行时取本机真实值）：" + mi.I)
	}

	// 邮箱：同理，缓存 → 自动生成，零输入。
	email := cache.Email
	if email == "" {
		email = patcher.AutoEmail()
		console.Info("邮箱已自动生成：" + email)
	}

	cache.MachineCode = mc
	cache.Email = email
	if err := patcher.SaveCache(*cache); err != nil {
		console.Warn("保存配置失败（不影响激活）：" + err.Error())
	}

	fmt.Println()
	console.Info("deviceId   : " + mi.L)
	console.Info("fingerprint: " + mi.I)
	console.Info("version    : " + mi.V)
	console.Info("email      : " + email)
	fmt.Println()

	// 真·一键：选 1 即开始，不再确认； Typora 在运行时才问一次（防丢未保存内容）
	if err := system.MustKillTypora(true, assumeYes, console.Confirm); err != nil {
		return err
	}

	fmt.Println()
	err = patcher.ApplyPatch(p, mi, email, func(s string) { console.Info(s) })
	if err != nil {
		fmt.Println()
		console.Fail("激活失败：" + err.Error())
		return fmt.Errorf("激活未完成")
	}
	fmt.Println()
	console.Ok("激活完成！")
	fmt.Println("  提示：首次启动较慢属正常；若 Typora 提示更新，选择「跳过此版本」即可。")
	launchTypora(p)
	return nil
}

func doRollback(cache *patcher.Cache, assumeYes bool) {
	if err := rollback(cache, assumeYes); err != nil {
		console.Fail(err.Error())
	}
}

func rollback(cache *patcher.Cache, assumeYes bool) error {
	p, err := ensureDir(cache, assumeYes)
	if err != nil {
		return err
	}
	fmt.Printf("\n  安装目录: %s\n  当前状态: %s\n\n", p.Root, p.Describe())
	if p.State() == patcher.StateClean {
		return fmt.Errorf("现在是官方原版，无需回滚")
	}
	if !assumeYes && !console.Confirm("回滚会删除补丁并恢复官方文件，继续？", false) {
		return fmt.Errorf("已取消")
	}
	if err := system.MustKillTypora(true, assumeYes, console.Confirm); err != nil {
		return err
	}
	fmt.Println()
	if err := patcher.RollbackPatch(p, func(s string) { console.Info(s) }); err != nil {
		console.Fail("回滚失败：" + err.Error())
		return fmt.Errorf("回滚未完成")
	}
	fmt.Println()
	console.Ok("已恢复为官方原版")
	return nil
}

func doStatus(cache patcher.Cache) {
	if cache.Path == "" {
		cache.Path = patcher.DetectTypora(cache.Path, nil)
	}
	if cache.Path == "" {
		console.Warn("没有找到 Typora 安装目录（可用菜单 3 手动指定）")
		return
	}
	p := patcher.NewPaths(cache.Path)
	fmt.Printf("\n  安装目录  : %s\n", p.Root)
	if err := p.Validate(); err != nil {
		console.Fail(err.Error())
		return
	}
	fmt.Printf("  安装状态  : %s\n", p.Describe())
	if ver := patcher.ReadTyporaVersion(p); ver != "" {
		fmt.Printf("  Typora 版本: %s\n", ver)
	}
	fmt.Printf("  app.asar  : %s\n", existsText(p.Asar))
	fmt.Printf("  app.asar.bak: %s\n", existsText(p.AsarBak))
	fmt.Printf("  app 目录  : %s\n", existsText(p.AppDir))
	fmt.Printf("  app.bak   : %s\n", existsText(p.AppBakDir))
	fmt.Printf("  exe 备份  : %s\n", existsText(p.ExeBak))
	if st, err := fuses.OnlyLoadAppFromAsarState(p.Exe); err == nil {
		txt := "开启（原版）"
		if st == '0' {
			txt = "关闭（已打补丁）"
		}
		fmt.Printf("  OnlyLoadAppFromAsar: %s\n", txt)
	}
	if patcher.Exists(p.LaunchDist) {
		b, _ := os.ReadFile(p.LaunchDist)
		if strings.Contains(string(b), patcher.InjectedMarker) {
			fmt.Printf("  注入代码  : 已注入\n")
		} else {
			fmt.Printf("  注入代码  : 未注入\n")
		}
	}
	sl, okSl := system.RegReadString(syscall.HKEY_CURRENT_USER, patcher.RegKey, "SLicense")
	id, okId := system.RegReadString(syscall.HKEY_CURRENT_USER, patcher.RegKey, "IDate")
	fmt.Printf("  注册表 SLicense: %s\n", valueText(sl, okSl))
	fmt.Printf("  注册表 IDate   : %s\n", valueText(id, okId))
	if cache.MachineCode != "" {
		if mi, err := patcher.DecodeMachineCode(cache.MachineCode); err == nil {
			fmt.Printf("  已存机器码: fingerprint=%s version=%s\n", mi.I, mi.V)
		}
	}
	if cache.Email != "" {
		fmt.Printf("  已存邮箱  : %s\n", cache.Email)
	}
	fmt.Println()
}

func existsText(p string) string {
	if patcher.Exists(p) {
		return "存在"
	}
	return "不存在"
}

func valueText(v string, ok bool) string {
	if !ok {
		return "不存在"
	}
	return v
}

// doConfig 修改安装目录 / 机器码 / 邮箱。
func doConfig(cache *patcher.Cache) {
	fmt.Println()
	fmt.Println("  1. 修改安装目录")
	fmt.Println("  2. 修改机器码")
	fmt.Println("  3. 修改邮箱")
	fmt.Println("  0. 返回")
	fmt.Print("  请选择: ")
	switch console.ReadLine() {
	case "1":
		d := console.Ask("新的 Typora 安装目录", cache.Path)
		d = strings.Trim(d, `"`)
		if d == "" {
			return
		}
		p := patcher.NewPaths(filepath.Clean(d))
		if err := p.Validate(); err != nil {
			console.Fail(err.Error())
			return
		}
		cache.Path = p.Root
		if err := patcher.SaveCache(*cache); err != nil {
			console.Fail("保存失败：" + err.Error())
			return
		}
		console.Ok("安装目录已保存")
	case "2":
		mc := console.Ask("机器码（留空则不修改）", "")
		if mc == "" {
			return
		}
		mi, err := patcher.DecodeMachineCode(mc)
		if err != nil {
			console.Fail(err.Error())
			return
		}
		cache.MachineCode = mc
		if err := patcher.SaveCache(*cache); err != nil {
			console.Fail("保存失败：" + err.Error())
			return
		}
		console.Ok("机器码已保存：fingerprint=" + mi.I)
	case "3":
		em := console.Ask("邮箱", cache.Email)
		if em == "" {
			return
		}
		cache.Email = em
		if err := patcher.SaveCache(*cache); err != nil {
			console.Fail("保存失败：" + err.Error())
			return
		}
		console.Ok("邮箱已保存")
	}
}

// launchTypora 以分离进程启动 Typora（DETACHED_PROCESS：不继承控制台与标准句柄，
// 管道/脚本调用时不会因句柄被占而卡住；双击场景照常弹 GUI）。
func launchTypora(p patcher.Paths) {
	cmd := exec.Command(p.Exe)
	cmd.Dir = p.Root
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008} // DETACHED_PROCESS
	if err := cmd.Start(); err != nil {
		console.Warn("启动 Typora 失败：" + err.Error())
		return
	}
	console.Info("已启动 Typora")
	// 不 Wait：让 Typora 独立于本工具运行，工具随时可退出
	go func() { _ = cmd.Wait() }()
}
