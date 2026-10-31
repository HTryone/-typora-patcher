// Package system 封装 Windows 系统层操作：注册表、进程、提权、自检。
// 全部走 syscall 直调，不引入任何第三方依赖。
package system

import (
	"fmt"
	"syscall"
	"unsafe"
)

// ---- 提权 ----

var (
	shell32           = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
	user32            = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW   = user32.NewProc("MessageBoxW")
)

const (
	swShowNormal   = 1
	errorCancelled = 1223
)

// IsElevated 判断当前进程是否已是管理员（查 token 的 TokenElevation）。
func IsElevated() bool {
	tok, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer tok.Close()
	const tokenElevation = 20
	var elevated uint32
	var retLen uint32
	if err := syscall.GetTokenInformation(tok, tokenElevation,
		(*byte)(unsafe.Pointer(&elevated)), uint32(unsafe.Sizeof(elevated)), &retLen); err != nil {
		return false
	}
	return elevated != 0
}

// RelaunchElevated 以管理员身份重启自己，成功启动后调用方应直接退出。
// 传 noElevate 标记防止提权后的进程再次尝试提权。
func RelaunchElevated() error {
	exe, err := osExecutable()
	if err != nil {
		return err
	}
	args := osArgs()
	filtered := args[:0:0]
	for _, a := range args {
		if a != "--no-elevate" {
			filtered = append(filtered, a)
		}
	}
	filtered = append(filtered, "--no-elevate")
	cmdline := quoteArgs(append([]string{exe}, filtered...))

	verb := utf16Ptr("runas")
	file := utf16Ptr(exe)
	params := utf16Ptr(cmdline)
	cwd := utf16Ptr(workingDir())

	r, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(params)),
		uintptr(unsafe.Pointer(cwd)),
		swShowNormal,
	)
	if r <= 32 {
		if r == errorCancelled {
			return fmt.Errorf("用户取消了管理员授权")
		}
		return fmt.Errorf("提权启动失败（ShellExecute 返回 %d）", r)
	}
	return nil
}

// MessageBox 弹一个 Win32 消息框。
func MessageBox(title, text string) {
	procMessageBoxW.Call(0,
		uintptr(unsafe.Pointer(utf16Ptr(text))),
		uintptr(unsafe.Pointer(utf16Ptr(title))),
		mbIconError)
}

const mbIconError = 0x00000010

func utf16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}

// quoteArgs 按 Windows 命令行规则给参数加引号。
func quoteArgs(args []string) string {
	var b []byte
	for i, a := range args {
		if i > 0 {
			b = append(b, ' ')
		}
		if a != "" && !strings_containsAny(a, " \t\"") {
			b = append(b, a...)
			continue
		}
		b = append(b, '"')
		backslashes := 0
		for _, r := range a {
			switch r {
			case '\\':
				backslashes++
				b = append(b, '\\')
			case '"':
				for j := 0; j < backslashes; j++ {
					b = append(b, '\\')
				}
				backslashes = 0
				b = append(b, '\\', '"')
			default:
				backslashes = 0
				b = append(b, string(r)...)
			}
		}
		for j := 0; j < backslashes; j++ {
			b = append(b, '\\')
		}
		b = append(b, '"')
	}
	return string(b)
}

func strings_containsAny(s, chars string) bool {
	for _, c := range s {
		for _, k := range chars {
			if c == k {
				return true
			}
		}
	}
	return false
}
