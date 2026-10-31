// Package fuses 实现 Electron Fuses 的读取与翻转，等价于 @electron/fuses 的
// flipFuses / getCurrentFuseWire。
//
// 原理：Electron 二进制里嵌了一段固定的哨兵字符串
//
//	"dL7pKGdnNz796PbbjQWNKmHXBZaB9tsX"
//
// 哨兵后面紧跟 2 字节头：version（1）、length（保险丝字节数），再往后就是保险丝线
// （每个保险丝 1 字节：'0'=禁用、'1'=启用、'r'=已移除、0x90=继承）。
// 改保险丝 = 就地覆写这几个字节，不改变文件大小与布局。
//
// 本补丁只需要把 OnlyLoadAppFromAsar（下标 5）置 0，
// 让 Electron 在 resources\app 目录存在时优先加载目录而不是 app.asar。
package fuses

import (
	"bytes"
	"fmt"
	"os"
)

var sentinel = []byte("dL7pKGdnNz796PbbjQWNKmHXBZaB9tsX")

const (
	fuseDisable = '0'
	fuseEnable  = '1'
	fuseRemoved = 'r'

	fuseV1Version           = 1
	fuseOnlyLoadAppFromAsar = 5
	fuseWireHeaderLen       = 2
	maxWireLen              = 255
)

// Info 是一处哨兵位置读出的保险丝线。
type Info struct {
	Pos     int64
	Version byte
	Wire    []byte
}

// Read 扫描二进制找出所有哨兵位置并读出保险丝线。
func Read(path string) ([]Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()

	const chunk = 4 << 20
	overlap := int64(len(sentinel) - 1)
	var out []Info
	buf := make([]byte, chunk+overlap)
	var pos int64
	for pos < size {
		n, err := f.ReadAt(buf, pos)
		if n <= 0 {
			break
		}
		hay := buf[:n]
		start := 0
		for {
			idx := bytes.Index(hay[start:], sentinel)
			if idx < 0 {
				break
			}
			abs := pos + int64(start+idx)
			hdr := make([]byte, fuseWireHeaderLen+maxWireLen)
			m, _ := f.ReadAt(hdr, abs+int64(len(sentinel)))
			if m < fuseWireHeaderLen {
				return nil, fmt.Errorf("保险丝头读取不完整（偏移 %d）", abs)
			}
			wireLen := int(hdr[1])
			if m < fuseWireHeaderLen+wireLen {
				return nil, fmt.Errorf("保险丝线读取不完整（偏移 %d，长度 %d）", abs, wireLen)
			}
			wire := make([]byte, wireLen)
			copy(wire, hdr[fuseWireHeaderLen:fuseWireHeaderLen+wireLen])
			out = append(out, Info{Pos: abs, Version: hdr[0], Wire: wire})
			start += idx + 1
		}
		if err != nil {
			break
		}
		pos += chunk
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("未在 %s 中找到保险丝哨兵（需要 Electron 12 及以上）", path)
	}
	return out, nil
}

// OnlyLoadAppFromAsarState 返回当前 OnlyLoadAppFromAsar 的状态（'0'=关、'1'=开）。
func OnlyLoadAppFromAsarState(path string) (byte, error) {
	fuses, err := Read(path)
	if err != nil {
		return 0, err
	}
	w := fuses[0].Wire
	if len(w) <= fuseOnlyLoadAppFromAsar {
		return 0, fmt.Errorf("保险丝线过短（%d），该 Electron 版本不支持该保险丝", len(w))
	}
	return w[fuseOnlyLoadAppFromAsar], nil
}

// DisableOnlyLoadAppFromAsar 把 OnlyLoadAppFromAsar 置 0，返回是否有改动。
func DisableOnlyLoadAppFromAsar(path string) (bool, error) {
	fuses, err := Read(path)
	if err != nil {
		return false, err
	}
	if len(fuses) > 2 {
		return false, fmt.Errorf("二进制中出现 %d 个哨兵（最多应为 2 个），文件可能已损坏", len(fuses))
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return false, err
	}
	defer f.Close()
	changed := false
	for _, fi := range fuses {
		if int(fi.Version) != fuseV1Version {
			return false, fmt.Errorf("保险丝版本 %d 与预期 %d 不符，请升级本工具", fi.Version, fuseV1Version)
		}
		if len(fi.Wire) <= fuseOnlyLoadAppFromAsar {
			return false, fmt.Errorf("保险丝线过短（%d）", len(fi.Wire))
		}
		cur := fi.Wire[fuseOnlyLoadAppFromAsar]
		if cur == fuseRemoved || cur == fuseDisable {
			continue
		}
		wirePos := fi.Pos + int64(len(sentinel)) + fuseWireHeaderLen
		if _, err := f.WriteAt([]byte{fuseDisable}, wirePos+fuseOnlyLoadAppFromAsar); err != nil {
			return false, err
		}
		changed = true
	}
	if changed {
		_ = f.Sync()
	}
	return changed, nil
}
