// Package patcher 是补丁核心：激活实体、注入模板、打补丁与回滚。
//
// 产物（与 typora_crack.js 完全一致，两种工具可互相回滚）：
//
//	<安装目录>\Typora.exe.bak            原始 exe（改保险丝前备份）
//	<安装目录>\resources\app.asar.bak    原始归档（改名为 .bak）
//	<安装目录>\resources\app\            解包后的目录（真正被 Electron 加载）
//	<安装目录>\resources\app.bak\        原始 app 目录副本（fs hook 的重定向目标）
//
// 关键点：app.bak 必须保留，注入代码会把 resources\app\ 的读取重定向到这里；
// 删了它 Typora 启动即卡死。
package patcher

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"typoratool/src/asar"
	"typoratool/src/system"
)

// RegKey 是 Typora 的注册表键（HKCU 下）。
const RegKey = `Software\Typora`

// SlicenseValue 是写入注册表的授权值。
const SlicenseValue = "RHJlYW1OeWE=#0#1/1/2059"

// TestMode 为 true 时不碰真实注册表与 %APPDATA%\Typora（自测用）。
var TestMode = false

// Entity 是写入注入代码的激活实体。
type Entity struct {
	DeviceID    string
	Fingerprint string
	Email       string
	Version     string
	Date        string
}

// MachineInfo 是 Typora 机器码 Base64 解码后的内容。
type MachineInfo struct {
	L string `json:"l"`
	I string `json:"i"`
	V string `json:"v"`
}

// DecodeMachineCode 解析用户粘贴（或自动生成）的机器码。
func DecodeMachineCode(s string) (MachineInfo, error) {
	var mi MachineInfo
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"`)
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		// 容错：去掉换行与空格
		clean := strings.Map(func(r rune) rune {
			if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, s)
		raw, err = base64.StdEncoding.DecodeString(clean)
		if err != nil {
			return mi, fmt.Errorf("机器码不是合法的 Base64 字符串，请重新复制")
		}
	}
	if err := json.Unmarshal(raw, &mi); err != nil {
		return mi, fmt.Errorf("机器码解码后不是合法 JSON，请确认复制完整（应为 Typora 生成的整段字符串）")
	}
	if mi.I == "" {
		return mi, fmt.Errorf("机器码里缺少 fingerprint（i 字段），无法生成激活实体")
	}
	return mi, nil
}

// TodayStr 返回 MM/DD/YYYY 格式的当天日期。
func TodayStr() string {
	n := time.Now()
	return fmt.Sprintf("%02d/%02d/%d", int(n.Month()), n.Day(), n.Year())
}

// Paths 汇总一个 Typora 安装目录下的所有相关路径。
type Paths struct {
	Root       string
	Exe        string
	ExeBak     string
	Resources  string
	Asar       string
	AsarBak    string
	AppDir     string
	AppBakDir  string
	LaunchDist string
}

