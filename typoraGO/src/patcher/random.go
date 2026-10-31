package patcher

import (
	"crypto/rand"
	"encoding/hex"
)

// RandomHex 生成 n 字节的十六进制随机串。
func RandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
