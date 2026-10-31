// Package asar 实现归档的只读解析与解包。
//
// 文件结构（与 asar 3.x / chromium-pickle 一致）：
//
//	[0:4]  uint32 LE  size-pickle 的 payload 长度，固定 4
//	[4:8]  uint32 LE  header-pickle 的总长度 N
//	[8:8+N] header pickle: [0:4]=payload 长度 P，[4:4+P]=JSON（4 字节对齐填充）
//	之后是各文件数据，偏移 = 8 + N + entry.offset
//
// 注意 header pickle 内部的 JSON 起点 = N - P + 4，不要硬编码成 8。
// 本包只解包、不重打包：补丁方式是把 resources\app 解包成真实目录，
// 再关闭 Electron 的 OnlyLoadAppFromAsar 保险丝让 Electron 优先加载目录。
package asar

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Integrity struct {
	Algorithm string   `json:"algorithm"`
	Hash      string   `json:"hash"`
	BlockSize int      `json:"blockSize"`
	Blocks    []string `json:"blocks"`
}

type Entry struct {
	Size       int64             `json:"size,omitempty"`
	Offset     string            `json:"offset,omitempty"`
	Unpacked   bool              `json:"unpacked,omitempty"`
	Executable bool              `json:"executable,omitempty"`
	Link       string            `json:"link,omitempty"`
	Integrity  *Integrity        `json:"integrity,omitempty"`
	Files      map[string]*Entry `json:"files,omitempty"`
}

// Archive 是一个已打开的归档。
type Archive struct {
	path       string
	headerSize int64 // 即 [4:8] 的 N
	Files      map[string]*Entry
	DataStart  int64
	file       *os.File
}

// Open 解析归档头，不读文件内容。
func Open(path string) (*Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var pre [8]byte
	if _, err := io.ReadFull(f, pre[:]); err != nil {
		f.Close()
		return nil, fmt.Errorf("读取 asar 头部失败: %w", err)
	}
	headerSize := int64(le32(pre[4:8]))
	if headerSize <= 0 || headerSize > 64<<20 {
		f.Close()
		return nil, fmt.Errorf("asar 头部长度异常: %d", headerSize)
	}
	hdr := make([]byte, headerSize)
	if _, err := io.ReadFull(f, hdr); err != nil {
		f.Close()
		return nil, fmt.Errorf("读取 asar 头内容失败: %w", err)
	}
	payload := int64(le32(hdr[0:4]))
	inner := headerSize - payload // header pickle 的头长度（通常 4）
	if inner < 0 || inner+4 > headerSize {
		f.Close()
		return nil, fmt.Errorf("asar 头部结构异常（inner=%d, size=%d）", inner, headerSize)
	}
	jsonLen := int64(le32(hdr[inner : inner+4]))
	if inner+4+jsonLen > headerSize {
		f.Close()
		return nil, fmt.Errorf("asar JSON 长度越界: %d", jsonLen)
	}
	var root struct {
		Files map[string]*Entry `json:"files"`
	}
	if err := json.Unmarshal(hdr[inner+4:inner+4+jsonLen], &root); err != nil {
		f.Close()
		return nil, fmt.Errorf("解析 asar 头 JSON 失败: %w", err)
	}
	return &Archive{
		path:       path,
		headerSize: headerSize,
		Files:      root.Files,
		DataStart:  8 + headerSize,
		file:       f,
	}, nil
}

// Close 关闭底层文件。
func (a *Archive) Close() error {
	if a.file != nil {
		return a.file.Close()
	}
	return nil
}

// UnpackedPath 返回归档对应的 *.unpacked 目录。
func (a *Archive) UnpackedPath() string { return a.path + ".unpacked" }

// Lookup 按相对路径找条目。
func (a *Archive) Lookup(rel string) *Entry {
	cur := a.Files
	parts := strings.Split(strings.ReplaceAll(rel, "\\", "/"), "/")
	for i, p := range parts {
		e, ok := cur[p]
		if !ok {
			return nil
		}
		if i == len(parts)-1 {
			return e
		}
		if e.Files == nil {
			return nil
		}
		cur = e.Files
	}
	return nil
}

// ExtractAll 把归档解包到 dest 目录。
func (a *Archive) ExtractAll(dest string) error {
	return a.walk("", a.Files, dest)
}

func (a *Archive) walk(rel string, entries map[string]*Entry, dest string) error {
	for name, e := range entries {
		if e == nil {
			continue
		}
		childRel := name
		if rel != "" {
			childRel = rel + string(filepath.Separator) + name
		}
		childDest := filepath.Join(dest, childRel)
		if e.Files != nil {
			if err := os.MkdirAll(childDest, 0o755); err != nil {
				return err
			}
			if err := a.walk(childRel, e.Files, dest); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(childDest), 0o755); err != nil {
			return err
		}
		if err := a.extractFile(childRel, e, childDest); err != nil {
			return fmt.Errorf("解包 %s 失败: %w", childRel, err)
		}
	}
	return nil
}

func (a *Archive) extractFile(rel string, e *Entry, dest string) error {
	// Windows 下符号链接按普通文件处理（与 asar CLI 的 followLinks 行为一致）
	if e.Link != "" {
		return a.extractFile(e.Link, a.Lookup(e.Link), dest)
	}
	var src io.Reader
	if e.Unpacked {
		f, err := os.Open(filepath.Join(a.UnpackedPath(), rel))
		if err != nil {
			return err
		}
		defer f.Close()
		src = f
	} else {
		off := int64(0)
		if e.Offset != "" {
			if _, err := fmt.Sscanf(e.Offset, "%d", &off); err != nil {
				return fmt.Errorf("偏移解析失败 %q: %w", e.Offset, err)
			}
		}
		src = io.NewSectionReader(a.file, a.DataStart+off, e.Size)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// ReadFile 直接读出归档内某个文件的内容（不解包）。
func (a *Archive) ReadFile(rel string) ([]byte, error) {
	e := a.Lookup(rel)
	if e == nil {
		return nil, fmt.Errorf("归档内没有 %s", rel)
	}
	if e.Unpacked {
		return os.ReadFile(filepath.Join(a.UnpackedPath(), rel))
	}
	off := int64(0)
	if e.Offset != "" {
		off, _ = strconv.ParseInt(e.Offset, 10, 64)
	}
	buf := make([]byte, e.Size)
	if _, err := a.file.ReadAt(buf, a.DataStart+off); err != nil {
		return nil, err
	}
	return buf, nil
}

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