// ResolveRoot 把用户给的路径规范化成「含 Typora.exe 的安装根目录」。
// 容错三种常见误选：直接选了 Typora.exe 文件、选了 resources 子目录、
// 选了 resources\app 目录 —— 一律向上找到真正的根。
func ResolveRoot(dir string) string {
	dir = strings.TrimSpace(strings.Trim(strings.TrimSpace(dir), `"`))
	if dir == "" {
		return dir
	}
	if fi, err := os.Stat(dir); err == nil && !fi.IsDir() {
		dir = filepath.Dir(dir) // 选的是文件（多半就是 Typora.exe）
	}
	cur := filepath.Clean(dir)
	for i := 0; i < 3; i++ {
		if Exists(filepath.Join(cur, "Typora.exe")) {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return filepath.Clean(dir)
}

// NewPaths 以安装目录为根推算全部相关路径（自动纠正用户选错的层级）。
func NewPaths(root string) Paths {
	root = ResolveRoot(root)
	res := filepath.Join(root, "resources")
	return Paths{
		Root:       root,
		Exe:        filepath.Join(root, "Typora.exe"),
		ExeBak:     filepath.Join(root, "Typora.exe.bak"),
		Resources:  res,
		Asar:       filepath.Join(res, "app.asar"),
		AsarBak:    filepath.Join(res, "app.asar.bak"),
		AppDir:     filepath.Join(res, "app"),
		AppBakDir:  filepath.Join(res, "app.bak"),
		LaunchDist: filepath.Join(res, "app", "launch.dist.js"),
	}
}

// State 是安装目录当前的状态。
type State int

const (
	StateUnknown State = iota
	StateClean         // 官方原版，可以打补丁
	StatePatched       // 已打补丁
	StateBroken        // 目录残缺
)

// Validate 校验这是不是一个可用的 Typora 安装目录。
func (p Paths) Validate() error {
	if fi, err := os.Stat(p.Exe); err != nil || fi.IsDir() {
		return fmt.Errorf("该目录下没有找到 Typora.exe（请选装了 Typora 的那一层文件夹，例如 D:\\Apps\\Typora）：%s", p.Root)
	}
	return nil
}

// State 探测安装状态。
func (p Paths) State() State {
	_, asarErr := os.Stat(p.Asar)
	asarOK := asarErr == nil
	appOK := IsDir(p.AppDir)
	appBakOK := IsDir(p.AppBakDir)
	switch {
	case asarOK && !appOK:
		return StateClean
	case !asarOK && appOK:
		return StatePatched
	case asarOK && appOK && appBakOK:
		// 上次打到一半（归档还在、目录已建）
		return StateBroken
	default:
		return StateUnknown
	}
}

// Describe 返回状态的可读描述。
func (p Paths) Describe() string {
	switch p.State() {
	case StateClean:
		return "官方原版（未打补丁）"
	case StatePatched:
		return "已打补丁"
	case StateBroken:
		return "状态异常（app.asar 与 app 目录同时存在，可能上次中断）"
	default:
		return "无法识别（缺 app.asar 且缺 app 目录）"
	}
}

// IsDir 判断路径存在且是目录。
func IsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// Exists 判断路径存在。
func Exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// AutoMachineCode 用户没提供机器码时自动生成一个：
// 本地校验只比对 fingerprint，而 fingerprint 运行时由注入代码用本机真实值覆盖
// （v1.0.6 预热轮询 + MFp 注册表缓存，Go 版已修复缓存路径），deviceId 不参与比对，
// 所以烘焙值用占位即可——首次激活无需用户粘贴任何东西。
// fingerprint 优先取 MFp（之前激活过就有本机真实值），没有则用占位符。
func AutoMachineCode(p Paths) (string, MachineInfo, error) {
	fp, _ := system.RegReadString(syscall.HKEY_CURRENT_USER, RegKey, "MFp")
	if fp == "" {
		fp = "auto"
	}
	dev, err := RandomHex(16)
	if err != nil {
		return "", MachineInfo{}, err
	}
	mi := MachineInfo{L: dev, I: fp, V: ReadTyporaVersion(p)}
	raw, err := json.Marshal(MachineInfo{L: mi.L, I: mi.I, V: mi.V})
	if err != nil {
		return "", MachineInfo{}, err
	}
	return base64.StdEncoding.EncodeToString(raw), mi, nil
}

// AutoEmail 自动生成一个展示用邮箱（仅写进激活实体，供 Typora 界面显示）。
func AutoEmail() string {
	if u := os.Getenv("USERNAME"); u != "" {
		return u + "@local.device"
	}
	return "user@local.device"
}

// ReadTyporaVersion 读 Typora 版本号：优先解包目录，其次直接从 asar 里取 package.json。
func ReadTyporaVersion(p Paths) string {
	for _, f := range []string{
		filepath.Join(p.AppBakDir, "package.json"),
		filepath.Join(p.AppDir, "package.json"),
		filepath.Join(p.Resources, "package.json"),
	} {
		if b, err := os.ReadFile(f); err == nil {
			var pkg struct {
				Version string `json:"version"`
			}
			if json.Unmarshal(b, &pkg) == nil && pkg.Version != "" {
				return pkg.Version
			}
		}
	}
	if ar, err := asar.Open(p.Asar); err == nil {
		defer ar.Close()
		if b, err := ar.ReadFile("package.json"); err == nil {
			var pkg struct {
				Version string `json:"version"`
			}
			if json.Unmarshal(b, &pkg) == nil && pkg.Version != "" {
				return pkg.Version
			}
		}
	}
	return ""
}

// CopyDir 递归复制目录。
func CopyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := out.ReadFrom(in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

// CopyFile 单文件复制。
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := out.ReadFrom(in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// DetectTypora 找 Typora 安装目录：缓存 → 注册表 → 常见路径。
// cachePath/cacheValid 由调用方注入（避免依赖 config 存储）。
func DetectTypora(cachedPath string, saveCache func(string)) string {
	if cachedPath != "" && Exists(filepath.Join(cachedPath, "Typora.exe")) {
		return cachedPath
	}
	if p := readAppPath(); p != "" {
		if saveCache != nil {
			saveCache(p)
		}
		return p
	}
	for _, c := range []string{
		`C:\Program Files\Typora`,
		`C:\Program Files (x86)\Typora`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Typora"),
	} {
		if Exists(filepath.Join(c, "Typora.exe")) {
			if saveCache != nil {
				saveCache(c)
			}
			return c
		}
	}
	return ""
}

// readAppPath 从注册表 App Paths 读取 Typora 安装路径（HKLM 与 HKCU 都查）。
func readAppPath() string {
	const appPaths = `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\Typora.exe`
	for _, root := range []syscall.Handle{syscall.HKEY_LOCAL_MACHINE, syscall.HKEY_CURRENT_USER} {
		v, ok := system.RegReadString(root, appPaths, "")
		if !ok || v == "" {
			continue
		}
		v = strings.Trim(v, `"`)
		if strings.HasSuffix(strings.ToLower(v), "typora.exe") {
			v = filepath.Dir(v)
		}
		if Exists(filepath.Join(v, "Typora.exe")) {
			return v
		}
	}
	return ""
}
