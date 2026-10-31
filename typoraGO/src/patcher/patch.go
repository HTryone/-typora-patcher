package patcher

// 打补丁与回滚的落地流程。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"typoratool/src/asar"
	"typoratool/src/fuses"
	"typoratool/src/system"
)

var hkcu = syscall.Handle(syscall.HKEY_CURRENT_USER)

// FindFirstRequire 等价于 JS 版的 content.match(/require\s*\([^)]*\)\s*;/):
// 找到第一个「完整语句」形式的 require(...); 并返回插入点（分号之后）。
// 注意：launch.dist.js 开头是 `const fs=require("fs"),path=require("path")…`，
// 那些 require 后面跟的是逗号而不是分号，正则不会命中；真正命中的是
// `Module=require("module");` —— JS 版一直把注入代码放在这里，必须保持一致。
func FindFirstRequire(s string) (int, int) {
	for i := 0; ; {
		idx := strings.Index(s[i:], "require")
		if idx < 0 {
			return -1, -1
		}
		start := i + idx
		// 前一个字符不能是标识符字符（避免匹配 myrequire）
		if start > 0 && isIdentByte(s[start-1]) {
			i = start + len("require")
			continue
		}
		j := start + len("require")
		for j < len(s) && isSpace(s[j]) {
			j++
		}
		if j >= len(s) || s[j] != '(' {
			i = start + len("require")
			continue
		}
		j++ // 跳过 '('
		// 参数里不允许出现 ')'，与 [^)]* 一致
		for j < len(s) && s[j] != ')' {
			j++
		}
		if j >= len(s) {
			return -1, -1
		}
		j++ // 跳过 ')'
		for j < len(s) && isSpace(s[j]) {
			j++
		}
		if j >= len(s) || s[j] != ';' {
			i = start + len("require")
			continue
		}
		return start, j + 1
	}
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func isIdentByte(b byte) bool {
	return b == '_' || b == '$' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// InjectHook 在 launch.dist.js 的第一个完整 require(...); 之后插入注入代码。
func InjectHook(launchPath string, ent Entity, debug bool) error {
	orig, err := os.ReadFile(launchPath)
	if err != nil {
		return fmt.Errorf("读取 launch.dist.js 失败: %w", err)
	}
	content := string(orig)
	if strings.Contains(content, InjectedMarker) {
		return fmt.Errorf("launch.dist.js 里已经有注入代码，请先回滚再重试")
	}
	code := BuildHookCode(ent, debug)
	idx, end := FindFirstRequire(content)
	if idx >= 0 {
		content = content[:end] + code + content[end:]
	} else {
		content = code + content
	}
	return os.WriteFile(launchPath, []byte(content), 0o644)
}

// CleanTrialData 删除 Chromium 的试用期持久化数据（与 JS 版同一份清单）。
func CleanTrialData() {
	if TestMode {
		return
	}
	base := filepath.Join(os.Getenv("APPDATA"), "Typora")
	for _, name := range TrialDirs {
		_ = os.RemoveAll(filepath.Join(base, name))
	}
	for _, k := range TrialRegValues {
		_ = system.RegDeleteValue(RegKey, k)
	}
}

// TrialDirs 是试用态相关的 Chromium 存储目录名（%APPDATA%\Typora 下）。
var TrialDirs = []string{
	"Local Storage", "Session Storage", "SharedStorage", "DIPS",
	"Code Cache", "Cache", "GPUCache", "DawnGraphiteCache",
	"DawnWebGPUCache", "Shared Dictionary", "Network",
}

// TrialRegValues 是试用态相关的注册表值（RegKey 下，只有进入试用态才会写）。
var TrialRegValues = []string{"SLicense", "IDate", "TrialStart", "TDate", "FirstLaunch", "LaunchCount"}

// HasTrialResidue 检测是否残留试用态（注册表试用键是否存在）。
// 用途：绿色版启动器的「检测到才清」逻辑；脚本版激活前也可参考。
func HasTrialResidue() bool {
	for _, k := range TrialRegValues {
		if _, ok := system.RegReadString(hkcu, RegKey, k); ok {
			return true
		}
	}
	return false
}

// ApplyPatch 执行完整打补丁流程。log 用于向界面输出进度。
func ApplyPatch(p Paths, mi MachineInfo, email string, log func(string)) error {
	if err := p.Validate(); err != nil {
		return err
	}
	switch p.State() {
	case StatePatched:
		return fmt.Errorf("这个 Typora 已经打过补丁了；如需重新打补丁，请先执行「回滚」")
	case StateBroken:
		return fmt.Errorf("目录状态异常（app.asar 与 app 目录同时存在）；请先执行「回滚」再打补丁")
	case StateUnknown:
		return fmt.Errorf("没有找到 resources\\app.asar，无法确认这是完整的 Typora 安装目录")
	}

	// 出错时把已做的破坏性改动收回去
	undo := []func(){}
	rollbackNow := func() {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
	}

	log("清理试用期数据…")
	CleanTrialData()

	log("解包 app.asar → resources\\app …")
	if Exists(p.AppDir) {
		if err := os.RemoveAll(p.AppDir); err != nil {
			return fmt.Errorf("删除旧的 app 目录失败: %w", err)
		}
	}
	ar, err := asar.Open(p.Asar)
	if err != nil {
		return err
	}
	err = ar.ExtractAll(p.AppDir)
	ar.Close()
	if err != nil {
		return fmt.Errorf("解包失败: %w", err)
	}
	undo = append(undo, func() { _ = os.RemoveAll(p.AppDir) })

	log("备份原始 app 目录 → resources\\app.bak …")
	if Exists(p.AppBakDir) {
		if err := os.RemoveAll(p.AppBakDir); err != nil {
			return fmt.Errorf("删除旧的 app.bak 失败: %w", err)
		}
	}
	if err := CopyDir(p.AppDir, p.AppBakDir); err != nil {
		return fmt.Errorf("备份 app 目录失败: %w", err)
	}
	undo = append(undo, func() { _ = os.RemoveAll(p.AppBakDir) })

	log("备份原始 app.asar → app.asar.bak …")
	if Exists(p.AsarBak) {
		_ = os.Remove(p.AsarBak)
	}
	if err := os.Rename(p.Asar, p.AsarBak); err != nil {
		return fmt.Errorf("重命名 app.asar 失败: %w", err)
	}
	undo = append(undo, func() { _ = os.Rename(p.AsarBak, p.Asar) })

	log("备份 Typora.exe 并关闭 OnlyLoadAppFromAsar 保险丝 …")
	if !Exists(p.ExeBak) {
		if err := CopyFile(p.Exe, p.ExeBak); err != nil {
			return fmt.Errorf("备份 Typora.exe 失败: %w", err)
		}
		undo = append(undo, func() { _ = os.Remove(p.ExeBak) })
	}
	if _, err := fuses.DisableOnlyLoadAppFromAsar(p.Exe); err != nil {
		return fmt.Errorf("修改保险丝失败: %w", err)
	}

	log("注入授权模块 …")
	ent := Entity{
		DeviceID:    mi.L,
		Fingerprint: mi.I,
		Email:       email,
		Version:     mi.V,
		Date:        TodayStr(),
	}
	if err := InjectHook(p.LaunchDist, ent, false); err != nil {
		rollbackNow()
		return err
	}

	log("写入授权信息 …")
	if TestMode {
		return nil
	}
	if err := system.RegWriteString(RegKey, "SLicense", SlicenseValue); err != nil {
		rollbackNow()
		return err
	}
	if err := system.RegWriteString(RegKey, "IDate", TodayStr()); err != nil {
		rollbackNow()
		return err
	}
	return nil
}

// RollbackPatch 把安装目录恢复成官方原版。
func RollbackPatch(p Paths, log func(string)) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if p.State() == StateClean {
		return fmt.Errorf("这个 Typora 现在是官方原版，无需回滚")
	}
	if Exists(p.AsarBak) {
		log("还原 app.asar …")
		if Exists(p.Asar) {
			_ = os.Remove(p.Asar)
		}
		if err := os.Rename(p.AsarBak, p.Asar); err != nil {
			return fmt.Errorf("还原 app.asar 失败: %w", err)
		}
	}
	if Exists(p.ExeBak) {
		log("还原 Typora.exe …")
		if err := os.Remove(p.Exe); err != nil {
			return fmt.Errorf("删除已修改的 Typora.exe 失败: %w", err)
		}
		if err := os.Rename(p.ExeBak, p.Exe); err != nil {
			return fmt.Errorf("还原 Typora.exe 失败: %w", err)
		}
	}
	if Exists(p.AppDir) {
		log("删除解包目录 app …")
		if err := os.RemoveAll(p.AppDir); err != nil {
			return fmt.Errorf("删除 app 目录失败: %w", err)
		}
	}
	if Exists(p.AppBakDir) {
		if err := os.RemoveAll(p.AppBakDir); err != nil {
			return fmt.Errorf("删除 app.bak 失败: %w", err)
		}
	}
	log("清理注册表中的授权信息 …")
	if TestMode {
		return nil
	}
	for _, k := range []string{"SLicense", "IDate"} {
		_ = system.RegDeleteValue(RegKey, k)
	}
	return nil
}
