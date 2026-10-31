package system

// 注册表读写：直接用 advapi32 的系统调用，不依赖 golang.org/x/sys 或任何第三方包，
// 也不 fork reg.exe（避免 GBK 解码与控制台窗口闪烁）。

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	keyQueryValue = 0x0001
	keySetValue   = 0x0002
	keyCreateSub  = 0x0004
	keyRead       = 0x00020019 // KEY_READ：标准读权限组合，HKLM 只读也够
	keyAllAccess  = 0x00020000 | keyQueryValue | keySetValue | keyCreateSub
	regSZ         = 1
)

var (
	procRegOpenKeyEx = advapi32.NewProc("RegOpenKeyExW")
	procRegSetValue  = advapi32.NewProc("RegSetValueExW")
	procRegQueryVal  = advapi32.NewProc("RegQueryValueExW")
	procRegDeleteVal = advapi32.NewProc("RegDeleteValueW")
)

var advapi32 = syscall.NewLazyDLL("advapi32.dll")

func regOpen(root syscall.Handle, sub string, access uint32) (syscall.Handle, error) {
	var h syscall.Handle
	r, _, _ := procRegOpenKeyEx.Call(
		uintptr(root),
		uintptr(unsafe.Pointer(utf16Ptr(sub))),
		0,
		uintptr(access),
		uintptr(unsafe.Pointer(&h)),
	)
	if r != 0 {
		return 0, fmt.Errorf("打开注册表键失败（%s）：错误码 %d", sub, r)
	}
	return h, nil
}

// RegReadString 读取任意根键下的字符串值；值不存在时返回 ("", false)。
// 用 KEY_READ 而非 KEY_ALL_ACCESS：非管理员进程对 HKLM 只有读权限。
func RegReadString(root syscall.Handle, sub, name string) (string, bool) {
	h, err := regOpen(root, sub, keyRead)
	if err != nil {
		return "", false
	}
	defer procRegCloseKey.Call(uintptr(h))
	// 第一次调用只探长度：lpType=&typ，lpData=NULL，lpcbData=&size
	n := utf16Ptr(name)
	var typ, size uint32
	r, _, _ := procRegQueryVal.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(n)),
		0,
		uintptr(unsafe.Pointer(&typ)),
		0,
		uintptr(unsafe.Pointer(&size)),
	)
	if r != 0 || size == 0 {
		return "", false
	}
	buf := make([]uint16, size/2+1)
	r, _, _ = procRegQueryVal.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(n)),
		0,
		uintptr(unsafe.Pointer(&typ)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r != 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

// RegWriteString 在 HKCU 下写入字符串值（REG_SZ）。
func RegWriteString(sub, name, value string) error {
	h, err := regOpen(syscall.HKEY_CURRENT_USER, sub, keyAllAccess)
	if err != nil {
		return err
	}
	defer procRegCloseKey.Call(uintptr(h))
	val, err := syscall.UTF16FromString(value)
	if err != nil {
		return err
	}
	r, _, _ := procRegSetValue.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(utf16Ptr(name))),
		0,
		regSZ,
		uintptr(unsafe.Pointer(&val[0])),
		uintptr(len(val)*2),
	)
	if r != 0 {
		return fmt.Errorf("写入注册表值 %s 失败：错误码 %d", name, r)
	}
	return nil
}

// RegDeleteValue 删除 HKCU 下的值；值不存在视为成功。
func RegDeleteValue(sub, name string) error {
	h, err := regOpen(syscall.HKEY_CURRENT_USER, sub, keyAllAccess)
	if err != nil {
		return nil // 键都不存在，等价于已删除
	}
	defer procRegCloseKey.Call(uintptr(h))
	procRegDeleteVal.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(utf16Ptr(name))),
	)
	return nil
}

var procRegCloseKey = advapi32.NewProc("RegCloseKey")
