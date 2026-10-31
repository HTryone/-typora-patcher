package patcher

// 配置缓存：%USERPROFILE%\.typoratool.json
//
// 与 typora_crack.js 的 ~/.typora_crack_config.json 分开存：
// 两个工具可以并存，互不覆盖对方的缓存。
// 只存路径、机器码、邮箱这类非敏感信息（邮箱按用户要求留存以便「自动填充」）。

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Cache 是持久化的用户配置。
type Cache struct {
	Path        string `json:"path,omitempty"`
	MachineCode string `json:"machineCode,omitempty"`
	Email       string `json:"email,omitempty"`
}

// CachePath 返回缓存文件完整路径。
func CachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".typoratool.json")
}

// LoadCache 读缓存；目录失效时清空 Path 字段。
func LoadCache() Cache {
	var c Cache
	b, err := os.ReadFile(CachePath())
	if err != nil {
		return c
	}
	_ = json.Unmarshal(b, &c)
	if c.Path != "" && !Exists(filepath.Join(c.Path, "Typora.exe")) {
		c.Path = ""
	}
	return c
}

// SaveCache 写缓存。
func SaveCache(c Cache) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(CachePath(), b, 0o600)
}
